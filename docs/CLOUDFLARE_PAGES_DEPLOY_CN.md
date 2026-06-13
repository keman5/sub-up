# Cloudflare Pages 前台静态化部署指南

本文用于把 51token 前台静态资源部署到 Cloudflare Pages，减轻 VPS 上 Caddy/sub2api 对 HTML、JS、CSS、字体和图片的处理压力。模型网关、登录、后台、支付、Turnstile secret 校验和计费仍留在 VPS 后端。

## 当前边界

- 静态资源：由 Cloudflare Pages 提供。
- API：通过 `frontend/public/_worker.js` 代理回 VPS。
- `ai.upit.top` 的 API 回源默认是 `https://api.upit.top`。
- `a2.upit.top` 的 API 回源默认是 `https://ap2.upit.top`。
- Pages 预览域名可通过环境变量 `SUB2API_ORIGIN` 指定回源。
- 当前部署可先切 `a2.upit.top` 做灰度。线上实测 `ap2.upit.top` 可承载 `/api/v1/*`、`/health` 和 `/51Token/v1/*`。
- `a1.upit.top` 暂不建议直接切。线上实测 `ap1.upit.top` 可承载 `/51Token/v1/*`，但不承载前台 `/api/v1/*` 和 `/health`；直接把 `a1.upit.top` 切到 Pages 会导致登录、公开设置和后台 API 断开。
- 前台改动走 Pages 发布；后端/API 改动走 VPS 发布并默认加 `--skip-frontend-build`，避免重复为 VPS 构建前台。
- VPS 后端暂时继续使用 `go build -tags embed`，保留已有 `backend/internal/web/dist` 作为 primary/ap1 和回滚兜底；等 primary/ap1 也迁到 Pages 后，再评估 no-embed 后端发布模式。

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
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://ap2.upit.top/api/v1/settings/public \
  --html backend/internal/web/dist/index.html
```

确认产物包含 Pages Worker：

```bash
test -f backend/internal/web/dist/_worker.js
rg -n "window\\.__APP_CONFIG__|api_base_url|<title>" backend/internal/web/dist/index.html
find backend/internal/web/dist -maxdepth 2 -type f | sort | sed -n '1,80p'
```

说明：

- VPS 内嵌前台由 Go 在返回 `index.html` 前注入 `window.__APP_CONFIG__` 和站点标题；Cloudflare Pages 只托管静态 HTML，不会自动执行这段后端注入。
- Pages 发布前必须从对应环境的公开设置接口拉取配置，并写入本环境产物的 `index.html`。a2 使用 `--settings-url https://ap2.upit.top/api/v1/settings/public`，避免被 Pages 全局 `SUB2API_ORIGIN` 影响。
- 只能注入 `/api/v1/settings/public` 返回的公开 `data` 字段，不要把后台 admin 配置、`.env`、数据库连接、密钥或其它私有配置写入静态文件。
- 如果线上 public settings 改了，需要重新执行 build、inject、Pages deploy；否则首屏会继续使用上一次写入的静态配置。

### 多环境前台产物原则

- 运行时配置：站点名称、logo、Turnstile site key、`api_base_url`、`doc_url`、功能开关等来自 `/api/v1/settings/public` 的公开字段，发布前用 `scripts/inject-pages-public-settings.mjs` 写入对应环境的 `index.html`。
- 构建期配置：凡是会被 Vite、`import.meta.env`、构建脚本或前端代码打进 JS/CSS bundle 的配置，必须按环境分别打包，不要靠 Worker 或后置注入覆盖。
- 不同服务未来可以绑定不同域名、部署不同 Pages 项目或不同 Direct Upload 产物。a2 当前使用独立静态产物，不使用 Worker 动态 HTML 注入。
- 如果确认某次改动只影响运行时公开配置，可以复用同一次前端构建后的 assets，再复制产物目录分别执行 inject；如果影响构建期配置，必须每个环境单独执行 `pnpm --dir frontend run build`。

## 二、创建 Cloudflare Pages 项目

推荐先用预览项目，不要一开始切 `ai.upit.top`。

### 方式 A：Dashboard Direct Upload

1. 打开 Cloudflare Dashboard。
2. 进入 `Workers & Pages`。
3. 点击 `Create application`。
4. 选择 `Pages`。
5. 选择 `Direct Upload`。
6. 项目名填：

```text
sub2api-frontend
```

7. 上传目录：

```text
/Users/okk/git-projects/sub2api/backend/internal/web/dist
```

8. 部署完成后，先使用 Cloudflare 分配的 `*.pages.dev` 地址验证。

如果是预览域名，进入项目设置添加环境变量：

```text
SUB2API_ORIGIN=https://api.upit.top
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
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://ap2.upit.top/api/v1/settings/public \
  --html backend/internal/web/dist/index.html
pnpm dlx wrangler pages deploy backend/internal/web/dist --project-name sub2api-frontend
```

仓库根目录的 `wrangler.toml` 已设置：

```toml
name = "sub2api-frontend"
pages_build_output_dir = "backend/internal/web/dist"
```

## 三、预览验证

把下面的 `PAGES_URL` 换成 Cloudflare 给出的预览地址。

```bash
PAGES_URL="https://sub2api-frontend.pages.dev"

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

建议先绑定灰度域名：

```text
a2.upit.top
```

通过后再切主域名：

```text
ai.upit.top
```

`a1.upit.top` 不建议在本轮直接切。要迁 a1，需要先准备一个不会被 Pages 覆盖的前台 API origin，例如：

```text
ap1-admin.upit.top -> sub2api-ap1 的 /api/v1 和 /health
```

然后为 a1 单独设置 Pages 环境变量：

```text
SUB2API_ORIGIN=https://ap1-admin.upit.top
```

注意：`api.upit.top`、`ap1.upit.top`、`ap2.upit.top` 不要切到 Pages，它们继续指向 VPS 后端。

切换后验证：

```bash
curl -sSIL --max-time 15 https://ai.upit.top/ | sed -n '1,30p'
curl -sSIL --max-time 15 https://ai.upit.top/assets/ | sed -n '1,20p'
curl -sSIL --max-time 15 https://ai.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -fsS --max-time 15 https://ai.upit.top/health
```

如果先切 a2：

```bash
curl -sSIL --max-time 15 https://a2.upit.top/ | sed -n '1,30p'
curl -sSi --max-time 15 https://a2.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -sSi --max-time 15 https://a2.upit.top/api/v1/settings/public | grep -Ei 'HTTP/|x-sub2api-edge-cache|cache-control|cf-cache-status'
curl -fsSL --max-time 15 https://a2.upit.top/login | rg "window\\.__APP_CONFIG__|https://ap2.upit.top/51Token/v1|<title>"
curl -fsS --max-time 15 https://a2.upit.top/health
curl -sSIL --max-time 15 https://a2.upit.top/51Token/v1/models | sed -n '1,30p'
```

如果后续单独切了 a1：

```bash
curl -sSIL --max-time 15 https://a1.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -fsS --max-time 15 https://a1.upit.top/health
```

## 五、关闭 VPS 上的旧前台入口

确认正式域名已经由 Cloudflare Pages 承载静态资源后，可以关闭 VPS 上对应域名的旧前台入口，避免误命中 VPS 内嵌前端。但不要关闭 API 回源域名和后端容器。

当前 a2 边界：

- `a2.upit.top`：Cloudflare Pages 自定义域名，承载前台 HTML / JS / CSS。
- `ap2.upit.top`：VPS API 回源域名，继续指向 `sub2api-ap2`。
- `sub2api-ap2`：后端/API 容器，不能关闭；Pages Worker 的 `/api/*`、`/health`、`/51Token/*` 等路径仍回源到它。

VPS 上的旧写法曾把 `a2` 和 `ap2` 合并在同一个 Caddy server block：

```caddyfile
ap2.upit.top:443, a2.upit.top:443 {
    ...
}
```

前台切到 Pages 后，应只保留 API 回源域名：

```caddyfile
ap2.upit.top:443 {
    ...
}
```

操作前先备份并校验：

```bash
ssh 51token-vps '
  cp /opt/cf-origin-ssl/Caddyfile /opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-$(date +%Y%m%d%H%M%S)
  docker exec cf-origin-ssl caddy validate --config /etc/caddy/Caddyfile
'
```

修改 `/opt/cf-origin-ssl/Caddyfile` 后重载：

```bash
ssh 51token-vps '
  docker exec cf-origin-ssl caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
'
```

验证：

```bash
curl -sSIL --max-time 15 https://a2.upit.top/login | sed -n '1,30p'
curl -fsS --max-time 15 https://a2.upit.top/health
curl -fsS --max-time 15 https://ap2.upit.top/health
curl -fsS --max-time 15 https://a2.upit.top/api/v1/settings/public

ssh 51token-vps '
  docker inspect -f "{{.State.Health.Status}}" sub2api-ap2
  curl -k -sSIL --resolve a2.upit.top:443:127.0.0.1 https://a2.upit.top/login | sed -n "1,12p" || true
'
```

预期：

- `a2.upit.top/login` 仍返回 Cloudflare Pages 页面。
- `a2.upit.top/health` 与 `ap2.upit.top/health` 均返回 ok。
- `sub2api-ap2` 保持 `healthy`。
- VPS 本地直连 `a2.upit.top` 不再返回旧前台页面。

2026-06-13 已执行：

- 备份：`/opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656`
- 变更：`ap2.upit.top:443, a2.upit.top:443` 改为 `ap2.upit.top:443`
- 验证：`a2.upit.top/login` 加载 Cloudflare Pages 资产 `assets/index-D26Z13h8.js`；`a2.upit.top/health`、`ap2.upit.top/health` 均返回 ok；`sub2api-ap2` 为 `healthy`。

## 六、回滚

Cloudflare Dashboard：

1. 打开 Pages 项目。
2. 进入 `Custom domains`。
3. 移除 `ai.upit.top` / `a1.upit.top`。
4. DNS 记录改回原来的 VPS/Caddy 入口。

如果已经关闭了 VPS 旧前台入口，需要先恢复 Caddy 备份或重新把对应域名加回 server block，再 reload：

```bash
ssh 51token-vps '
  cp /opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656 /opt/cf-origin-ssl/Caddyfile
  docker exec cf-origin-ssl caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
'
```

回滚后验证：

```bash
curl -fsS https://ai.upit.top/health
curl -fsS https://a1.upit.top/health
curl -sSIL https://ai.upit.top/logo.svg | grep -Ei 'HTTP/|cf-cache-status|cache-control'
curl -sSIL https://ap2.upit.top/51Token/v1/models | sed -n '1,20p'
```

## 七、注意事项

- 不要把 Turnstile `api.js` 下载到本地或代理缓存，继续从 Cloudflare 官方地址加载。
- 不要把模型流式代理、计费、账号选择、支付回调、OAuth 回调搬进 Pages Worker。
- Pages Worker 只负责轻量路由：API 回源，静态资源留在 Pages。
- Pages 静态 HTML 发布前必须执行 `scripts/inject-pages-public-settings.mjs`，把对应环境的 public settings 注入 `window.__APP_CONFIG__`。
- 只缓存公开只读接口，且 TTL 保持短周期；不要缓存任何携带用户态、管理员态、支付态或模型流式响应的接口。
- 关闭 VPS 旧前台入口时，只移除已切到 Pages 的前台域名；不要移除 `ap2.upit.top` 这类 API 回源域名，也不要停止 `sub2api-ap2` 后端容器。
- 修改 `frontend/public/_worker.js`、Pages 配置注入脚本或相关部署流程后，必须重新运行 Worker/注入单测和前端构建。
