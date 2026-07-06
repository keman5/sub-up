# Cloudflare Pages 前台静态化部署指南

本文用于把 51token 前台静态资源部署到 Cloudflare Pages，减轻 VPS 上 Caddy/sub2api 对 HTML、JS、CSS、字体和图片的处理压力。模型网关、登录、后台、支付、Turnstile secret 校验和计费仍留在 VPS 后端。

## 当前边界

- 静态资源：由 Cloudflare Pages 提供。
- API：通过 `frontend/public/_worker.js` 代理回 VPS。
- `ai.upit.top` 的 API 回源固定是 `https://api.upit.top`。
- `a1.upit.top` 的 API 回源固定是 `https://ap1.upit.top`。
- `test.upit.top` 的 API 回源固定是 `https://a2t.upit.top`。
- Pages 预览域名可通过环境变量 `SUB2API_ORIGIN` 指定回源。
- 三套正式前台都迁到 Cloudflare Pages：`ai.upit.top`、`a1.upit.top`、`test.upit.top`。
- 线上 API 回源域名继续留在 VPS：`api.upit.top`、`ap1.upit.top`、`a2t.upit.top`。
- 前台改动走 Pages 发布；后端/API 改动走 VPS 发布并默认加 `--skip-frontend-build`，避免重复为 VPS 构建前台。
- 后端/API 常规发布使用 `deploy/local-gzip-binary-deploy.sh --apply --deploy --skip-frontend-build`，默认按 `test -> ap1 -> primary` 滚动三套线上后端环境；test 通过 `/opt/sub2api-test-deploy/.env` 的 `IMAGE_TAG` 切换镜像。
- VPS 后端暂时继续使用 `go build -tags embed`，保留已有 `backend/internal/web/dist` 作为回滚兜底；三套前台正式流量迁到 Pages 后，后端/API 常规发布不再需要为 VPS 重新打前台产物。

被代理回源的路径：

```text
/api/*
/51Token/*
/v1/*
/v1beta/*
/backend-api/*
/antigravity/*
/setup/*
/responses
/responses/*
/images/*
/health
```

其它路径交给 Pages 静态资源。这样 Vue 前端的 history 路由和 `/api/v1/settings/public` 仍能按现有相对路径工作。

公开只读接口在 Pages Worker 边缘做短缓存，减少 VPS 后端请求：

| 路径 | 方法 | TTL | 说明 |
| --- | --- | ---: | --- |
| `/api/v1/settings/public` | `GET` | `60s` | 登录页、站点 logo、Turnstile 开关、公开配置 |
| `/api/status` | `GET` | `60s` | 兼容旧前台状态接口 |
| `/api/home_page_content` | `GET` | `60s` | 首页内容兼容接口 |

缓存命中会返回响应头：

```text
X-Sub2API-Edge-Cache: HIT
```

首次回源后写入缓存：

```text
X-Sub2API-Edge-Cache: MISS
```

强制刷新或显式 `Cache-Control: no-cache/no-store` 会绕过边缘缓存。缓存回源请求会移除 `Authorization` 和 `Cookie`，这些接口必须继续保持公开只读，不得返回用户态信息。

以下路径只代理回 VPS，不做边缘缓存：

- 登录、注册、刷新 token、用户资料、后台管理等写入或用户态接口。
- `/51Token/*`、`/v1/*`、`/responses/*`、`/images/*` 等模型网关和长流式接口。
- 支付回调、OAuth 回调、账号调度、计费扣费相关接口。

## 一、本地构建和验证

```bash
cd /Users/okk/git-projects/sub2api

pnpm --dir frontend install
pnpm --dir frontend exec vitest run \
  src/cloudflare/__tests__/pages-worker.spec.ts \
  src/cloudflare/__tests__/pages-config-injection.spec.ts
pnpm --dir frontend run build

rm -rf /tmp/sub2api-pages-main /tmp/sub2api-pages-a1 /tmp/sub2api-pages-test
cp -R backend/internal/web/dist /tmp/sub2api-pages-main
cp -R backend/internal/web/dist /tmp/sub2api-pages-a1
cp -R backend/internal/web/dist /tmp/sub2api-pages-test

node scripts/inject-pages-public-settings.mjs \
  --settings-url https://api.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-main/index.html
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://ap1.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-a1/index.html
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://a2t.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-test/index.html
```

确认产物包含 Pages Worker：

```bash
test -f /tmp/sub2api-pages-main/_worker.js
test -f /tmp/sub2api-pages-a1/_worker.js
test -f /tmp/sub2api-pages-test/_worker.js
rg -n "window\\.__APP_CONFIG__|api_base_url|<title>" /tmp/sub2api-pages-main/index.html
rg -n "window\\.__APP_CONFIG__|api_base_url|<title>" /tmp/sub2api-pages-a1/index.html
rg -n "window\\.__APP_CONFIG__|api_base_url|<title>" /tmp/sub2api-pages-test/index.html
```

说明：

- VPS 内嵌前台由 Go 在返回 `index.html` 前注入 `window.__APP_CONFIG__` 和站点标题；Cloudflare Pages 只托管静态 HTML，不会自动执行这段后端注入。
- Pages 发布前必须从对应环境的公开设置接口拉取配置，并写入本环境产物的 `index.html`。主环境使用 `https://api.upit.top/api/v1/settings/public`，a1 使用 `https://ap1.upit.top/api/v1/settings/public`，test 使用 `https://a2t.upit.top/api/v1/settings/public`。
- 只能注入 `/api/v1/settings/public` 返回的公开 `data` 字段，不要把后台 admin 配置、`.env`、数据库连接、密钥或其它私有配置写入静态文件。
- 如果线上 public settings 改了，需要重新执行 build、inject、Pages deploy；否则首屏会继续使用上一次写入的静态配置。
- 对依赖 public settings 的后台功能入口（例如 `risk_control_enabled` 控制的 `/admin/risk-control`），前端路由守卫必须在本地缓存为关闭时强制刷新一次 `/api/v1/settings/public` 后再判断。否则管理员刚保存开关后，当前 SPA 仍可能因为旧缓存把“前往配置”跳转拦回设置页。

### 多环境前台产物原则

- 运行时配置：站点名称、logo、Turnstile site key、`api_base_url`、`doc_url`、功能开关等来自 `/api/v1/settings/public` 的公开字段，发布前用 `scripts/inject-pages-public-settings.mjs` 写入对应环境的 `index.html`。
- 构建期配置：凡是会被 Vite、`import.meta.env`、构建脚本或前端代码打进 JS/CSS bundle 的配置，必须按环境分别打包，不要靠 Worker 或后置注入覆盖。
- 不同服务未来可以绑定不同域名、部署不同 Pages 项目或不同 Direct Upload 产物。test 当前使用独立静态产物，不使用 Worker 动态 HTML 注入。
- 如果确认某次改动只影响运行时公开配置，可以复用同一次前端构建后的 assets，再复制产物目录分别执行 inject；如果影响构建期配置，必须每个环境单独执行 `pnpm --dir frontend run build`。

## 二、创建 Cloudflare Pages 项目

三套前台使用三个 Pages 项目，避免不同环境共享同一个 `index.html` 里的 `window.__APP_CONFIG__`：

| 环境 | 前台域名 | Pages 项目 | API 回源 |
| --- | --- | --- | --- |
| primary | `ai.upit.top` | `sub2api-frontend-main` | `https://api.upit.top` |
| a1 | `a1.upit.top` | `sub2api-frontend-a1` | `https://ap1.upit.top` |
| test | `test.upit.top` | `sub2api-frontend-test` | `https://a2t.upit.top` |

旧的 `sub2api-frontend` 项目可以作为历史/预览项目保留，不再作为三套正式前台的配置来源。

### 方式 A：Dashboard Direct Upload

1. 打开 Cloudflare Dashboard。
2. 进入 `Workers & Pages`。
3. 点击 `Create application`。
4. 选择 `Pages`。
5. 选择 `Direct Upload`。
6. 项目名分别填：

```text
sub2api-frontend-main
sub2api-frontend-a1
sub2api-frontend-test
```

7. 上传目录分别选择：

```text
/tmp/sub2api-pages-main
/tmp/sub2api-pages-a1
/tmp/sub2api-pages-test
```

8. 部署完成后，先使用 Cloudflare 分配的 `*.pages.dev` 地址验证。

如果是预览域名，进入项目设置添加环境变量。正式域名的 host 映射写在 `_worker.js`，预览域名才依赖 `SUB2API_ORIGIN`：

```text
sub2api-frontend-main: SUB2API_ORIGIN=https://api.upit.top
sub2api-frontend-a1:   SUB2API_ORIGIN=https://ap1.upit.top
sub2api-frontend-test:   SUB2API_ORIGIN=https://a2t.upit.top
```

### 方式 B：Wrangler

首次使用需要登录 Cloudflare：

```bash
pnpm dlx wrangler login
```

部署：

```bash
cd /Users/okk/git-projects/sub2api
pnpm --dir frontend run build

rm -rf /tmp/sub2api-pages-main /tmp/sub2api-pages-a1 /tmp/sub2api-pages-test
cp -R backend/internal/web/dist /tmp/sub2api-pages-main
cp -R backend/internal/web/dist /tmp/sub2api-pages-a1
cp -R backend/internal/web/dist /tmp/sub2api-pages-test

node scripts/inject-pages-public-settings.mjs \
  --settings-url https://api.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-main/index.html
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://ap1.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-a1/index.html
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://a2t.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-test/index.html

pnpm dlx wrangler pages project create sub2api-frontend-main --production-branch subapi
pnpm dlx wrangler pages project create sub2api-frontend-a1 --production-branch subapi
pnpm dlx wrangler pages project create sub2api-frontend-test --production-branch subapi

pnpm dlx wrangler pages deploy /tmp/sub2api-pages-main --project-name sub2api-frontend-main --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-a1 --project-name sub2api-frontend-a1 --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-test --project-name sub2api-frontend-test --branch subapi
```

仓库根目录的 `wrangler.toml` 仍保留给单项目/预览命令使用：

```toml
name = "sub2api-frontend"
pages_build_output_dir = "backend/internal/web/dist"
```

## 三、预览验证

把下面的 `PAGES_URL` 换成 Cloudflare 给出的预览地址。

```bash
PAGES_URL="https://<project>.pages.dev"

curl -sSIL --max-time 15 "$PAGES_URL/" | sed -n '1,30p'
curl -sSIL --max-time 15 "$PAGES_URL/assets/" | sed -n '1,20p'
curl -sSIL --max-time 15 "$PAGES_URL/api/v1/settings/public" | sed -n '1,30p'
curl -fsSL --max-time 15 "$PAGES_URL/login" | rg "window\\.__APP_CONFIG__|api_base_url|<title>"
curl -fsS --max-time 15 "$PAGES_URL/health"
```

浏览器验证：

- 首页能打开。
- `/login` 能打开。
- `/admin/accounts` 刷新后仍能显示前端页面，而不是 404。
- 页面 HTML 里包含 `window.__APP_CONFIG__`，且 `api_base_url` 指向本环境 API origin。
- Network 里 `/api/v1/settings/public` 返回 200。
- 登录页 Turnstile 脚本仍从 `https://challenges.cloudflare.com/turnstile/v0/api.js` 加载。
- 不要看到 API 请求命中 `*.pages.dev/api/...` 后返回静态 HTML。

## 四、绑定正式域名

正式域名绑定关系：

```text
ai.upit.top -> sub2api-frontend-main
a1.upit.top -> sub2api-frontend-a1
test.upit.top -> sub2api-frontend-test
```

注意：`api.upit.top`、`ap1.upit.top`、`a2t.upit.top` 不要切到 Pages，它们继续指向 VPS 后端。

切换后验证：

```bash
curl -sSIL --max-time 15 https://ai.upit.top/ | sed -n '1,30p'
curl -sSIL --max-time 15 https://ai.upit.top/assets/ | sed -n '1,20p'
curl -sSIL --max-time 15 https://ai.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -fsS --max-time 15 https://ai.upit.top/health

curl -sSIL --max-time 15 https://a1.upit.top/ | sed -n '1,30p'
curl -sSi --max-time 15 https://a1.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -sSi --max-time 15 https://a1.upit.top/api/v1/settings/public | grep -Ei 'HTTP/|x-sub2api-edge-cache|cache-control|cf-cache-status'
curl -fsSL --max-time 15 https://a1.upit.top/login | rg "window\\.__APP_CONFIG__|https://ap1.upit.top/51Token/v1|<title>"
curl -fsS --max-time 15 https://a1.upit.top/health

curl -sSIL --max-time 15 https://test.upit.top/ | sed -n '1,30p'
curl -sSi --max-time 15 https://test.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -sSi --max-time 15 https://test.upit.top/api/v1/settings/public | grep -Ei 'HTTP/|x-sub2api-edge-cache|cache-control|cf-cache-status'
curl -fsSL --max-time 15 https://test.upit.top/login | rg "window\\.__APP_CONFIG__|https://a2t.upit.top/51Token/v1|<title>"
curl -fsS --max-time 15 https://test.upit.top/health
curl -sSIL --max-time 15 https://test.upit.top/51Token/v1/models | sed -n '1,30p'
```

## 五、关闭 VPS 上的旧前台入口

确认正式域名已经由 Cloudflare Pages 承载静态资源后，可以关闭 VPS 上对应域名的旧前台入口，避免误命中 VPS 内嵌前端。但不要关闭 API 回源域名和后端容器。

三套边界：

- `ai.upit.top` / `a1.upit.top` / `test.upit.top`：Cloudflare Pages 自定义域名，承载前台 HTML / JS / CSS。
- `api.upit.top` / `ap1.upit.top` / `a2t.upit.top`：VPS API 回源域名，继续指向对应 sub2api 后端。
- `sub2api` / `sub2api-ap1` / `sub2api-test`：后端/API 容器，不能关闭；Pages Worker 的 `/api/*`、`/health`、`/51Token/*` 等路径仍回源到它们。

VPS 上如果存在前台域名 server block 或把前台和 API 域名合并在同一个 Caddy server block，需要移除前台域名，只保留 API 回源域名：

```caddyfile
ai.upit.top:443, api.upit.top:443 {
    ...
}

a1.upit.top:443, ap1.upit.top:443 {
    ...
}

a2t.upit.top:443, test.upit.top:443 {
    ...
}
```

前台切到 Pages 后，应只保留 API 回源域名及其内部测试别名，不要保留 `test.upit.top` 前台域名：

```caddyfile
api.upit.top:443, api.51tokens.upit.top:443 {
    ...
}

ap1.upit.top:443, ap1.51tokens.upit.top:443 {
    ...
}

a2t.upit.top:443, test.51tokens.upit.top:443 {
    ...
}
```

操作前先备份并校验：

```bash
ssh 51tokens '
  cp /opt/cf-origin-ssl/Caddyfile /opt/cf-origin-ssl/Caddyfile.bak-disable-frontends-$(date +%Y%m%d%H%M%S)
  docker exec cf-origin-ssl caddy validate --config /etc/caddy/Caddyfile
'
```

修改 `/opt/cf-origin-ssl/Caddyfile` 后重载：

```bash
ssh 51tokens '
  docker exec cf-origin-ssl caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
'
```

验证：

```bash
curl -sSIL --max-time 15 https://ai.upit.top/login | sed -n '1,30p'
curl -sSIL --max-time 15 https://a1.upit.top/login | sed -n '1,30p'
curl -sSIL --max-time 15 https://test.upit.top/login | sed -n '1,30p'
curl -fsS --max-time 15 https://api.upit.top/health
curl -fsS --max-time 15 https://ap1.upit.top/health
curl -fsS --max-time 15 https://a2t.upit.top/health
curl -fsS --max-time 15 https://ai.upit.top/health
curl -fsS --max-time 15 https://a1.upit.top/health
curl -fsS --max-time 15 https://test.upit.top/health

ssh 51tokens '
  docker inspect -f "{{.State.Health.Status}}" sub2api
  docker inspect -f "{{.State.Health.Status}}" sub2api-ap1
  docker inspect -f "{{.State.Health.Status}}" sub2api-test
  grep -n "ai.upit.top\\|a1.upit.top\\|a2t.upit.top\\|api.upit.top\\|ap1.upit.top\\|test.upit.top" /opt/cf-origin-ssl/Caddyfile
  curl -k -sSIL --resolve ai.upit.top:443:127.0.0.1 https://ai.upit.top/login | sed -n "1,12p" || true
  curl -k -sSIL --resolve a1.upit.top:443:127.0.0.1 https://a1.upit.top/login | sed -n "1,12p" || true
  curl -k -sSIL --resolve test.upit.top:443:127.0.0.1 https://test.upit.top/login | sed -n "1,12p" || true
'
```

预期：

- `ai/a1/test` 的 `/login` 仍返回 Cloudflare Pages 页面。
- `ai/a1/test` 的 `/health` 均经 Worker 回源返回 ok。
- `api/ap1/a2t` 的 `/health` 均直接返回 ok。
- `sub2api`、`sub2api-ap1`、`sub2api-test` 保持 `healthy`。
- VPS 本地直连 `ai/a1/test` 不再返回旧前台页面。

2026-06-13 已执行：

- 备份：`/opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656`
- 2026-07-06 更正：当前前台域名为 `test.upit.top`，API 回源域名为 `a2t.upit.top`；Caddy 应只保留 `a2t.upit.top:443, test.51tokens.upit.top:443`，不要把 `test.upit.top` 加回 VPS origin。
- 2026-07-06 后续命名清理：Caddy 内部别名从 `ap2.51tokens.upit.top` 改为 `test.51tokens.upit.top`，Caddy 静态首页目录从 `/etc/caddy/ap2-home` 改为 `/etc/caddy/test-home`。
- 验证：`test.upit.top/login` 加载 Cloudflare Pages 资产；`test.upit.top/health` 经 Worker 回源返回 ok；`a2t.upit.top/health` 直接返回 ok；`sub2api-test` 为 `healthy`。

## 六、回滚

Cloudflare Dashboard：

1. 打开 Pages 项目。
2. 进入 `Custom domains`。
3. 移除 `ai.upit.top` / `a1.upit.top`。
4. DNS 记录改回原来的 VPS/Caddy 入口。

如果已经关闭了 VPS 旧前台入口，需要先恢复 Caddy 备份或重新把对应域名加回 server block，再 reload：

```bash
ssh 51tokens '
  cp /opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656 /opt/cf-origin-ssl/Caddyfile
  docker exec cf-origin-ssl caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
'
```

回滚后验证：

```bash
curl -fsS https://ai.upit.top/health
curl -fsS https://a1.upit.top/health
curl -sSIL https://ai.upit.top/logo.svg | grep -Ei 'HTTP/|cf-cache-status|cache-control'
curl -sSIL https://a2t.upit.top/51Token/v1/models | sed -n '1,20p'
```

## 七、注意事项

- 不要把 Turnstile `api.js` 下载到本地或代理缓存，继续从 Cloudflare 官方地址加载。
- 不要把模型流式代理、计费、账号选择、支付回调、OAuth 回调搬进 Pages Worker。
- Pages Worker 只负责轻量路由：API 回源，静态资源留在 Pages。
- Pages 静态 HTML 发布前必须执行 `scripts/inject-pages-public-settings.mjs`，把对应环境的 public settings 注入 `window.__APP_CONFIG__`。
- 只缓存公开只读接口，且 TTL 保持短周期；不要缓存任何携带用户态、管理员态、支付态或模型流式响应的接口。
- 修改由 public settings 控制的前台入口或路由守卫后，验证要包含“后台保存开关后不刷新页面直接跳转”的场景，避免旧 `cachedPublicSettings` 把已开启功能误拦截。
- 关闭 VPS 旧前台入口时，只移除已切到 Pages 的前台域名；不要移除 `a2t.upit.top` 这类 API 回源域名，也不要停止 `sub2api-test` 后端容器。
- 修改 `frontend/public/_worker.js`、Pages 配置注入脚本或相关部署流程后，必须重新运行 Worker/注入单测和前端构建。
