# Fork 维护说明

本文档用于记录本 fork 相对官方仓库的本地修复、产品定制和线上部署差异，并在后续同步官方版本时逐项复查这些改动是否仍然需要保留。

## 维护原则

- 本 fork 的非上游改动必须记录为可恢复清单，包括功能目的、入口、涉及文件、验证命令和后续复查方式。
- 每个本地 bugfix 都要记录问题现象、修改文件、验证命令和后续复查方式。
- 同步官方版本后，先检查官方是否已经提供同类功能或修复，再决定保留、移除或重新实现本地补丁。
- VPS 部署应使用本 fork 构建出的镜像或二进制，避免使用官方 `latest` 镜像覆盖本地修复。

## 更新检查注意事项

当前代码里的在线检查更新默认指向官方仓库：

- `backend/internal/service/update_service.go` 中的 `githubRepo = "Wei-Shaw/sub2api"`
- `deploy/install.sh` 中的 `GITHUB_REPO="Wei-Shaw/sub2api"`

如果 VPS 运行的是本 fork，但保留上述默认值，后台“检查更新”和一键更新会以官方 release 为准。对于保留本地补丁的部署，不建议直接点击后台一键更新；应先在本 fork 中同步官方代码、复查补丁、验证通过后再重新构建并部署。

如果希望检查更新也跟随本 fork，需要将上述仓库名改为自己的 GitHub 仓库，并在 fork 中发布对应 release。

## 自动化护栏与恢复脚本

本仓库提供 fork 维护自动化脚本，目标是把“非上游改动”变成可盘点、可阻断、可导出、可验证、可恢复的流程。脚本不会在上游同步时静默改代码；遇到冲突仍需要人工审查。

**脚本位置：**

- `tools/fork-maintenance/fork-maintenance.sh`
- `tools/fork-maintenance/fork_maintenance.py`
- `tools/fork-maintenance/install-hooks.sh`
- `tools/fork-maintenance/production-state/login-agreement.json`

**常用命令：**

```bash
# 上游同步前：盘点当前 fork 相对官方上游的差异
tools/fork-maintenance/fork-maintenance.sh inventory --base upstream/main

# 提交前：检查关键 fork 路径变更是否同时更新本文件
tools/fork-maintenance/fork-maintenance.sh check-doc

# 自动追加一段待完善记录模板；提交前需要补完 TODO
tools/fork-maintenance/fork-maintenance.sh record --title "说明这次 fork 本地改动"

# 上游同步前：导出 patch 快照，便于必要时人工重放
tools/fork-maintenance/fork-maintenance.sh snapshot --base upstream/main

# merge/rebase 上游后：验证关键 fork 补丁仍存在
tools/fork-maintenance/fork-maintenance.sh verify-after-upstream

# 恢复线上非 Git 状态；默认 dry-run，不会改远端
tools/fork-maintenance/fork-maintenance.sh reapply-production-state

# 确认 dry-run 输出无误后，显式执行远端恢复
tools/fork-maintenance/fork-maintenance.sh reapply-production-state --apply
```

也可以使用 Makefile 入口：

```bash
make fork-check
make fork-inventory FORK_BASE=upstream/main
make fork-snapshot FORK_BASE=upstream/main
make fork-verify
make fork-restore-dry-run
```

部署 86 MB 左右的嵌入式后端产物时，使用 gzip 分块上传脚本，避免直接传原始二进制或在低配 VPS 上编译：

```bash
# dry-run
deploy/local-gzip-binary-deploy.sh

# 执行构建、gzip 上传、远端原子解压、打镜像并滚动两个服务
deploy/local-gzip-binary-deploy.sh --apply --deploy
```

脚本细节记录在 `docs/VPS_DEPLOY_NOTES.md` 的“sub2api 本地构建产物的 gzip 分块上传与远端解压方案”。

如果本地还没有官方上游 remote，先添加：

```bash
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
git fetch upstream
```

**可选 Git hook：**

```bash
tools/fork-maintenance/install-hooks.sh
```

安装后，`pre-commit` 会在提交前执行 `check-doc`。如果修改了品牌首页、logo/favicon、OAuth、部署、迁移等关键 fork 路径，但没有同步更新 `docs/FORK_MAINTENANCE_CN.md`，提交会被阻止。

安装脚本同时写入 `post-merge` 和 `post-rewrite` hook。执行 merge/rebase 后会自动运行 `verify-after-upstream --skip-build`，若 fork 关键补丁缺失会打印警告；它不会自动硬套补丁，也不会替代人工冲突审查。

**覆盖范围：**

- Git 内改动：
  - 通过 `inventory`、`check-doc`、`snapshot` 和 `verify-after-upstream` 盘点和验证。
  - 已提交的代码改动仍依赖 Git merge/rebase 保留；脚本只做护栏，不替代人工冲突处理。
- 线上非 Git 状态：
  - `reapply-production-state` 可恢复 `/opt/51token-home` 静态首页 logo/favicon 覆盖。
  - `favicon.ico` 会由当前 `frontend/public/logo.png` 重新生成，不直接复用可能过期的本地 ICO。
  - 可将 `tools/fork-maintenance/production-state/login-agreement.json` 中的登录条款写入 `sub2api-postgres` 和 `sub2api-standby-postgres`。
  - 默认 dry-run；只有加 `--apply` 才会通过 SSH 修改远端。

**2026-05-31 恢复测试结果：**

- 在 VPS 上故意将 `/opt/51token-home/index.html` 的 icon 链接改回 `/favicon.ico`，追加 `alternate icon`，并将 `/opt/51token-home/favicon.ico` 写成非 ICO 文本。
- 执行 `tools/fork-maintenance/fork-maintenance.sh reapply-production-state --apply` 后，静态首页恢复为 `/logo.svg?v=bg-20260531` 和 `/logo.png?v=bg-20260531`，`favicon.ico` 重新生成且 SHA256 为 `fbb2da677dce258104549107cd6b687aa485fb5a49287382729efec150dc8aaa`。
- 测试中修复了两处脚本问题：静态 HTML 恢复改为先删除所有 `icon` / `alternate icon` / `apple-touch-icon` link 再插入标准链接；登录条款 SQL 不再假设 `settings` 表存在 `created_at` / `updated_at` 字段。
- 两个库直查确认：`login_agreement_enabled=true`、`login_agreement_mode=modal`、`login_agreement_updated_at=2026-05-31`。
- 测试备份目录：`/opt/fork-maintenance-restore-test-20260531234541`。

**限制：**

- 脚本无法自动判断所有业务改动是否属于 fork 定制；新增定制仍必须人工补充本文件。
- 脚本不会自动解决上游同步冲突。
- 不要把数据库密码、session secret、API key 等敏感信息写入 `production-state/`。该目录只允许保存可公开审查的恢复状态。

## 非上游功能恢复清单

以下清单记录本 fork 中不是官方上游原生能力、但线上需要保留的功能。下次从上游重新同步或重建分支时，优先按本节逐项恢复。

### 1. 51token 品牌首页与本地静态资源

**目的：** 将默认首页替换为 51token 算力营销首页，提供套餐、FAQ、接入示例、登录/注册/控制台入口，并避免依赖外部字体资源。

**关键入口：**

- `/`、`/home`：`frontend/src/views/HomeView.vue`
- 首页组件：`frontend/src/views/home/components/*`
- 静态资源：`frontend/public/logo.svg`、`frontend/public/logo.png`、`frontend/public/favicon.ico`、`frontend/public/fonts/space-grotesk-*.ttf`

**涉及文件：**

- `frontend/index.html`
- `frontend/src/views/HomeView.vue`
- `frontend/src/views/home/components/AnimatedNumber.vue`
- `frontend/src/views/home/components/HomeFaq.vue`
- `frontend/src/views/home/components/HomeFeatures.vue`
- `frontend/src/views/home/components/HomeFooter.vue`
- `frontend/src/views/home/components/HomeHero.vue`
- `frontend/src/views/home/components/HomeIntegrations.vue`
- `frontend/src/views/home/components/HomePricing.vue`
- `frontend/src/views/home/components/HomeStats.vue`
- `frontend/src/views/home/components/PublicHeader.vue`
- `frontend/src/views/home/components/SiteLogo.vue`
- `frontend/src/views/home/components/homeData.ts`
- `frontend/src/views/home/components/useHomeScrollRestoration.ts`
- `frontend/src/style.css`
- `frontend/tailwind.config.js`
- `frontend/vite.config.ts`

**恢复要点：**

- 首页登录、注册、控制台链接必须指向当前 sub2api 前台路由，不新开窗口。
- 价格卡片、FAQ 和接入示例内容维护在 `homeData.ts`。
- `Space Grotesk` 字体文件必须放在 `frontend/public/fonts/`，由 `/fonts/*` 直接访问，避免线上 404。
- 首页滚动恢复逻辑依赖 `home-scroll-restoring` class 和 `useHomeScrollRestoration.ts`，不要只复制组件而漏掉 `index.html` 的首屏脚本。

**验证：**

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
curl -sSIL https://a1.upit.top/fonts/space-grotesk-700.ttf
```

同步上游后复查 `frontend/src/views/HomeView.vue`、`frontend/src/views/home/components/homeData.ts`、`frontend/public/fonts/`。如果官方首页结构变化，仍以 51token 首页作为线上入口。

### 2. 首页接入示例运行时 API Base 派生

**目的：** 首页 Codex、Claude、OpenAI SDK、curl 等示例里的 `base_url` 跟随当前部署副本，不再硬编码单一域名。

**规则：**

- 优先使用公开设置里的 `api_base_url`。
- 如果后端未返回配置，则按当前访问域名派生：`window.location.origin + /51Token/v1`。
- Claude/Anthropic 示例去掉末尾 `/v1`。
- 现有线上副本示例：
  - 主环境：`https://api.upit.top/51Token/v1`
  - a1 环境：`https://a1.upit.top/51Token/v1` 或由公开设置指定的 `https://ap1.upit.top/51Token/v1`

**涉及文件：**

- `frontend/src/views/HomeView.vue`
- `frontend/src/views/home/components/homeApiBase.ts`
- `frontend/src/views/home/components/homeData.ts`
- `frontend/src/views/home/components/HomeHero.vue`
- `frontend/src/views/home/components/HomeIntegrations.vue`
- `docs/plans/2026-05-28-runtime-home-api-base.md`
- `docs/plans/2026-05-28-runtime-home-api-base-design.md`

**恢复要点：**

- Codex 配置面板和 Claude 配置面板放在首页右侧示例的最前面。
- `~/.codex/config.toml` 中 `base_url` 必须来自 `buildHomeSnippetUrls()`。
- Claude 配置中的 `ANTHROPIC_BASE_URL` 使用 `buildClaudeBaseUrl()`。

**验证：**

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

浏览器打开不同副本首页，确认示例中的域名随当前副本或公开设置变化。

### 3. 顶部主题切换器与全局主题初始化

**目的：** 在控制台和首页提供系统/浅色/深色三态主题切换，并在 Vue 挂载前应用主题，避免闪屏。

**涉及文件：**

- `frontend/src/components/common/ThemeSwitcher.vue`
- `frontend/src/composables/useTheme.ts`
- `frontend/src/components/layout/AppHeader.vue`
- `frontend/src/views/home/components/PublicHeader.vue`
- `frontend/src/main.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- `docs/plans/2026-05-23-theme-switcher.md`
- `docs/plans/2026-05-23-theme-switcher-design.md`

**恢复要点：**

- `localStorage.theme` 只允许 `system`、`light`、`dark`。
- `system` 模式监听 `prefers-color-scheme`，并动态切换 `html.dark`。
- `ThemeSwitcher` 在控制台顶部和首页导航都要可见。
- i18n key 在 `common.theme.*` 下。

**验证：**

```bash
pnpm --dir frontend run typecheck
```

手工验证：切换浅色/深色/系统后刷新页面，`html.dark` 和 `localStorage.theme` 保持一致。

### 4. API Key 5h/1d/7d 速率限制用量与按窗口重置

**目的：** 用户可以在密钥列表和编辑弹窗中查看 5 小时、1 天、7 天速率限制用量、重置时间，并可重置全部或单个窗口。

**涉及文件：**

- `frontend/src/views/user/KeysView.vue`
- `frontend/src/types/index.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/api/keys.ts`
- `backend/internal/handler/api_key_handler.go`
- `backend/internal/service/api_key_service.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/handler/dto/types.go`

**恢复要点：**

- 前端 payload 支持 `reset_rate_limit_usage` 和 `reset_rate_limit_window`。
- 后端 `reset_rate_limit_window` 允许值：`5h`、`1d`、`7d`。
- 表格内每个窗口有独立重置按钮；编辑弹窗保留整体重置按钮。
- 成功重置后要刷新 Key 列表和相关 cache。

**验证：**

```bash
pnpm --dir frontend run typecheck
go test ./internal/service ./internal/handler/...
```

同步上游后搜索 `reset_rate_limit_window`、`resetRateLimitWindow`、`reset_5h_at`。如果官方仅支持整体重置，需要恢复本地按窗口能力。

### 5. 首 Token 延迟与完整耗时拆分

**目的：** 避免“平均响应”被长输出时长拉高，将首 Token 延迟和完整响应耗时分开统计、展示和聚合。

**涉及文件：**

- `backend/migrations/142_add_first_token_to_dashboard_aggregates.sql`
- `backend/internal/pkg/usagestats/usage_log_types.go`
- `backend/internal/repository/usage_log_repo.go`
- `backend/internal/repository/dashboard_aggregation_repo.go`
- `backend/internal/repository/dashboard_cache.go`
- `backend/internal/service/usage_service.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/admin/dashboard_handler.go`
- `frontend/src/api/usage.ts`
- `frontend/src/api/admin/usage.ts`
- `frontend/src/components/user/dashboard/UserDashboardStats.vue`
- `frontend/src/components/admin/usage/UsageStatsCards.vue`
- `frontend/src/views/user/UsageView.vue`
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

**恢复要点：**

- API 输出字段包括 `average_first_token_ms` / `avg_first_token_ms`。
- 预聚合表需要 `total_first_token_ms` 和 `first_token_requests`。
- 前端根级 `dashboard.firstToken`、`usage.firstToken`、`admin.dashboard.firstToken` 都必须有中英文翻译，避免显示 key。

**验证：**

```bash
pnpm --dir frontend run typecheck
go test ./internal/repository ./internal/pkg/usagestats ./internal/handler/... ./internal/service
```

同步上游后搜索 `average_first_token_ms`、`avg_first_token_ms`、`total_first_token_ms`、`first_token_requests`。

### 6. 订阅管理页运营增强

**目的：** 优化管理员订阅管理体验，便于按用户搜索、选择展示邮箱或用户名、查看周期用量窗口、打开内置操作说明。

**涉及文件：**

- `frontend/src/views/admin/SubscriptionsView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

**恢复要点：**

- 顶部过滤区包含用户模糊搜索下拉。
- 列设置里可切换用户列显示邮箱或用户名，偏好写入 localStorage。
- 订阅用量行显示日/周/月窗口进度和重置时间。
- 页面保留“使用指南”弹窗。

**验证：**

```bash
pnpm --dir frontend run typecheck
```

手工验证 `/admin/subscriptions`：用户搜索、列设置、用量窗口、指南弹窗均可用。

### 7. 51token 品牌与 OAuth 回调细节

**目的：** 线上前台显示 51token 品牌，并修复 OAuth 回调在部分路径中缺少安全状态的问题。

**涉及文件：**

- `frontend/index.html`
- `frontend/src/router/index.ts`
- `frontend/src/views/HomeView.vue`
- `frontend/src/views/auth/OAuthCallbackView.vue`
- `frontend/public/logo.svg`
- `frontend/public/logo.png`
- `frontend/public/favicon.ico`

**恢复要点：**

- 页面标题、meta description、站点名使用 51token 算力。
- `OAuthCallbackView.vue` 中保留安全回调补丁。
- 不恢复已删除的临时 OAuth 测试文件，遵循“不保留测试文件”的线上维护要求。

**验证：**

```bash
pnpm --dir frontend run typecheck
```

### 8. 线上多副本 Caddy 与 Turnstile 部署差异

**目的：** 线上同时运行主环境和 a1 环境，首页静态资源、sub2api 前台、API 前缀和 Turnstile 必须按副本正确路由。

**关键线上规则：**

- `api.upit.top` 转发主 sub2api。
- `a1.upit.top` 首页和前台走 standby sub2api。
- `ap1.upit.top` 支持 a1 API，并兼容 `/51Token/v1` 与旧 `/api/get-token/v1`。
- Caddy 首页静态 matcher 必须使用 `path + file exists`，避免 `/fonts/*` 被首页静态目录误拦后 404。
- Turnstile 前端脚本必须从 `https://challenges.cloudflare.com/turnstile/v0/api.js` 加载，不要下载成本地依赖。

**文档位置：**

- `docs/VPS_DEPLOY_NOTES.md`

**恢复要点：**

- 恢复 Caddy 时查看 `docs/VPS_DEPLOY_NOTES.md` 的“多站点 Caddy 静态资源路由”和“Turnstile 前端脚本与 CSP”。
- 首页静态目录如果不包含字体，应让 `/fonts/*` 回落到 sub2api 前台。
- Turnstile CSP 至少允许 `script-src` 和 `frame-src` 中的 `https://challenges.cloudflare.com`。

**验证：**

```bash
curl -sSIL https://a1.upit.top/fonts/space-grotesk-700.ttf
curl -sSIL 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'
```

浏览器打开 `https://a1.upit.top/login`，确认字体 200、Turnstile 主脚本 200，且没有 `dashboard.firstToken` 等 i18n key 直出。

### 9. 2026-05-31 线上 OAuth、登录条款、logo 与 favicon 迭代

**目的：** 修复 GitLab/OIDC 登录回调失败后的排障路径，完成两个 sub2api 服务滚动部署，写入登录所需条款，并修复手机浏览器标签页图标仍显示旧 favicon 的问题。

**问题现象：**

- GitLab 授权成功后回调到 `/api/v1/auth/oauth/github/callback`，前端最终跳转到 `/auth/oauth/callback#error=token_exchange_failed&error_description=missing+access_token&error_message=failed+to+exchange+oauth+code`。
- VPS 远端 Go 构建长时间无响应，最终确认 1 CPU / 约 2 GiB 内存环境下编译压力过高，不适合在 VPS 上直接构建。
- 手机浏览器打开 `https://ai.upit.top/` 后，标签页图标仍显示旧 `/favicon.ico` 的“两点”样式。
- 需要将登录协议条款写入主库和 standby 库，并同步部署两个 `sub2api` 服务。

**本地代码改动：**

- `frontend/index.html`
  - 静态 `<link rel="icon">` 从 `/favicon.ico` 改为 `/logo.svg`，类型为 `image/svg+xml`。
  - 保留 `apple-touch-icon` 指向 `/logo.png`。
- `frontend/src/App.vue`
  - 首页 `/` 和 `/home` 不再强制回退到 `/favicon.ico`。
  - 首页先读取 public settings，再使用 `site_logo` 更新浏览器图标。
  - 站点 logo 变化时同步更新桌面 tab icon 和移动端 touch icon。
- `frontend/src/utils/siteIcons.ts`
  - 新增 `resolveIconMimeType()`，支持带 query/hash 的 `.svg`、`.png`、`.ico` URL。
  - 新增 `applySiteIcons()`，统一维护 `rel="icon"` 与 `rel="apple-touch-icon"`。
- `frontend/src/__tests__/app-favicon.spec.ts`
  - 覆盖 `/logo.svg?v=...` MIME 识别。
  - 覆盖桌面 favicon 与移动端 touch icon 同步更新。

**数据库配置：**

两个后台数据库均写入登录条款配置：

- `sub2api-postgres`
- `sub2api-standby-postgres`

写入 key：

- `login_agreement_enabled=true`
- `login_agreement_mode=modal`
- `login_agreement_updated_at=2026-05-31`
- `login_agreement_documents`
  - 写入 4 份中文 Markdown 条款文档。
  - public settings 校验到文档数量为 4，正文总长度约 2393 字符。

**构建与部署：**

- 前端构建：`pnpm build`
- 后端构建：本机交叉编译 Linux amd64 静态二进制，避免 VPS 直接拉依赖和编译。
- 本地产物：`/tmp/sub2api-build-output/sub2api`
- 产物类型：Linux x86_64 static binary，约 86 MB。
- 远端镜像：`sub2api:subapi-6b800b77-favicon-20260531210858`
- 基础镜像：`sub2api:subapi-6b800b77-logo-dbterms-20260530222347`
- 部署顺序：
  1. 更新 `sub2api-standby`。
  2. 等待 `sub2api-standby` health 变为 `healthy`。
  3. 更新 `sub2api`。
  4. 等待 `sub2api` health 变为 `healthy`。

Compose 备份：

- `/opt/sub2api-deploy/docker-compose.yml.bak-20260531210930`
- `/opt/sub2api-standby-deploy/docker-compose.yml.bak-20260531210930`

**Caddy 静态首页修复：**

`ai.upit.top` 的根首页由 Caddy 优先托管 `/opt/51token-home`，不是直接返回 sub2api 容器内 Vue 首页。因此容器内构建修复后，还必须同步修复 Caddy 静态首页：

- `/opt/51token-home/index.html`
  - 移除旧的 `<link rel="alternate icon" href="/favicon.ico" />`。
  - 移除旧的末尾 `<link rel="icon" href="/favicon.ico">`。
  - 改为 `/logo.svg?v=bg-20260531` 和 `/logo.png?v=bg-20260531`。
- `/opt/51token-home/favicon.ico`
  - 使用当前 `frontend/public/logo.png` 重新生成 ICO，作为浏览器自动请求 `/favicon.ico` 的兜底。
- `/opt/51token-home/logo.png`、`/opt/51token-home/logo.svg`
  - 同步为当前仓库 logo 资源。
- 发现 `cf-origin-ssl` 容器内只读 bind mount 仍停留在旧目录 inode，重启 `cf-origin-ssl` 后重新挂载当前 `/opt/51token-home` 并生效。

静态首页备份：

- `/opt/51token-home/index.html.bak-20260531211900`
- `/opt/51token-home/favicon.ico.bak-20260531211900`
- `/opt/51token-home/logo.png.bak-20260531211900`
- `/opt/51token-home/logo.svg.bak-20260531211900`

**验证：**

```bash
pnpm vitest run src/__tests__/app-favicon.spec.ts
pnpm build
curl -fsS http://127.0.0.1:8081/health
curl -fsS http://127.0.0.1:8082/health
curl -fsS https://ai.upit.top/health
curl -k -sS -H 'Cache-Control: no-cache' 'https://ai.upit.top/?v=bg-20260531'
curl -k -fsS 'https://ai.upit.top/favicon.ico?v=bg-20260531' -o /tmp/ai-favicon.ico
```

结果：

- `src/__tests__/app-favicon.spec.ts`：2 个测试通过。
- `pnpm build`：通过，仅有既有 Vite chunk size / dynamic import 提醒。
- `sub2api` 与 `sub2api-standby`：均为 `healthy`。
- `https://ai.upit.top/health`：返回 `{"status":"ok"}`。
- 公开首页 HTML 只包含：
  - `<link rel="icon" type="image/svg+xml" href="/logo.svg?v=bg-20260531" />`
  - `<link rel="apple-touch-icon" href="/logo.png?v=bg-20260531" />`
- 未再发现旧 `favicon.ico` 或 `alternate icon` 引用。
- `https://ai.upit.top/favicon.ico?v=bg-20260531` 返回当前 logo 生成的 ICO。

**自动恢复实测：**

```bash
tools/fork-maintenance/fork-maintenance.sh reapply-production-state --apply
tools/fork-maintenance/fork-maintenance.sh verify-after-upstream --skip-build
make fork-check
make fork-restore-dry-run
git diff --check
```

结果：

- 故意破坏线上静态首页 icon 链接和 `/favicon.ico` 后，`reapply-production-state --apply` 成功恢复。
- 公网首页 HTML 不包含 `favicon.ico` / `alternate icon`，只保留当前 logo 链接。
- 远端 `/opt/51token-home/favicon.ico` 为有效 ICO，SHA256 为 `fbb2da677dce258104549107cd6b687aa485fb5a49287382729efec150dc8aaa`。
- 两个 PostgreSQL 容器均恢复登录条款配置：`enabled=true`、`mode=modal`、`updated_at=2026-05-31`。
- `verify-after-upstream --skip-build` 中的 favicon 回归测试通过：1 个测试文件、2 个测试。

**回滚方式：**

- 应用容器回滚：
  - 将两个 compose 的应用镜像改回 `sub2api:subapi-6b800b77-logo-dbterms-20260530222347`。
  - 按 standby、primary 顺序执行 `docker compose up -d sub2api`。
- Caddy 静态首页回滚：
  - 使用 `/opt/51token-home/*.bak-20260531211900` 恢复对应文件。
  - 重启 `cf-origin-ssl` 让只读 bind mount 重新挂载。
- 数据库条款回滚：
  - 按需将 `login_agreement_enabled` 改回 `false`，或恢复 `login_agreement_documents` 的旧值。

**同步官方后的复查：**

- 搜索 `applySiteIcons`、`resolveIconMimeType`、`siteLogo`、`favicon.ico`。
- 确认首页和登录页仍使用当前站点 logo，而不是官方默认 favicon。
- 如果官方后续统一了 favicon / touch icon 管理，可按官方方案收敛，但必须保留 Caddy 静态首页的图标覆盖和 `/favicon.ico` 兜底。
- 更新 Caddy 或重建 `cf-origin-ssl` 后，重新检查 `/srv/51token-home/index.html` 是否和 `/opt/51token-home/index.html` 一致。

### 10. 2026-06-01 线上重新构建与 gzip 分块部署

**目的：** 将当前 fork 重新构建并滚动部署到线上两个 sub2api 服务，同时验证 gzip 传输和远端解压方案在当前 VPS SSH 链路下可用。

**部署信息：**

- 分支：`subapi`
- Git 版本：`0507503d`
- 新镜像：`sub2api:subapi-0507503d-redeploy-20260601-20260601083104`
- 基础镜像：`sub2api:subapi-6b800b77-favicon-20260531210858`
- 本地二进制：`/tmp/sub2api-build-output/sub2api`
- 原始大小：`90075298` bytes
- gzip 大小：`27178093` bytes
- 远端二进制：`/opt/sub2api-runtime-build/sub2api`

**脚本调整：**

- `deploy/local-gzip-binary-deploy.sh` 的 Go 构建目录修正为 `backend/`，避免在仓库根目录执行 `go build ./cmd/server` 时找不到 `go.mod`。
- 上传步骤从单条 gzip 管道改为本地生成 `.gz`、切成 `UPLOAD_CHUNK_SIZE=1m` 小块、通过 SSH 标准输入逐块写入远端 `/tmp/sub2api-upload-<timestamp>`。
- 远端拼接 chunk 后执行 `gzip -t`，再解压到临时可执行文件并原子替换正式二进制。
- SSH / scp 类操作增加 5 次退避重试，减少偶发 `Connection reset by peer` 对部署的影响。

**验证：**

```bash
TAG_SUFFIX=redeploy-20260601 UPLOAD_CHUNK_SIZE=1m deploy/local-gzip-binary-deploy.sh --apply --deploy --skip-frontend-build --skip-backend-build
curl -fsS https://ai.upit.top/health
curl -k -sS -H 'Cache-Control: no-cache' 'https://ai.upit.top/?redeploy=20260601'
```

结果：

- `sub2api-standby` 先更新并变为 `healthy`，`http://127.0.0.1:8082/health` 返回 `{"status":"ok"}`。
- `sub2api` 后更新并变为 `healthy`，`http://127.0.0.1:8081/health` 返回 `{"status":"ok"}`。
- `https://ai.upit.top/health` 返回 `{"status":"ok"}`。
- 两个 compose 均指向 `sub2api:subapi-0507503d-redeploy-20260601-20260601083104`。
- 公网首页仍只包含 `/logo.svg?v=bg-20260531` 和 `/logo.png?v=bg-20260531` 图标链接，没有回退到旧 `favicon.ico`。

## 本地补丁记录

| 日期 | 问题 | 本地修复 | 验证 | 后续同步复查 |
| --- | --- | --- | --- | --- |
| 2026-05-23 | 账号状态为历史值 `disabled` 时，后台账号列表显示翻译 key：`admin.accounts.status.disabled`。 | 在 `frontend/src/components/account/AccountStatusIndicator.vue` 中将旧值 `disabled` 的显示 key 规范化为 `inactive`，复用现有“停用 / Inactive”文案；在 `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts` 增加回归测试。 | `pnpm vitest run src/components/account/__tests__/AccountStatusIndicator.spec.ts`；`pnpm typecheck`。 | 同步官方后搜索 `admin.accounts.status.disabled` 和 `normalizeAccountStatusLabelKey`。如果官方已在 API 或组件层将 `disabled` 规范化为 `inactive`，可移除此本地补丁；如果仍可能返回旧值，需要保留或按新结构重做。 |
| 2026-05-24 | Chrome 中后台账号编辑弹窗快速滚动或频繁切换时，偶发出现弹窗内容消失、背景表格露出或关闭时弹窗残影二次出现；Edge 不易复现。 | 在 `frontend/src/components/common/BaseDialog.vue` 中将关闭时的 body 滚动解锁和焦点恢复延后到 Vue leave transition 的 `after-leave`，避免关闭动画期间提前移除 `html/body.modal-open`；在 `frontend/src/style.css` 中保留弹窗进入/关闭动画，同时在 `html.modal-open` 期间禁用背景 glass/table header 的 `backdrop-filter`，规避 Chrome 对固定弹窗、滚动容器和背景模糊层组合时的合成闪烁。 | 基线版本可复现闪烁；仅加入 modal-open 期间禁用背景 `backdrop-filter` 后，3 分钟系统录屏逐帧分析未发现稳定闪烁候选帧；`pnpm --dir frontend run typecheck`；`git diff --check`；浏览器侧确认 ESC 关闭时 `modal-open` 与 leave DOM 同步保留到动画结束。 | 同步官方后搜索 `pendingUnlockAfterLeave`、`handleAfterLeave`、`html.modal-open .glass` 和 `html.modal-open .table-scroll-container thead`。如果官方已重构弹窗或移除了背景 `backdrop-filter` 闪烁根因，需要在 Chrome 中重新执行“打开编辑账号弹窗、快速滚动、频繁切换弹窗、ESC 关闭”的回归测试；确认无闪烁后可删除本地补丁，否则按新结构补回。 |
| 2026-05-25 | 线上“平均响应”容易被完整流式输出时长拉高，无法区分首 Token 延迟和完整输出耗时。 | 后端统计类型、用户/管理端用量聚合、API Key 查询和账号统计新增 `average_first_token_ms` / `avg_first_token_ms`；新增迁移 `backend/migrations/142_add_first_token_to_dashboard_aggregates.sql`，为仪表盘预聚合表记录首 Token 总耗时与样本数；前端仪表盘、用量统计、API Key 查询和账号统计将响应耗时拆为“首 Token”和“完整耗时”。 | `pnpm --dir frontend run typecheck`；`go test ./internal/repository`；`go test ./internal/pkg/usagestats`；`go test ./internal/handler/...`；`go test ./internal/service`。 | 同步官方后搜索 `average_first_token_ms`、`avg_first_token_ms`、`total_first_token_ms` 和 `first_token_requests`。如果官方已提供等价的 TTFT/完整耗时拆分，按官方字段名收敛前端展示；否则保留本地迁移和聚合逻辑。历史预聚合行默认没有首 Token 累计值，如需要历史口径准确，升级后执行对应聚合回填。 |
| 2026-05-31 | 手机浏览器打开 `ai.upit.top` 时仍显示旧 `/favicon.ico` 图标；同时完成 GitLab/OIDC OAuth 排障、登录条款入库和两个 sub2api 服务滚动部署。 | 前端新增 `siteIcons` 工具并让首页使用 public settings 的 `site_logo` 更新 favicon / touch icon；`frontend/index.html` 静态图标改为 `/logo.svg`；线上 Caddy 静态首页 `/opt/51token-home` 移除旧 favicon 引用，替换当前 logo 与 ICO 兜底；两个后台库写入登录条款；两个服务部署镜像 `sub2api:subapi-6b800b77-favicon-20260531210858`；新增 fork 维护自动化，支持关键路径变更文档护栏、上游同步后验证、线上静态资源和登录条款恢复、gzip 分块传输部署。 | `pnpm vitest run src/__tests__/app-favicon.spec.ts`；`pnpm build`；`curl -fsS https://ai.upit.top/health`；`curl -k -sS -H 'Cache-Control: no-cache' 'https://ai.upit.top/?v=bg-20260531'`，确认公开 HTML 不再包含 `favicon.ico` / `alternate icon`；故意破坏 `/opt/51token-home` 后执行 `reapply-production-state --apply`，再直查公网 HTML、ICO 哈希和两个库登录条款；`verify-after-upstream --skip-build`；`make fork-check`；`make fork-restore-dry-run`；`git diff --check`。 | 同步官方后搜索 `applySiteIcons`、`resolveIconMimeType`、`favicon.ico`、`site_logo`；merge/rebase 后 hook 会自动运行 `verify-after-upstream --skip-build`；必要时先看 `make fork-restore-dry-run`，确认无误后执行 `tools/fork-maintenance/fork-maintenance.sh reapply-production-state --apply`；重建 Caddy 或静态首页后检查 `/srv/51token-home/index.html` 和 `/opt/51token-home/index.html` 是否一致，并确认 `/favicon.ico` 仍为当前 logo 兜底。 |
| 2026-06-01 | 重新构建上线时，仓库根目录执行 Go 构建找不到 `go.mod`，且当前 VPS SSH 链路对长时间 gzip 管道和单文件 scp 不稳定。 | 修正 `deploy/local-gzip-binary-deploy.sh` 后端构建目录为 `backend/`；gzip 上传改为本地 `.gz` + 1 MB chunk + SSH stdin 写入 `/tmp` + 远端拼接校验解压；增加 SSH 操作重试；重新构建并部署镜像 `sub2api:subapi-0507503d-redeploy-20260601-20260601083104` 到 standby 和 primary。 | `bash -n deploy/local-gzip-binary-deploy.sh`；前端 `pnpm --dir frontend run build` 通过；后端本地交叉编译通过；远端 `/opt/sub2api-runtime-build/sub2api` 为 86 MB 静态 Linux x86_64 可执行文件；两个 compose 指向新镜像；`8081`、`8082` 和公网 `/health` 均返回 ok；公网首页图标链接未回退。 | 后续部署继续使用 gzip 分块方案；若 SSH 链路恢复稳定，可调大 `UPLOAD_CHUNK_SIZE`，但保留远端 `gzip -t` 和 standby-first 滚动顺序。 |

## 同步官方版本后的复查流程

1. 记录当前 fork 状态：

   ```bash
   git status --short
   git log --oneline -5
   ```

2. 拉取官方仓库更新：

   ```bash
   git remote -v
   git fetch upstream
   ```

3. 合并或 rebase 官方分支前，先确认本地补丁清单：

   ```bash
   sed -n '/## 本地补丁记录/,$p' docs/FORK_MAINTENANCE_CN.md
   ```

4. 合并官方更新：

   ```bash
   git merge upstream/main
   ```

   如果你的维护方式偏向线性历史，也可以使用 `git rebase upstream/main`。遇到冲突时，优先保留官方实现；只有官方仍未解决本地问题时，才保留 fork 补丁。

5. 对每条本地补丁做复查：

   ```bash
   rg -n "admin\\.accounts\\.status\\.disabled|normalizeAccountStatusLabelKey" frontend/src
   pnpm vitest run src/components/account/__tests__/AccountStatusIndicator.spec.ts
   pnpm typecheck
   ```

6. 根据复查结果更新“本地补丁记录”：

   - `官方已修复`：删除本地重复补丁，记录已由官方覆盖。
   - `仍需保留`：保留本地补丁，确认测试仍通过。
   - `需要重做`：官方代码结构已变化，按新结构重新实现最小修复，并补充新的测试路径。

7. 重新构建并部署 fork 版本：

   ```bash
   docker build -t your-registry/sub2api:fork .
   ```

   或使用当前项目既有的发布流程构建二进制。不要直接切回 `weishaw/sub2api:latest`，否则本地补丁会被官方镜像覆盖。

## 新增补丁记录模板

新增本地 bugfix 后，复制下面模板追加到“本地补丁记录”表格或单独展开为小节：

~~~~markdown
### YYYY-MM-DD: 问题标题

**现象：**
描述用户看到的问题、页面、接口或日志。

**原因：**
描述定位到的根因，尽量引用具体文件和 key。

**修改：**
- `path/to/file`
- `path/to/test`

**验证：**
```bash
command
```

**同步官方后的复查：**
说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。
~~~~
