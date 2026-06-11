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

## 一、本地构建和验证

```bash
cd /Users/okk/git-projects/sub2api

pnpm --dir frontend install
pnpm --dir frontend exec vitest run src/cloudflare/__tests__/pages-worker.spec.ts
pnpm --dir frontend run build
```

确认产物包含 Pages Worker：

```bash
test -f backend/internal/web/dist/_worker.js
find backend/internal/web/dist -maxdepth 2 -type f | sort | sed -n '1,80p'
```

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
curl -fsS --max-time 15 "$PAGES_URL/health"
```

浏览器验证：

- 首页能打开。
- `/login` 能打开。
- `/admin/accounts` 刷新后仍能显示前端页面，而不是 404。
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
curl -sSIL --max-time 15 https://a2.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -fsS --max-time 15 https://a2.upit.top/health
curl -sSIL --max-time 15 https://a2.upit.top/51Token/v1/models | sed -n '1,30p'
```

如果后续单独切了 a1：

```bash
curl -sSIL --max-time 15 https://a1.upit.top/api/v1/settings/public | sed -n '1,30p'
curl -fsS --max-time 15 https://a1.upit.top/health
```

## 五、回滚

Cloudflare Dashboard：

1. 打开 Pages 项目。
2. 进入 `Custom domains`。
3. 移除 `ai.upit.top` / `a1.upit.top`。
4. DNS 记录改回原来的 VPS/Caddy 入口。

回滚后验证：

```bash
curl -fsS https://ai.upit.top/health
curl -fsS https://a1.upit.top/health
curl -sSIL https://ai.upit.top/logo.svg | grep -Ei 'HTTP/|cf-cache-status|cache-control'
curl -sSIL https://api.upit.top/51Token/v1/models | sed -n '1,20p'
```

## 六、注意事项

- 不要把 Turnstile `api.js` 下载到本地或代理缓存，继续从 Cloudflare 官方地址加载。
- 不要把模型流式代理、计费、账号选择、支付回调、OAuth 回调搬进 Pages Worker。
- Pages Worker 只负责轻量路由：API 回源，静态资源留在 Pages。
- 修改 `frontend/public/_worker.js` 后必须重新运行 Worker 单测和前端构建。
