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

# 提交前：为非上游同步的 staged 本地改动自动追加维护记录
tools/fork-maintenance/fork-maintenance.sh check-doc

# 自动追加一段待完善记录模板；提交前需要补完 TODO
tools/fork-maintenance/fork-maintenance.sh record --title "说明这次 fork 本地改动"

# 手动整理“本地补丁记录”表格，按日期递增排序
tools/fork-maintenance/fork-maintenance.sh sort-doc

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

安装后，`pre-commit` 会在提交前执行 `check-doc`。如果当前不是 merge/rebase/cherry-pick 等上游同步流程，脚本会把本次 staged 的源码、配置和文档改动视为 fork 维护候选；当 `docs/FORK_MAINTENANCE_CN.md` 尚未 staged 时，会自动追加一条待完善记录并把文档加入本次提交。`check-doc` 和 `record` 写入记录后会自动整理“本地补丁记录”表格，保持按日期递增排序；手工调整旧记录时可执行 `sort-doc`。提交后需要将自动记录补充为可复查的业务说明、验证命令和同步官方后的处理方式。

安装脚本同时写入 `post-merge` 和 `post-rewrite` hook。执行 merge/rebase 后会自动运行 `verify-after-upstream --skip-build`，若 fork 关键补丁缺失会打印警告；它不会自动硬套补丁，也不会替代人工冲突审查。

**覆盖范围：**

- Git 内改动：
  - 通过 `inventory`、`check-doc`、`snapshot` 和 `verify-after-upstream` 盘点和验证。
  - `check-doc` 不再依赖固定 protected 路径；除维护文档本身、临时目录和构建产物外，非 merge/rebase/cherry-pick 等上游同步期间 staged 的本地改动都会触发自动记录。
  - 已提交的代码改动仍依赖 Git merge/rebase 保留；脚本只做护栏，不替代人工冲突处理。
- 线上非 Git 状态：
  - `reapply-production-state` 可恢复 `/opt/51token-home` 静态首页 logo/favicon 覆盖。
  - `favicon.ico` 会由当前 `frontend/public/logo.png` 重新生成，不直接复用可能过期的本地 ICO。
  - 可将 `tools/fork-maintenance/production-state/login-agreement.json` 中的登录条款写入共享 `sub2api-postgres` 内的 `sub2api`、`sub2api_ap1`、`sub2api_ap2` 数据库；`sub2api-ap1-postgres` 已在 2026-06-11 共享数据层迁移后废弃。
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
- Claude/Anthropic 示例从 OpenAI/Codex base 派生时，必须保留 51token 接入路径但去掉末尾 `/v1`，即 `https://<host>/51Token`。Claude Code 会自行拼接 `/v1/messages`，这里不要写成 `/51Token/v1`。
- 现有线上副本示例：
  - 主环境：`https://api.upit.top/51Token/v1`
  - a1 环境：`https://a1.upit.top/51Token/v1` 或由公开设置指定的 `https://ap1.upit.top/51Token/v1`
  - Claude Code：`https://api.upit.top/51Token`、`https://a1.upit.top/51Token`、`https://ap1.upit.top/51Token`

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
- Claude 配置中的 `ANTHROPIC_BASE_URL` 使用 `buildClaudeBaseUrl()`；绝对 URL 形式的 OpenAI/Codex base 应生成 `https://<host>/51Token`，不要带 `/v1`。
- Claude Code JSON 配置保持合法 JSON，不在 `settings.json` 内写 `//` 注释；页面说明和 Shell 示例中需要注明“Claude Code 这里不带 `/v1`”。

**验证：**

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

浏览器打开不同副本首页，确认示例中的域名随当前副本或公开设置变化。

### 3. CC Switch 导入服务商与用量查询脚本

**目的：** 用户在 API Key 列表中点击“导入到 CC Switch”后，Codex 服务商可直接带入当前副本的 API Base URL、API Key 和用量查询脚本，并能在 CC Switch 桌面版显示 51token 剩余额度。

**规则：**

- 模型请求地址继续使用公开设置里的 `api_base_url`，例如 `https://api.upit.top/51Token/v1` 或其它环境自己的公开 API base。
- deep link 的 `homepage` / 官网地址必须从 `api_base_url` 派生为站点 `origin`，不要直接写成带 API 路径的 `baseUrl`；`endpoint` 才继续使用完整模型请求地址。
- `usageScript.request.url` 使用当前 `baseUrl` 去掉尾斜杠后拼接 `/usage`，不要额外拼 `/v1`。
- extractor 需要兼容三类响应：
  - 钱包/订阅余额型：`remaining`、`quota.remaining`、`balance`。
  - 订阅窗口型：按日/周/月 limit - usage 取可用余额。
  - API Key 速率限制型：从 `rate_limits[].remaining` 取最小剩余额度。

**涉及文件：**

- `frontend/src/views/user/KeysView.vue`
- `frontend/src/utils/ccswitchImport.ts`

**恢复要点：**

- `usageScript` 保持在 `executeCcsImport()` 内联生成，不额外抽新文件。
- `baseUrl=https://<host>/51Token/v1` 时，`homepage` 必须是 `https://<host>`，`endpoint` 必须是 `https://<host>/51Token/v1`，脚本 URL 必须是 `https://<host>/51Token/v1/usage`。
- 重新导入旧服务商后，要在 CC Switch 中确认“配置用量查询”的脚本不包含 `/v1/v1/usage`。

**验证：**

```bash
pnpm --dir frontend run typecheck
```

手工验证：在 CC Switch 桌面版刷新 `51token 算力`、`Ap1`、`Ap2`、`api` 等条目的用量，限额型 key 应显示类似 `剩余：40.00 USD`，订阅/钱包型 key 应显示对应余额。

同步上游后搜索 `executeCcsImport`、`usageScript`、`rate_limits`、`/v1/usage`。如果官方新增 CC Switch 导入逻辑，也必须保留 51token 的 `/51Token/v1/usage` 与速率限制型 extractor 兼容。

### 4. 顶部主题切换器与全局主题初始化

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

### 5. API Key 5h/1d/7d 速率限制用量与按窗口重置

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

### 6. 首 Token 延迟与完整耗时拆分

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

### 7. 订阅管理页运营增强

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

### 8. 51token 品牌与 OAuth 回调细节

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
- `a1.upit.top` 首页和前台走 `sub2api-ap1`。
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
| 2026-06-02 | 首页在移动端部分区块存在文本与代码片段横向溢出风险；同时新增网关动态模型路由以在不同压力和能力场景自动选择模型。 | 前端 `HomeHero/HomeIntegrations/HomePricing/HomeFeatures/HomeFaq/HomeFooter`、`homeData.ts` 与 `style.css` 调整为移动端优先，增加 `min-w-0`、`break-words`、窄屏字号与容器约束避免横向滚动；后端新增 `gateway.model_router.*` 配置与网关接入逻辑（`backend/internal/config/config.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_model_router.go`）。动态模型路由不是默认常开，而是同时受配置项 `gateway.model_router.enabled` 与全局设置 `openai_advanced_scheduler_enabled` 双重控制：配置层负责是否编译/部署此能力，全局设置页里的 “OpenAI 实验调度策略” 开关负责是否真正放量启用。默认应保持关闭，只有在后台全局设置显式打开后，`gpt-5.3-codex-spark` / `gpt-5.4` / `gpt-5.5` 三档路由才会生效；OpenAI OAuth 默认 `passthrough`，可通过 `GATEWAY_MODEL_ROUTER_OAUTH_MODE=adaptive_codex` 在 a2 灰度启用 OAuth/Codex Pro 自适应路由，用户侧响应和普通用量记录仍隐藏真实 `upstream_model`。 | `pnpm --dir frontend run typecheck`；移动端手工检查首页无横向超出；线上检查 `docker exec sub2api env | grep GATEWAY_MODEL_ROUTER`；后台全局设置确认 `openai_advanced_scheduler_enabled=false` 时动态路由不生效、切换为 `true` 后再灰度验证；`go test ./internal/service -run 'TestOpenAIModelRouter|TestReplaceModelInSSELine|TestReplaceModelInResponseBody'`；`curl -fsS https://ai.upit.top/health` 与 `https://a1.upit.top/health`。 | 同步官方后复查首页组件是否仍保留移动端防溢出约束（`min-w-0`、`break-words`、`whitespace-pre-wrap` 等）；若官方已提供同等修复可移除本地差异。路由侧复查 `gateway.model_router`、`openai_model_router.go`、`openai_advanced_scheduler_enabled` 和 compose 中的 `GATEWAY_MODEL_ROUTER_*` 是否被覆盖；如果线上看到高压账号仍走 `gpt-5.5/gpt-5.4`，先检查容器环境变量、后台全局设置开关是否真正开启，并确认 OAuth 是否需要显式设置 `GATEWAY_MODEL_ROUTER_OAUTH_MODE=adaptive_codex`。 |
| 2026-06-02 | 首页新增悬浮客服入口，需要在移动端和桌面端保持可见但不遮挡主要 CTA，同时保留 Turnstile 校验与现有首页内容布局。 | 新增 `frontend/src/views/home/components/HomeSupportWidget.vue` 和 `frontend/public/qq-support-qr.jpeg`，在首页右下角提供可展开的 QQ 客服悬浮窗与二维码；`frontend/src/views/HomeView.vue` 挂载该组件；`frontend/src/components/TurnstileWidget.vue` 调整容器与层级，避免悬浮客服与 Turnstile/弹窗覆盖冲突；`frontend/src/views/home/components/homeData.ts` 增补客服展示数据。 | 建议执行 `pnpm --dir frontend run typecheck`；桌面端与移动端分别打开首页，确认悬浮按钮可展开/收起，二维码清晰可见，且不会遮挡首页主按钮、表单或 Turnstile。 | 同步官方后搜索 `HomeSupportWidget`、`qq-support-qr.jpeg`、`supportWidget` 和 `TurnstileWidget`。如果官方后续引入统一的悬浮客服/联系入口，可优先收敛到官方方案；否则保留当前组件，并继续检查移动端安全区、z-index 和 Turnstile 覆盖关系。 |
| 2026-06-02 | 运维监控缺少磁盘运行指标，管理员无法在同一面板直接判断容器磁盘压力；GPU 指标后续确认不需要展示。 | 新增迁移 `backend/migrations/145_add_ops_system_disk_gpu_metrics.sql` 扩展 `ops_system_metrics` 的磁盘字段；后端采集器 `backend/internal/service/ops_metrics_collector.go` 增加根文件系统磁盘用量采样；仓储与 DTO 扩展 `backend/internal/repository/ops_repo_metrics.go`、`backend/internal/service/ops_port.go`；前端 `frontend/src/views/admin/ops/components/OpsDashboardHeader.vue` 新增 Disk 卡片并补充 `frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts` 文案与 `frontend/src/api/admin/ops.ts` 类型。2026-06-14 后续移除 GPU 展示、采集、DTO 和数据库列，新增 `backend/migrations/157_remove_ops_gpu_metrics.sql`。 | `go test ./...`（backend）；`pnpm --dir frontend exec tsc --noEmit`；`pnpm --dir frontend exec vitest run src/views/admin/ops/components/__tests__/OpsOpenAITokenStatsCard.spec.ts src/views/admin/ops/components/__tests__/OpsErrorScopeCharts.spec.ts`。 | 同步官方后搜索 `disk_usage_percent`、`145_add_ops_system_disk_gpu_metrics.sql`、`157_remove_ops_gpu_metrics.sql`。若官方已提供等价磁盘字段/采集逻辑，优先收敛到官方实现；运维监控中继续不要恢复 GPU 卡片、`nvidia-smi` 采集或 `gpu_usage_percent` API 字段。 |
| 2026-06-02 | 运维告警规则缺少磁盘空间预警，无法在磁盘接近满载时提前通知。 | 后端规则白名单与评估器新增 `disk_usage_percent`（`backend/internal/handler/admin/ops_alerts_handler.go`、`backend/internal/service/ops_alert_evaluator_service.go`）；前端规则类型与配置项新增磁盘指标（`frontend/src/api/admin/ops.ts`、`frontend/src/views/admin/ops/components/OpsAlertRulesCard.vue`、`frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`）；新增迁移 `backend/migrations/146_seed_ops_disk_alert_rules.sql` 预置两条规则：85%/5分钟/P2 与 95%/3分钟/P1。 | `go test ./internal/service ./internal/handler/admin`；`pnpm --dir frontend exec tsc --noEmit`。 | 同步官方后搜索 `disk_usage_percent` 与 `146_seed_ops_disk_alert_rules.sql`；若官方已提供同等磁盘告警指标与种子规则，清理本地重复；若仅有面板展示无告警能力，保留本地预警规则并复跑告警创建/触发回归。 |
| 2026-06-02 | CC Switch 从 `ap1.upit.top` / `ap2.upit.top` / `api.upit.top` 导入服务商时，用量查询脚本会把已带 `/51Token/v1` 的 `api_base_url` 再拼一次 `/v1`，导致桌面版请求 `.../v1/v1/usage` 并显示“查询失败”；限额型 API Key 只返回 `rate_limits[].remaining` 时，旧 extractor 也会取不到剩余额度。 | 在 `frontend/src/views/user/KeysView.vue` 的 `executeCcsImport()` 内联生成 `usageUrl` 和 `usageScript`：`baseUrl` 去掉尾斜杠后拼接 `/usage`，不再写死 `{{baseUrl}}/v1/usage`；extractor 兼容 `remaining`、`quota.remaining`、`balance`、订阅窗口剩余量和 `rate_limits[].remaining`，确保钱包/订阅/限额型 Key 都能显示余额；`frontend/src/utils/ccswitchImport.ts` 继续只负责 deep link 参数组装。 | `pnpm --dir frontend typecheck`；在 CC Switch 桌面版打开“配置用量查询”，确认 `request.url` 为当前 `api_base_url` 对应的 `.../usage`，且不会出现 `.../v1/v1/usage`；刷新 `51token 算力`，限额型 Key 应显示类似 `剩余：40.00 USD`。 | 同步官方后搜索 `executeCcsImport`、`usageScript`、`rate_limits` 和 `/v1/usage`。如果官方后续也改为根据 `api_base_url` 直接拼 `/usage`，仍需确认 extractor 支持限额型 `rate_limits` 响应；若导入逻辑再次抽出页面层，需要重新验证 `api_base_url=https://<host>/51Token/v1` 时 CC Switch 脚本是否仍只生成一层 `/v1`。 |
| 2026-06-02 | `ap2/a2` 灰度环境并不复用 `sub2api-standby`，而是单独运行 `sub2api-ap2` compose；如果沿用 `--deploy` 的双环境滚动脚本，会误更新 standby / primary，而不是只更新 ap2。 | 线上确认 `ap2` 目录为 `/opt/sub2api-ap2-deploy`，通过 `.env` 中的 `IMAGE_TAG=` 控制镜像；单独部署 ap2 时，先用 `HOST=51token-vps deploy/local-gzip-binary-deploy.sh --apply` 完成本地构建、gzip 分块上传、远端解压和打镜像，再仅修改 `sub2api-ap2` 的 `.env` 并执行 `docker compose up -d sub2api-ap2`；2026-06-02 实际将 ap2 从 `sub2api:subapi-a5e4b0c6-ap2-oauth-adaptive-v2-202606021754` 升级到 `sub2api:subapi-9d75fb6b-ap2-redeploy-20260602232707`。 | `ssh 51token-vps 'grep ^IMAGE_TAG= /opt/sub2api-ap2-deploy/.env'`；`ssh 51token-vps 'docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" sub2api-ap2'`；`ssh 51token-vps 'curl -fsS http://127.0.0.1:8083/health'`；`ssh 51token-vps 'docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep sub2api-ap2'`。 | 同步官方或后续重构部署脚本后，优先确认 `ap2` 是否仍保持独立 compose 目录 `/opt/sub2api-ap2-deploy` 与 `IMAGE_TAG` 控制方式；如果未来把 ap2 并回 standby 或纳入统一脚本，先更新 `docs/VPS_DEPLOY_NOTES.md` 的单独部署步骤，再删除这条本地差异说明。 |
| 2026-06-03 | 首页和 API Key 使用弹窗里的 Claude/Anthropic 配置不能直接复用带路径的 OpenAI/Codex `api_base_url`；否则 Claude Code 会在该路径后拼 `/v1/messages` 并检查失败。 | 前台配置生成逻辑从公开设置里的 OpenAI/Codex base 派生 Claude base：`buildClaudeBaseUrl()` 和 `buildAnthropicBaseUrl()` 对绝对 URL 取 `origin`，因此 `https://ap1.upit.top/51Token/v1`、`https://api.upit.top/openai/v1` 等都会生成 `ANTHROPIC_BASE_URL=https://<host>`，Antigravity 则生成 `https://<host>/antigravity`。这属于前台配置问题，不需要修改 sub2api Go 路由，也不硬编码某个业务前缀。 | `pnpm --dir frontend run typecheck`；打开首页和 API Key 使用弹窗，确认 Claude/Anthropic 配置中的 `ANTHROPIC_BASE_URL` 是当前域名根，Codex/OpenAI 配置仍保留公开设置中的完整 API base。 | 同步官方后搜索 `buildClaudeBaseUrl` 和 `buildAnthropicBaseUrl`。如果官方改动了首页示例或 API Key 使用弹窗，必须继续确认 Claude/Anthropic 配置从带路径的 OpenAI/Codex base 派生时会取 URL origin。 |
| 2026-06-03 | Claude Code 配置示例缺少模型环境变量，用户复制 `~/.claude/settings.json` 后会落到 Claude 默认模型或错误模型，无法稳定使用 51token 的 GPT 模型。 | 首页 `frontend/src/views/home/components/homeData.ts` 与 API Key 使用弹窗 `frontend/src/components/keys/UseKeyModal.vue` 的 Claude 配置示例统一加入 `ANTHROPIC_MODEL=gpt-5.5` 和 `ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-5.5`；三语 README 的普通 Claude 与 Antigravity Claude 示例同步补充这两项。 | `pnpm --dir frontend run typecheck`；打开首页和 API Key 使用弹窗，确认 `~/.claude/settings.json`、Terminal、Command Prompt、PowerShell 示例都包含 `ANTHROPIC_MODEL` 与 `ANTHROPIC_DEFAULT_SONNET_MODEL`。 | 同步官方后搜索 `ANTHROPIC_MODEL`、`ANTHROPIC_DEFAULT_SONNET_MODEL`、`generateAnthropicFiles` 和 `buildClaudeConfigJson`，确认模型字段没有被上游示例覆盖丢失。 |
| 2026-06-04 | 订阅管理列表的“重置配额”原本只能一次性重置日/周/月三种用量；运营需要只重置每日、每周、每月或全部额度，避免误清其它周期用量，且默认只选每日以降低误清范围。 | 将 `frontend/src/views/admin/SubscriptionsView.vue` 的重置配额确认框改为可选范围弹窗，新增 `daily`、`weekly`、`monthly`、`all` 单选状态，弹窗打开/关闭/提交后默认回到 `daily`，并按选择向既有 `resetQuota` 接口发送 `{ daily, weekly, monthly }`；补充中英文文案 `resetQuotaScopes` 和 `resetQuotaScopeDescriptions`。 | `pnpm --dir frontend run typecheck`；`pnpm --dir frontend exec eslint src/views/admin/SubscriptionsView.vue src/i18n/locales/en.ts src/i18n/locales/zh.ts`；`pnpm --dir frontend run build`。 | 同步官方后搜索 `resetQuotaScope`、`resetQuotaScopeOptions`、`resetQuotaScopes`。如果官方已提供等价范围选择，优先收敛到官方组件；否则保留本地弹窗逻辑，并确认后端 `AdminResetQuota` 仍支持三个布尔字段。 |
| 2026-06-04 | 订阅管理列表刷新后仅显示列表接口里的用量窗口快照，运营需要刷新列表后自动逐个查询当前页订阅的最新用量窗口，减少手工进入详情或等待旧数据的成本。 | `frontend/src/views/admin/SubscriptionsView.vue` 在 `loadSubscriptions()` 成功后，以 3 并发对当前页有日/周/月限额的订阅调用 `/admin/subscriptions/:id/progress`，并用批次 token 防止翻页/筛选后的旧请求覆盖新列表；新增 `subscriptionProgressMerge.ts` 纯函数，将后端 `used_usd/window_start` 合并回列表行，同时保留旧字段兼容。 | `pnpm --dir frontend exec vitest run src/views/admin/__tests__/subscriptionProgressMerge.spec.ts`；`pnpm --dir frontend exec eslint src/views/admin/SubscriptionsView.vue src/views/admin/subscriptionProgressMerge.ts src/views/admin/__tests__/subscriptionProgressMerge.spec.ts src/types/index.ts`；`pnpm --dir frontend run typecheck`。 | 同步官方后搜索 `refreshVisibleSubscriptionProgress`、`mergeSubscriptionProgressById` 和 `getProgress`。如果官方改造订阅列表或 progress 接口，继续保留“列表刷新后自动查询当前页进度，且旧批次不能覆盖新列表”的行为。 |
| 2026-06-04 | 订阅管理批量分配弹窗的“最近注册用户”和搜索结果只显示邮箱与 ID，运营无法快速用用户名或备注辨认用户；最近注册列表还需要只展示最近 2 天用户，但搜索必须仍支持所有用户。 | 扩展管理员用量搜索用户接口 `/admin/usage/search-users`，在简化用户结果中返回 `username` 和 `notes`；前端 `SimpleUser` 类型、最近注册用户映射和 `SubscriptionsView.vue` 用户选择列表同步展示一行辅助信息，优先显示备注，备注为空时显示用户名，已选用户标签也显示相同副信息；新增 `subscriptionRecentUsers.ts` 过滤最近注册列表为 48 小时内用户，搜索接口不加时间限制。 | `go test ./internal/handler/admin -run TestUsageSearchUsersIncludesDisplayFields`；`pnpm --dir frontend exec vitest run src/api/__tests__/admin.usage.spec.ts src/views/admin/__tests__/subscriptionRecentUsers.spec.ts`；`pnpm --dir frontend run typecheck`；`pnpm --dir frontend exec eslint src/views/admin/SubscriptionsView.vue src/views/admin/subscriptionRecentUsers.ts src/views/admin/__tests__/subscriptionRecentUsers.spec.ts src/api/admin/usage.ts src/api/__tests__/admin.usage.spec.ts`。 | 同步官方后搜索 `SearchUsers`、`SimpleUser`、`formatSimpleUserMeta`、`toSimpleUser` 和 `filterRecentRegisteredUsers`。如果官方重构订阅分配弹窗或用户搜索接口，必须继续确保最近注册列表只展示最近 2 天用户，而搜索仍覆盖所有用户。 |
| 2026-06-04 | 订阅管理主列表用户列只能看到邮箱或用户名，列表行里看不到管理员备注，运营需要在列表中直接识别备注信息；首次前端展示改动后，后端 admin 订阅列表仍返回普通 `user` DTO，导致接口 JSON 缺少嵌套 `user.notes`。 | 将 `frontend/src/views/admin/SubscriptionsView.vue` 用户列改为两行显示：第一行保持当前 email/username 列模式，第二行仅在用户 `notes` 非空时显示备注；新增 `subscriptionUserDisplay.ts` 纯函数和单测，并补充 `UserSubscription.user` 的管理员备注类型；后端 `AdminUserSubscription` 改为返回 `AdminUser` 形态的嵌套用户，并在 mapper 单测覆盖 `user.notes`。 | `go test ./internal/handler/dto -run TestUserSubscriptionFromServiceAdmin_IncludesNestedUserNotes`；`go test ./internal/handler/dto`；`go test ./internal/handler/admin`；`pnpm --dir frontend exec vitest run src/views/admin/__tests__/subscriptionUserDisplay.spec.ts`；`pnpm --dir frontend run typecheck`；`pnpm --dir frontend exec eslint src/views/admin/SubscriptionsView.vue src/views/admin/subscriptionUserDisplay.ts src/views/admin/__tests__/subscriptionUserDisplay.spec.ts src/types/index.ts`。 | 同步官方后搜索 `getSubscriptionUserNotes`、`getSubscriptionUserLabel`、`AdminUserSubscription` 和 `UserSubscriptionFromServiceAdmin`。如果官方重构订阅列表或 admin DTO，继续保留“备注单独换行且空备注不显示”，并确认 admin 订阅列表接口仍返回嵌套 `user.notes`。 |
| 2026-06-04 | 账号状态倒计时偶发显示 `common.time.countdown.daysHours` 等翻译 key，说明倒计时格式化在语言包未就绪或旧包缺少 key 时会把 key 暴露到界面。 | `frontend/src/utils/format.ts` 为 `formatCountdown()` 和 `formatCountdownWithSuffix()` 增加 `translateOrFallback()` 兜底：i18n 返回原 key 或包含该 key 的缺失提示时改用内置短格式 `Xd Yh`、`Xh Ym`、`Xm` 和 `{time} to lift`；新增 `formatCountdown.spec.ts` 覆盖缺失消息时不会显示 key。 | `pnpm --dir frontend exec vitest run src/utils/__tests__/formatCountdown.spec.ts`；`pnpm --dir frontend exec eslint src/utils/format.ts src/utils/__tests__/formatCountdown.spec.ts`；`pnpm --dir frontend run typecheck`。 | 同步官方后搜索 `formatCountdown`、`formatCountdownWithSuffix` 和 `translateOrFallback`。如果官方重构时间格式化，必须继续保证倒计时缺失翻译时不会把 `common.time.countdown.*` key 暴露给用户。 |
| 2026-06-04 | fork 维护护栏只检查一组写死的 protected 路径，而且只会阻止提交；订阅配额范围选择这类本地改动不在旧路径表里，所以提交时没有自动写入维护文档。 | 将 `tools/fork-maintenance/fork_maintenance.py` 的 `check-doc` 改为基于 Git staged 文件判断：非 merge/rebase/cherry-pick 流程中，只要 staged 本地改动不是维护文档自身、临时目录或构建产物，就自动追加一条待完善记录并 `git add docs/FORK_MAINTENANCE_CN.md`；同步更新 README 和本文档说明。`verify-after-upstream` 对已不存在的 favicon 测试改为显式 skip，避免 amend 后 post-rewrite 假失败。 | `python3 -m py_compile tools/fork-maintenance/fork_maintenance.py`；`tools/fork-maintenance/fork-maintenance.sh record --title "测试自动记录" --dry-run`；`tools/fork-maintenance/fork-maintenance.sh check-doc`；`tools/fork-maintenance/fork-maintenance.sh verify-after-upstream --skip-build`；`rg -n "PROTECTED_PATTERNS|is_protected|protected fork|protected paths|protected fork-maintenance" tools/fork-maintenance`。 | 同步官方后搜索 `check-doc`、`changed_files_from_index`、`upstream_sync_in_progress`、`IGNORED_RECORD_PATTERNS`。如果官方新增自己的 fork 维护机制，保留“非上游同步 staged 本地改动自动记录”的行为，不再恢复固定 protected 路径表。 |
| 2026-06-05 | 分组真实费率倍数需要继续参与实际扣费，但用户端不应看到真实倍率；运营需要在分组管理中单独配置用户看到的“展示倍率”，默认 1 倍。 | 新增 `groups.display_rate_multiplier` 迁移与 ent/service/DTO 字段，管理员接口继续用 `rate_multiplier` 表示真实计费倍率，并额外返回 `display_rate_multiplier` 与 `billing_rate_multiplier`；普通用户分组 DTO、支付 checkout、可用渠道接口返回的 `rate_multiplier` 改为展示倍率。前端分组创建/编辑弹窗新增“用户显示倍率”输入框，默认 1；用户端 Payment、Keys、Available Channels 通过 `getGroupDisplayRateMultiplier()` 展示该倍率，Usage 历史日志仍显示请求实际倍率。 | `go test ./internal/handler/dto ./internal/handler ./internal/service -run 'TestGroupFromService_UsesDisplayRateMultiplierForUserDTO|Test.*Group|Test.*Available|Test.*Checkout'`；`pnpm --dir frontend exec vitest run src/utils/__tests__/groupDisplayRate.spec.ts`；`pnpm --dir frontend exec eslint src/views/admin/GroupsView.vue src/views/user/KeysView.vue src/views/user/PaymentView.vue src/components/channels/AvailableChannelsTable.vue src/utils/groupDisplayRate.ts src/utils/__tests__/groupDisplayRate.spec.ts src/types/index.ts src/types/payment.ts src/api/channels.ts`；`pnpm --dir frontend run typecheck`。 | 同步官方后搜索 `display_rate_multiplier`、`DisplayRateMultiplier`、`BillingRateMultiplier` 和 `getGroupDisplayRateMultiplier`。如果官方重构分组倍率或用户端分组展示，必须继续保持“真实计费倍率与用户展示倍率分离，展示倍率默认 1 且不参与扣费”。 |
| 2026-06-05 | a2 灰度启用动态模型路由后，管理员用量记录需要同时看到用户请求模型和真实上游模型，并清楚区分；普通用户仍不能看到真实上游模型。 | 新增 `formatAdminUsageModel()`，管理员用量表格模型列显示为 `请求模型 (真实上游模型)`，仅当 `upstream_model` 存在且不同于 `model` 时追加括号；管理员导出 Excel 的模型列也使用同一格式，同时保留单独上游模型列便于分析。用户用量页面和用户 DTO 不改。 | `pnpm --dir frontend exec vitest run src/utils/__tests__/usageModelDisplay.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts`；`pnpm --dir frontend exec eslint src/components/admin/usage/UsageTable.vue src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/admin/UsageView.vue src/utils/usageModelDisplay.ts src/utils/__tests__/usageModelDisplay.spec.ts`；`pnpm --dir frontend run typecheck`。 | 同步官方后搜索 `formatAdminUsageModel`、`upstream_model`、`UsageTable` 和 `UsageView`。如果官方重构管理员用量记录，继续保持“管理员可见请求模型与真实上游模型，普通用户不可见真实上游模型”的边界。 |
| 2026-06-05 | a2 动态模型路由开启后，短的多条 input 请求仍经常显示为 `gpt-5.5` 或被升到 `gpt-5.4`，不符合“默认应走 `gpt-5.3-codex-spark`”的灰度策略。 | 修复 `backend/internal/service/openai_model_router.go` 的复杂度统计：当 `/responses` 同时传入原始 JSON `rawBody` 和已解析 `reqBody` 时，不再重复累计同一份 `input` 与 `instructions`；同时 balanced 档只按文本长度阈值判断，不再因短 input 条数达到 `complex_input_min_items` 就升到 `gpt-5.4`。`backend/internal/service/openai_model_router_test.go` 覆盖短多条 input 默认走 `gpt-5.3-codex-spark`、长文本走 `gpt-5.4`、超大文本走 `gpt-5.5`。 | `go test ./internal/service -run 'TestOpenAIModelRouter(ComplexTextDoesNotDoubleCountParsedBody|ShortMultiItemRequestUsesDefaultTier|LongTextRequestUsesBalancedTier|PremiumTextUsesPremiumTier)' -count=1`；`go test ./internal/service -count=1`；`git diff --check`。 | 同步官方后搜索 `isOpenAIModelRouterComplexText`、`ComplexInputMinItems` 和 `openai_model_router_test.go`。如果官方重构路由复杂度统计，必须保留“短的普通多条 input 默认走 `gpt-5.3-codex-spark`，长文本才升 `gpt-5.4`，超大/图片/视觉才升 `gpt-5.5`”的行为，并在 a2 灰度环境用短多条 input 请求复查用量日志里的真实上游模型。 |
| 2026-06-05 | 管理员用户列表的用户名列看不到备注，运营需要在用户名后直接看到备注以便快速辨认用户。 | `frontend/src/views/admin/UsersView.vue` 新增 `formatUsernameWithNotes()`，用户名列在 `notes` 非空时显示为 `用户名(备注)`，空备注保持原用户名或 `-`；备注独立列继续保留。`frontend/src/views/admin/__tests__/UsersView.spec.ts` 增加回归测试覆盖备注拼接格式。 | `pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsersView.spec.ts`；`pnpm --dir frontend exec eslint src/views/admin/UsersView.vue src/views/admin/__tests__/UsersView.spec.ts`；`pnpm --dir frontend run typecheck`。 | 同步官方后搜索 `formatUsernameWithNotes`、`cell-username` 和 `UsersView.spec.ts`。如果官方重构用户列表表格，继续保持“用户名列有备注时追加括号备注、无备注不显示括号”的展示规则。 |
| 2026-06-06 | 需要在本机不依赖 Docker 复现 `ap2` 数据并启动服务；同时在“订阅/账号/API Key/使用记录”搜索下拉改造后，前端新增共享搜索组件引入了 `unknown` 泛型回调报错和 `admin-accounts` 打包体积膨胀。 | 1. 线上通过 SSH 从 `sub2api-ap2-postgres` 导出 custom dump，落到本地 `/Users/okk/git-projects/sub2api/.local/ap2-db/sub2api-ap2.dump`，并用 `sha256` 校验。2. 本机保留现有 PostgreSQL 16 服务不动，另外安装 `postgresql@18` 和 `redis`，在 `.local/pg18` 初始化独立 PG18 data dir，并用 `pg_ctl -o "-p 5433"` 启动；再将 dump 恢复到本地 `sub2api_ap2`。3. 在 `.local/ap2-runtime/config.yaml` 写入本地直连配置，后端使用 `go build -tags embed -o .local/ap2-runtime/sub2api-embed ./cmd/server` 编译并通过 `DATA_DIR=.local/ap2-runtime` 在 `127.0.0.1:18083` 启动，`/health` 和 `/` 均可访问。4. 前端共享搜索组件 `frontend/src/components/common/SearchSuggestInput.vue` 将 `SearchSuggestOption` / `select` 事件的泛型参数从 `unknown` 放宽，修复 `SubscriptionsView` 的 `SimpleUser.deleted` 回填与 `frontend/src/api/admin/usage.ts` 缺失的 `average_first_token_ms` 字段。5. 为降低账号管理首屏包体积，`frontend/src/i18n/index.ts` 去掉对 `router/title/app store` 的动态导入，`frontend/src/views/admin/AccountsView.vue` 将创建/编辑/批量编辑/同步/测试/统计/重授权/计划面板/错误透传/TLS 指纹等非首屏弹窗改为 `defineAsyncComponent` 按需加载；`frontend/vite.config.ts` 新增 `admin-accounts`、`admin-users`、`admin-usage`、`admin-subscriptions`、`user-keys` 手动分包。 | `shasum -a 256 .local/ap2-db/sub2api-ap2.dump`；`/opt/homebrew/opt/postgresql@18/bin/pg_isready -h 127.0.0.1 -p 5433`；`curl -fsS http://127.0.0.1:18083/health`；`curl -I -s http://127.0.0.1:18083/`；`pnpm --dir frontend run build`。结果：本地 `sub2api` 可无 Docker 直连本地 PG18/Redis 运行；`pnpm build` 通过；`i18n` 的 dynamic import warning 消失；`admin-accounts` 从约 960 kB 降到约 533 kB，但仍略高于 500 kB。 | 同步官方后搜索 `.local/ap2-runtime`、`sub2api-ap2.dump`、`SearchSuggestInput`、`average_first_token_ms`、`defineAsyncComponent(() => import('@/components/account/`、`admin-accounts`、`manualChunks`。如果官方后续为搜索建议输入框补了泛型透传或为账号管理页面引入按需加载，优先收敛到官方实现；若需要再次本地复现 `ap2` 数据，继续使用“PG18 独立 5433 + Redis + `go build -tags embed`”这条非 Docker 路线，避免和本机已有 PG16 冲突。 |
| 2026-06-06 | 本地继续收口用户端视觉与配置示例：深色模式下登录后左上角 logo 中黑色 `∞` 在深色背景里不易辨认；“使用 API 密钥”弹窗在移动端 client tabs 会横向挤出；Claude/Anthropic 示例里的 `ANTHROPIC_BASE_URL` 需要固定为 `域名/51Token` 且明确“不带 /v1”，`~/.claude/settings.json` 还缺少完整模型相关字段。 | 1. `frontend/src/components/layout/AppSidebar.vue` 给登录后侧边栏左上角图片 logo 容器补白底和轻描边，避免透明底黑色 `∞` 在深色侧边栏里“消失”；首页公用 SVG logo `frontend/src/views/home/components/SiteLogo.vue` 也补 `dark:text-white`，确保静态首页头部在深色模式下可见。2. `frontend/src/components/keys/UseKeyModal.vue` 将 `Client Tabs` 从单行 `flex space-x-6` 改成 `flex-wrap + gap-x/gap-y`，移动端超出自动换行。3. `UseKeyModal.vue` 与首页示例共用的 `frontend/src/views/home/components/homeApiBase.ts` 统一修正 Claude/Anthropic base 派生规则：从带路径的 OpenAI/Codex `api_base_url` 推导时生成 `https://<host>/51Token`，而不是仅取域名根，也不附带 `/v1`。4. API Key 使用弹窗中的 Claude Code 示例继续补全：Terminal / CMD / PowerShell 的 `ANTHROPIC_BASE_URL` 旁新增“这里没有v1”注释，`~/.claude/settings.json` 示例补充 `ANTHROPIC_DEFAULT_HAIKU_MODEL`、`ANTHROPIC_DEFAULT_OPUS_MODEL`、`ANTHROPIC_REASONING_MODEL`，并保留 `CLAUDE_CODE_ATTRIBUTION_HEADER` 与 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`。 | `pnpm --dir frontend run build`；`curl -fsS http://127.0.0.1:18083/health`；本地重启 `.local/ap2-runtime/sub2api-embed` 后，手工检查登录后深色模式左上角 logo、移动端“使用 API 密钥”弹窗 tabs 自动换行、Claude/Anthropic 示例中的 `ANTHROPIC_BASE_URL=https://<host>/51Token` 且注释明确“不带 /v1”。 | 同步官方后搜索 `AppSidebar.vue`、`SiteLogo.vue`、`UseKeyModal.vue`、`buildClaudeBaseUrl`、`buildAnthropicBaseUrl`、`ANTHROPIC_REASONING_MODEL`。如果官方后续重构登录后侧边栏、首页集成示例或 API Key 使用弹窗，必须继续确认：1）深色模式 logo 在深背景可辨认；2）移动端 tabs 不横向溢出；3）Claude/Anthropic 配置始终输出 `域名/51Token` 且明确这里没有 `/v1`；4）`~/.claude/settings.json` 保持完整模型字段。 |
| 2026-06-07 | 已提交的前端显示修复在移动端仍需可追溯：`frontend/index.html` 调整 viewport 为固定缩放比例，避免小屏下误缩放；`frontend/src/views/admin/UsersView.vue` 调整用户名显示与布局避免溢出。 | 本次提交 `399bb8fc chore: refine mobile and users view layout`（后续 `v1` 分支）补齐这两项轻量体验改动，避免影响功能逻辑。 | `pnpm --dir frontend run build`；登录后台后在 iOS/Android 用户管理页验证横竖屏下用户名显示与列表不会挤压。 | 同步官方后搜索 `UsersView.vue` 的 `cell-username` 布局与 `frontend/index.html` 的 viewport 配置。若官方已同步同类修复可标记为已被替代，否则保留。 |
| 2026-06-07 | `image_or_vision_force_premium` 的外部配置默认从 `true` 改为 `false`，避免图片/视觉请求不必要地直接走 premium。 | 后端默认值改为 `false`，并同步更新 `deploy/*.yml`、`.local/ap2-runtime/*.yml`、`.local/ap2-runtime/.env`、`.local/ap2-runtime/config.yaml` 及 `docs/VPS_DEPLOY_NOTES.md` 中示例默认值，避免 compose/env 层再次回写 `true`。 | `rg -n "GATEWAY_MODEL_ROUTER_IMAGE_OR_VISION_FORCE_PREMIUM|image_or_vision_force_premium"` 全量复核；容器运行后执行 `docker exec <container> env | grep GATEWAY_MODEL_ROUTER_IMAGE_OR_VISION_FORCE_PREMIUM` 确认实际值。 | 线上同步时先确认该 env 变量是否有额外注入；若需要回退可在单环境 `.env` 明确覆盖。 |
| 2026-06-07 | 账号管理的 OpenAI OAuth 用量窗口需要默认只显示主套餐，Spark 套餐要通过下拉箭头展开；同时本地 `http://localhost:8317/v0/management/api-call` 这条管理接口可用于排查 cliproxyapi 的管理态调用，但它需要管理密钥，不是公开 GET 数据面。 | `frontend/src/components/account/AccountUsageCell.vue` 现在默认收起 Spark 区块，展开后同时展示 `codex_primary` / `codex_secondary` 两份用量，并在 OpenAI 用量刷新键变化时清理缓存，避免旧快照残留；后端 `backend/internal/service/account_usage_service.go` 与 `backend/internal/service/openai_gateway_service.go` 负责把两份 codex 快照写回 usage 结果。 | 本地复查 `curl http://127.0.0.1:8317/v0/management/api-call`，GET 返回 404；未带管理密钥时 POST 返回 401 `missing management key`，说明管理接口需要通过 `X-Management-Key` 授权。 | 同步官方后搜索 `codex_primary`、`codex_secondary`、`openAIUsageRefreshKey`、`/v0/management/api-call` 和 `missing management key`。如果官方后续把 Spark 套餐和主套餐拆回单一窗口，继续保留“默认收起、点击再展开”与“刷新时不要吃旧缓存”的交互。 |
| 2026-06-08 | 当前工作区 staged 改动需要完整纳入 fork 维护清单，避免只记录其中某个单点修复。 | 本次 staged 改动分三组：1) OpenAI 动态模型路由修正，涉及 `backend/internal/config/config.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_model_router.go`、`backend/internal/service/openai_model_router_test.go`，短多条 input 默认走 `gpt-5.3-codex-spark`，长文本才升 `gpt-5.4`，超大文本才升 `gpt-5.5`；2) OpenAI OAuth Spark 用量窗口主动探测修正，涉及 `backend/internal/service/account_usage_service.go`、`backend/internal/service/account_usage_service_test.go`，主动查询固定请求 `gpt-5.3-codex-spark` 并解析 `X-Codex-*` 快照；3) Claude Code 配置示例修正，涉及 `frontend/src/components/keys/UseKeyModal.vue`、`frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`、`frontend/src/views/home/components/homeApiBase.ts`、`frontend/src/views/home/components/homeData.ts`、`frontend/src/views/home/components/__tests__/homeApiBase.spec.ts`，`ANTHROPIC_BASE_URL` 保留 `/51Token` 且不带 `/v1`，并补齐 Claude Code 相关模型环境变量。 | `go test ./internal/service -run 'TestOpenAIModelRouter(ComplexTextDoesNotDoubleCountParsedBody|ShortMultiItemRequestUsesDefaultTier|LongTextRequestUsesBalancedTier|PremiumTextUsesPremiumTier)' -count=1`；`go test ./internal/service -run 'Test(ShouldRefreshOpenAICodexSnapshot|ExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders|BuildOpenAICodexProbePayloadUsesSparkModel|AccountUsageService_.*OpenAI|BuildCodexUsageProgressFromExtra)' -count=1`；`pnpm --dir frontend exec vitest run src/views/home/components/__tests__/homeApiBase.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts`；`pnpm --dir frontend run typecheck`；`git diff --check`。 | 同步官方后按文件组复查：动态路由继续搜索 `isOpenAIModelRouterComplexText`、`ComplexInputMinItems`、`openai_model_router_test.go`；Spark 用量继续搜索 `buildOpenAICodexProbePayload`、`openAICodexProbeModel`、`codex_5h`、`codex_7d`；Claude Code 示例继续搜索 `buildClaudeBaseUrl`、`buildAnthropicBaseUrl`、`ANTHROPIC_BASE_URL`、`ANTHROPIC_REASONING_MODEL`。若官方已有等价实现，可收敛到官方；否则保留这三组本地行为。 |
| 2026-06-08 | OpenAI OAuth 显式 spark 配额路由、Spark/主套餐额度展示与 token 时效存在一组线上问题：5.3 spark 不能发图片却依赖 spark 优先、Spark 用量显示经常误绑主套餐、`Access Token` 过期过快。 | 1) 后端在 `backend/internal/service/account_usage_service.go` 引入 Codex 额度窗口标准化：增加 `codex_5h_*` 与 `codex_7d_*` 字段映射，并让 `/admin/accounts` 用量面板能稳定读到 `codex_secondary`/`codex_primary` 的上游头部快照；2) `openAIUsage` 兼容 `X-Codex-*` 百分比/秒/窗口字段，包含 `X-Codex-Secondary-Used-Percent`、`X-Codex-Secondary-Reset-After-Seconds`、`X-Codex-Secondary-Window-Minutes`、`X-Codex-Primary-Over-Secondary-Limit-Percent`；3) `backend/internal/handler/admin/account_handler.go` 增加 `debug_usage=1` 查询以便联调时输出原始 usage 头部；4) `backend/internal/config/config.go` 默认 token 时效改为 30 天；5) 前端 `frontend/src/components/account/AccountUsageCell.vue` 与 `frontend/src/types/index.ts` 改为默认折叠 Spark，展开后显示 Spark 使用量窗口，并与刷新键一并更新副套餐；6) 多语言文案同步更新 `Spark 使用量`；7) 用量窗口主动探测新增 `buildOpenAICodexProbePayload()`，固定使用 `gpt-5.3-codex-spark`，不再复用可能变成 `gpt-5.4` 的 OpenAI 账号通用测试模型。 | `rg -n "codex_5h|codex_7d|openAICodexSparkWindows|debug_usage|AccessTokenTTL|image_or_vision_force_premium|buildOpenAICodexProbePayload|openAICodexProbeModel" backend frontend`；`pnpm --dir frontend exec vue-tsc --noEmit --pretty false`；`go test ./internal/service -run 'Test(ShouldRefreshOpenAICodexSnapshot|ExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders|BuildOpenAICodexProbePayloadUsesSparkModel|AccountUsageService_.*OpenAI|BuildCodexUsageProgressFromExtra)' -count=1`；`git diff --check`。 | 同步官方后逐项确认 `account_usage_service.go` 的 `setCodexUsageSnapshotFields`、`buildOpenAICodexProbePayload`、`openAICodexProbeModel`、`openai_gateway_service.go` 的 `ParseCodexRateLimitHeaders`、`AccountUsageCell.vue` 的 Spark 展开/刷新流程、`config.go` 的 `AccessTokenTTL`。如果官方已统一返回可直接消费的 Spark 专用窗口字段且前端已内建同类折叠策略，可清理本地回退逻辑；否则保留该兼容层并持续复核 `spark` 是否会被主套餐快照污染，用量窗口主动查询也必须继续请求 `gpt-5.3-codex-spark`。 |
| 2026-06-08 | a1 环境长期沿用 `standby` 命名，容易和真正热备/回滚环境混淆；后续部署脚本也会继续寻找旧 `/opt/sub2api-standby-deploy`。 | 线上将 `/opt/sub2api-standby-deploy` 无损迁移为 `/opt/sub2api-ap1-deploy`，容器改名为 `sub2api-ap1`、`sub2api-ap1-postgres`、`sub2api-ap1-redis`，保留宿主机 `8082` 与 `a1.upit.top` 路由不变；`deploy/local-gzip-binary-deploy.sh` 默认滚动目标改为 ap1 + primary，并保留旧 `STANDBY_*` 环境变量兼容；`tools/fork-maintenance/fork_maintenance.py` 的登录条款恢复目标同步改为 `sub2api-ap1-postgres`；`docs/VPS_DEPLOY_NOTES.md` 和本文档更新当前命名。 | `deploy/local-gzip-binary-deploy.sh --skip-frontend-build --skip-backend-build --deploy` dry-run 确认目标为 `/opt/sub2api-ap1-deploy` 与 `sub2api-ap1`；`ssh 51token-vps 'docker inspect ... sub2api-ap1 sub2api-ap1-postgres sub2api-ap1-redis'` 均为 `healthy`；`curl -fsS http://127.0.0.1:8082/health`；`curl -fsS https://a1.upit.top/health`；`bash -n deploy/local-gzip-binary-deploy.sh`；`python3 -m py_compile tools/fork-maintenance/fork_maintenance.py`；`git diff --check`。 | 同步官方或后续调整部署拓扑后，继续搜索 `sub2api-standby`、`STANDBY_COMPOSE_DIR` 和 `/opt/sub2api-standby-deploy`。历史记录可保留旧名，但当前部署脚本、恢复脚本和线上操作说明必须默认指向 `sub2api-ap1`。 |
| 2026-06-08 | 账号管理页默认筛选为“全部状态”，运营首次进入时会混入停用、异常和不可调度账号，不利于优先处理正常可用账号。 | `frontend/src/views/admin/AccountsView.vue` 将账号列表初始 `status` 改为 `active`，让状态筛选默认选中“正常”；保留筛选下拉里的“全部状态”选项，运营仍可手动切回全量。`frontend/src/views/admin/__tests__/AccountsView.searchSuggest.spec.ts` 增加首次加载默认带 `status: active` 的回归测试。 | `pnpm --dir frontend exec vitest run src/views/admin/__tests__/AccountsView.searchSuggest.spec.ts`；`pnpm --dir frontend run typecheck`；`git diff --check`。 | 同步官方后搜索 `AccountsView.vue` 的 `initialParams.status` 和 `AccountsView.searchSuggest.spec.ts`。如果官方重构账号筛选或表格加载逻辑，继续保持“首次进入默认只看正常账号，但允许手动切回全部状态”的运营口径。 |
| 2026-06-08 | 账号管理用量窗口里 Spark 使用量仍和主套餐显示相同；cliproxyapi 可单独查询 Spark，说明本地把两种套餐快照混到同一组字段。 | 将 OpenAI Codex quota 快照按模型族分开：请求/探测模型为 `gpt-5.3-codex-spark` 时写 `codex_5h_*` / `codex_7d_*` 与 `codex_usage_updated_at`，非 Spark Codex 主套餐模型写 `codex_main_5h_*` / `codex_main_7d_*` 与 `codex_main_usage_updated_at`。`AccountUsageService` 主套餐区域只优先从 `codex_main_*` 构造 `five_hour/seven_day`，不得把 Spark 的 `codex_*` 历史快照提升为主套餐；Spark 展开区读取明确 Spark 字段，并避免在缺少 Spark 更新时间时被 raw primary/secondary 兜底污染。 | `go test ./internal/service -run 'TestBuildCodexUsageExtraUpdates|TestAccountUsageService_GetOpenAIUsage|TestExtractOpenAICodexProbeUpdates|TestHandle429_OpenAI|TestOpenAIModelRouter|TestBuildOpenAICodexProbePayload|TestCodexSnapshotBaseTime|TestCodexResetAtRFC3339' -count=1`；`go test ./internal/service -count=1`；`pnpm --dir frontend exec vitest run src/components/account/__tests__/AccountUsageCell.spec.ts src/views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts`；`pnpm --dir frontend run typecheck`；`git diff --check`。 | 同步官方后搜索 `codex_main_5h`、`codex_main_7d`、`buildCodexUsageExtraUpdatesForFamily`、`openAICodexUsageFamilyForModel`、`openAIMainFiveHour` 和 `openAICodexSparkWindows`。如果官方提供独立 Spark/主套餐管理接口，可收敛主动探测实现，但必须保留“模型族决定写入字段、Spark 不被主套餐或 raw 兜底覆盖、主套餐不吃 Spark 快照”的边界。 |
| 2026-06-13 | 账号管理列表中 OpenAI OAuth 主套餐周用量与 Spark 周限额用量仍可能反显示；同时运营排查单个账号时缺少直观账号 ID 列。 | 前端 `AccountUsageCell` 的主套餐 5h/7d 展示改为优先读取 `codex_main_5h_*` / `codex_main_7d_*`，只有缺少主套餐快照时才回退通用 `five_hour` / `seven_day`，避免通用字段里残留的 Spark 窗口覆盖主套餐；Spark 展开区继续读取明确的 `codex_5h_*` / `codex_7d_*`，但不再在缺少 `codex_usage_updated_at` 时用 raw `codex_primary/secondary` 兜底。账号管理列表新增固定显示的 `ID` 列，便于定位账号 17 等具体账号。补充 `AccountUsageCell.spec.ts` 复现“主套餐 22%、Spark 0%、通用 seven_day 95%”以及“只有 raw 头时不显示 Spark”的错位场景，并补充 `AccountsView.usageWindowsHint.spec.ts` 覆盖 ID 列顺序。 | `pnpm --dir frontend exec vitest run src/components/account/__tests__/AccountUsageCell.spec.ts`；`pnpm --dir frontend exec vitest run src/views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts`；`pnpm --dir frontend run typecheck`；`git diff --check`。 | 同步官方后搜索 `openAIMainFiveHour`、`openAIMainSevenDay`、`codex_main_7d_used_percent`、`codex_usage_updated_at`、`admin.accounts.columns.id` 和 `cell-id`。如果官方重构 usage API 或账号列表列配置，必须继续保持“主套餐优先读 `codex_main_*`、Spark 只读明确 Spark 字段、账号 ID 默认可见”的排查口径。 |
| 2026-06-09 | OpenAI 路由内二次账单检查仍有“额度已超限后直接拒绝”问题：`/v1/chat/completions`、`/v1/images/*`、`/v1/embeddings` 分支未复用中间件接力结果，导致同一请求仍被 2 次额度拦截。 | 在 `backend/internal/handler/openai_gateway_handler.go` 增加统一兜底 `enforceBillingEligibilityWithFallback` 与 `billingErrorDetailsWithFallback`，并在 `NewOpenAIGatewayHandler` 注入同一 `subscriptionService`（`backend/cmd/server/wire_gen.go`）；OpenAIGateway `ChatCompletions`、`Embeddings`、`Images` 改为统一调用该兜底路径，在二次检查失败且命中限额错误时触发 `ResolveQuotaFallback` 后重试。 | `go test ./internal/handler -run TestEnforceBillingEligibilityWithFallback -count=1`；`go test ./internal/handler -count=1`；`go test ./... -run TestNonExistent -count=1`；`ssh 51token-vps 'grep ^IMAGE_TAG= /opt/sub2api-ap2-deploy/.env'`；`ssh 51token-vps 'docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" sub2api-ap2'`；`ssh 51token-vps 'docker inspect -f "{{.Config.Image}}" sub2api-ap2'`；`ssh 51token-vps 'curl -fsS http://127.0.0.1:8083/health'`；`git diff --check`。| 已在 ap2 执行：`IMAGE_TAG=sub2api:subapi-aebdd6f1-a2-redeploy-20260609075235`；`docker inspect -f` 健康检查返回 `healthy`；`docker inspect -f '{{.Config.Image}}'` 返回同镜像；`curl` 返回 `{"status":"ok"}`；`docker compose` 为 `sub2api-ap2` 使用该镜像。 | 同步官方后核对 `OpenAIGatewayHandler` 在 `ChatCompletions`/`Embeddings`/`Images` 二次检查路径是否仍有接力重试逻辑；并复核 `subscriptionService` 注入链路（`wire_gen.go`）、`ResolveQuotaFallback` 与 `GetGroup`/`GetQuotaFallback` 上游行为。新增测试文件 `backend/internal/handler/openai_chat_completions_billing_fallback_test.go` 建议保留作为回归。 |
| 2026-06-11 | 上游新增管理员合规确认弹窗后，线上后台每个管理员首次进入都需要逐字确认；当前 fork 不需要该强制确认流程，后续部署也不能因为未写 `settings.admin_compliance_acknowledgement:*` 或版本号变化而再次弹窗。 | 保留上游合规文档、API 和前端组件，但后端 `backend/internal/service/admin_compliance.go` 增加 `AdminComplianceEnabled=false`，让 `GetAdminComplianceStatus` 默认返回 `required=false`，`AcceptAdminCompliance` 在关闭时不写 settings；中间件 `AdminComplianceGuard` 因 `IsAdminComplianceAcknowledged` 返回 true 而放行所有 admin 路由。同步调整 `backend/internal/service/admin_compliance_test.go` 与 `backend/internal/server/middleware/admin_compliance_test.go`，覆盖“缺少确认记录也不拦截、不持久化确认记录”的 fork 行为。 | `go test ./internal/service -run 'TestAdminCompliance' -count=1`；`go test ./internal/server/middleware -run 'TestAdminComplianceGuard' -count=1`；`go test ./internal/service ./internal/server/middleware -count=1`；`git diff --check`。 | 同步官方后搜索 `AdminComplianceEnabled`、`AdminComplianceGuard`、`GetAdminComplianceStatus`、`AcceptAdminCompliance` 和 `ADMIN_COMPLIANCE_ACK_REQUIRED`。如果上游改动合规确认实现，继续保持本 fork 默认关闭：后台路由不得返回 423，`/admin/compliance` 状态应为 `required=false`，确认接口在关闭时不新增 `settings.admin_compliance_acknowledgement:*`。 |
| 2026-06-11 | 1 vCPU VPS 同时运行 primary、ap1、ap2 三套独立 Postgres/Redis、Grafana/PDC 和 WARP 时，CPU 长时间接近 100%，重启后所有容器同时恢复会进一步放大负载；部署文档仍按旧的三套数据层描述。 | 线上将 `ap1` / `ap2` 数据迁入共享 `sub2api-postgres`：primary 使用 `sub2api`，ap1 使用 `sub2api_ap1`，ap2 使用 `sub2api_ap2`；Redis 合并到共享 `sub2api-redis`：DB0/DB1/DB2 分别给 primary/ap1/ap2；`/opt/sub2api-ap1-deploy` 与 `/opt/sub2api-ap2-deploy` 的 compose 改为接入外部网络 `sub2api-deploy_sub2api-network`，不再定义独立 `postgres` / `redis` 服务；移除 `grafana`、`pdc-agent-sub2api` 和旧 ap1/ap2 Postgres/Redis 容器；移除未使用的 `snapd`，安装 `sysstat` 并将采样改为 1 分钟。 | 线上验证：`curl -fsS http://127.0.0.1:8081/health`、`8082/health`、`8083/health` 均返回 ok；`docker exec headroom-a2 curl -fsS http://127.0.0.1:8787/readyz` 返回 `ready=true`；`docker ps -a` 中不再有 Grafana/PDC/旧 ap1-ap2 DB Redis 容器；`sub2api`、`sub2api_ap1`、`sub2api_ap2` 均有 86 张 public 表；`sar -u 1 5` 稳定后平均 idle 约 58%。 | 同步官方或后续调整部署时，先看 `docs/VPS_DEPLOY_NOTES.md` 的“线上共享数据层、Grafana 移除与 sysstat 监控”。继续搜索 `sub2api-ap1-postgres`、`sub2api-ap2-postgres`、`grafana`、`pdc-agent-sub2api`、`sub2api_ap1`、`sub2api_ap2`、`REDIS_DB=1`、`REDIS_DB=2`；当前生产不得再恢复三套 Postgres/Redis 或 Grafana/PDC。 |
| 2026-06-13 | a2 前台静态资源已迁到 Cloudflare Pages，但 VPS 上 `cf-origin-ssl` 仍把 `a2.upit.top` 和 `ap2.upit.top` 合并在同一个 Caddy server block，导致旧内嵌前台仍可从 VPS origin 命中；需要关闭旧前台入口，同时不能误停仍为 Pages 提供 API 回源的 `sub2api-ap2`。 | Cloudflare Pages 继续承载 `a2.upit.top` 前台静态资源；Pages Worker 将 `/api/*`、`/health`、`/51Token/*` 等回源到 `https://ap2.upit.top`。线上备份 `/opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656` 后，把 Caddy server label 从 `ap2.upit.top:443, a2.upit.top:443` 改为 `ap2.upit.top:443` 并 reload `cf-origin-ssl`。同步更新 `docs/CLOUDFLARE_PAGES_DEPLOY_CN.md` 与 `docs/VPS_DEPLOY_NOTES.md`，明确“关闭 VPS 前台入口”只移除已迁移的前台域名，不关闭 API 回源域名和后端容器。 | 线上验证：`curl -sSIL https://a2.upit.top/login` 仍返回 Cloudflare Pages 页面且加载 `assets/index-D26Z13h8.js`；`curl -fsS https://a2.upit.top/health`、`https://ap2.upit.top/health` 均返回 ok；`curl -fsS https://a2.upit.top/api/v1/settings/public` 返回 `code=0`；`docker inspect -f "{{.State.Health.Status}}" sub2api-ap2` 为 `healthy`；VPS 本地直连 `a2.upit.top` 不再返回旧前台；`git diff --check`。 | 后续调整 Pages、自定义域名、Caddy 或回滚时，继续搜索 `a2.upit.top:443`、`ap2.upit.top:443`、`SUB2API_ORIGIN`、`A2_API_ORIGIN`、`sub2api-frontend`。除非明确要回滚 Pages，否则不要把 `a2.upit.top` 加回 VPS origin；无论前台是否在 Pages，都不要关闭 `ap2.upit.top` 或 `sub2api-ap2`，它们仍承担 API 回源。 |
| 2026-06-13 | a2 前台迁到 Cloudflare Pages 后，公开配置类接口仍每次由 Pages Worker 回源 VPS；这些接口访问频繁且内容短周期内稳定，可以先迁到 Cloudflare 边缘短缓存，进一步降低 VPS 后端请求压力。 | `frontend/public/_worker.js` 新增公开只读接口短缓存：仅 `GET` 且精确匹配 `/api/v1/settings/public`、`/api/status`、`/api/home_page_content` 时使用 Cloudflare Cache API，TTL 为 60 秒；缓存回源请求剥离 `Authorization` 与 `Cookie`；请求显式 `Cache-Control: no-cache/no-store` 或 `Pragma: no-cache` 时绕过缓存。其它认证、后台、支付、OAuth、模型网关、长流式接口继续只代理回 VPS，不做边缘缓存。`frontend/src/cloudflare/__tests__/pages-worker.spec.ts` 增加 HIT/MISS、敏感请求头剥离、写接口/网关不缓存、HEAD 不缓存的回归测试；部署文档补充验证响应头 `X-Sub2API-Edge-Cache`。 | `pnpm --dir frontend exec vitest run src/cloudflare/__tests__/pages-worker.spec.ts`；`pnpm --dir frontend run build`；`git diff --check`；部署 Pages 后验证 `curl -sSi https://a2.upit.top/api/v1/settings/public | grep -Ei 'HTTP/|x-sub2api-edge-cache|cache-control|cf-cache-status'`，并确认 `a2.upit.top/health` 与 `ap2.upit.top/health` 返回 ok。 | 同步官方或后续修改 Worker 时，继续搜索 `EDGE_CACHEABLE_PUBLIC_PATHS`、`X-Sub2API-Edge-Cache`、`createOriginRequest`、`shouldProxyToOrigin`。只允许缓存公开只读 GET 接口；不得缓存 `/api/v1/auth/*`、`/api/v1/admin/*`、支付/OAuth 回调、`/51Token/*`、`/v1/*`、`/responses/*`、`/images/*`。如果公开设置开始返回用户态字段，必须先移除边缘缓存。 |
| 2026-06-13 | a2 前台迁到 Cloudflare Pages 后，VPS 内嵌前台原本由 Go 注入的 `window.__APP_CONFIG__` 不再执行，导致 Pages 静态 HTML 首屏使用构建默认配置；站点标题、logo、Turnstile 开关和 `api_base_url` 等 a2 线上 public settings 不能在首屏生效。 | 新增 `scripts/cloudflare-pages-config.mjs` 与 `scripts/inject-pages-public-settings.mjs`：Pages 发布前从对应环境的公开设置接口拉取公开响应，只取 wrapper 里的 `data` 对象，安全转义后写入对应环境产物 `index.html` 的 `window.__APP_CONFIG__`，并同步替换 `<title>`；脚本重复运行会替换旧注入，不会累积多份配置。不同服务未来可部署不同域名和多套前台产物；运行时 public settings 可发布前注入，凡是进入 Vite/JS/CSS bundle 的构建期配置必须按环境分别打包，不靠 Worker 动态 HTML 注入兜底。新增 `frontend/src/cloudflare/__tests__/pages-config-injection.spec.ts` 覆盖 wrapper 解包、内联 JSON 转义、标题替换和幂等注入；`docs/CLOUDFLARE_PAGES_DEPLOY_CN.md`、`docs/VPS_DEPLOY_NOTES.md` 补充 build -> inject -> deploy 流程。 | `pnpm --dir frontend exec vitest run src/cloudflare/__tests__/pages-worker.spec.ts src/cloudflare/__tests__/pages-config-injection.spec.ts`；`pnpm --dir frontend run build`；`node scripts/inject-pages-public-settings.mjs --settings-url https://ap2.upit.top/api/v1/settings/public --html backend/internal/web/dist/index.html`；`rg -n "window\\.__APP_CONFIG__|api_base_url|<title>" backend/internal/web/dist/index.html`；部署 Pages 后验证 `curl -fsSL https://a2.upit.top/login | rg "window\\.__APP_CONFIG__|https://ap2.upit.top/51Token/v1|<title>"`。 | 后续调整 Pages 部署、同步官方前台构建或新增环境时，继续搜索 `inject-pages-public-settings.mjs`、`cloudflare-pages-config.mjs`、`window.__APP_CONFIG__`、`settings/public`、`api_base_url`。只能注入公开 settings `data` 字段；不得把 `.env`、后台 admin 配置、数据库连接、Token secret、Turnstile secret 等私有配置写入静态文件。若差异包含构建期配置，必须每套环境单独 build。 |
| 2026-06-14 | 主环境、a1、a2 三套前台都迁 Cloudflare Pages 后，Worker 必须按正式前台 host 回各自 API origin；否则 `a1.upit.top` 会继续使用 `https://api.upit.top` 的公开配置和模型端点，导致 a1 前台显示主环境 base URL。 | `frontend/public/_worker.js` 新增正式域名到 API origin 的显式映射：`ai.upit.top -> https://api.upit.top`、`a1.upit.top -> https://ap1.upit.top`、`a2.upit.top -> https://ap2.upit.top`；只有预览域名才读取 `SUB2API_ORIGIN`。三套正式前台使用独立 Pages 项目和独立注入产物：`sub2api-frontend-main`、`sub2api-frontend-a1`、`sub2api-frontend-a2`。`docs/CLOUDFLARE_PAGES_DEPLOY_CN.md` 与 `docs/VPS_DEPLOY_NOTES.md` 更新 build -> copy -> inject -> deploy 三套产物流程，并记录 VPS 只移除前台域名旧入口，保留 `api/ap1/ap2` 回源和后端容器。 | `pnpm --dir frontend exec vitest run src/cloudflare/__tests__/pages-worker.spec.ts src/cloudflare/__tests__/pages-config-injection.spec.ts`；`pnpm --dir frontend run build`；分别注入 `/tmp/sub2api-pages-main`、`/tmp/sub2api-pages-a1`、`/tmp/sub2api-pages-a2`；部署后验证 `curl -fsSL https://ai.upit.top/login` 包含 `https://api.upit.top/51Token/v1`，`https://a1.upit.top/login` 包含 `https://ap1.upit.top/51Token/v1`，`https://a2.upit.top/login` 包含 `https://ap2.upit.top/51Token/v1`，三套 `/health` 及 `api/ap1/ap2` `/health` 均返回 ok。 | 同步官方或后续调整 Cloudflare Pages 时，继续搜索 `PRODUCTION_API_ORIGINS_BY_HOST`、`A1_API_ORIGIN`、`A2_API_ORIGIN`、`sub2api-frontend-main`、`sub2api-frontend-a1`、`sub2api-frontend-a2`。不要把三套正式前台合并到同一个注入后的 `index.html`；如果只是运行时 public settings 差异，可复用 assets 但必须分别 inject；如果有 Vite 构建期差异，必须分别 build。 |
| 2026-06-13 | 共享数据层后，三套 sub2api 仍沿用默认大连接池，1 vCPU VPS 上会长期保留过多 PostgreSQL / Redis 连接；同时三套都是线上环境，不能简单停掉 primary 或 ap1，a2 只是内测环境可更激进降配。 | 线上按环境分档调整 `.env` 并滚动重启：primary `/opt/sub2api-deploy` 与 ap1 `/opt/sub2api-ap1-deploy` 使用 `DATABASE_MAX_OPEN_CONNS=12`、`DATABASE_MAX_IDLE_CONNS=2`、`REDIS_POOL_SIZE=128`、`REDIS_MIN_IDLE_CONNS=2`；内测 ap2 `/opt/sub2api-ap2-deploy` 使用 `4`、`1`、`32`、`1`。调整前分别备份为 `.env.bak-pool-tuning-20260613082813`。`docs/VPS_DEPLOY_NOTES.md` 新增“连接池按环境分档”，记录分档、滚动重启顺序和验证命令。 | 线上验证：`docker exec sub2api/sub2api-ap1/sub2api-ap2 env` 显示新连接池参数；`pg_stat_activity` idle 连接降为 `sub2api=2`、`sub2api_ap1=2`、`sub2api_ap2=1`；`curl -fsS http://127.0.0.1:8081/health`、`8082/health`、`8083/health` 均返回 ok；`docker inspect -f "{{.State.Health.Status}}" sub2api sub2api-ap1 sub2api-ap2` 均为 `healthy`；公网 `api.upit.top`、`ai.upit.top`、`a1.upit.top`、`ap2.upit.top`、`a2.upit.top` 的 `/health` 均返回 ok；`git diff --check`。 | 后续部署、重建 compose 或同步官方部署模板时，继续搜索 `DATABASE_MAX_OPEN_CONNS`、`DATABASE_MAX_IDLE_CONNS`、`REDIS_POOL_SIZE`、`REDIS_MIN_IDLE_CONNS`。除非升级 VPS 或重新评估并发压力，不要把三套环境恢复为 `50/10/512/10`；a2 作为内测环境继续保留最低档，primary/ap1 保留线上建议档。 |
| 2026-06-14 | 新 VPS 上继续让 primary 和 a1 使用 Headroom，但默认 `compression_executor.max_workers=8` 会在大请求压缩时带来瞬时 CPU 尖峰；直接设置 `HEADROOM_COMPRESSION_MAX_WORKERS=2` 后，当前 Headroom 镜像的 `headroom proxy` CLI 没有把该 env 灌进 `ProxyConfig`，`/health` 仍显示 `source=auto`。 | 线上新增 `/opt/headroom_start_with_compression_workers.py` 启动 wrapper，在执行原 `headroom proxy` 前 monkey-patch `ProxyConfig`，把 `HEADROOM_COMPRESSION_MAX_WORKERS=2` 显式传入 `compression_max_workers`；`/opt/sub2api-deploy/docker-compose.yml` 的 `headroom-main` 和 `/opt/sub2api-ap1-deploy/docker-compose.yml` 的 `headroom-a1` 挂载该 wrapper 并设置 `entrypoint: [python, /opt/headroom_start_with_compression_workers.py]`。`HEADROOM_WORKERS=1` 继续只表示 Uvicorn 单进程，不改成 2；a2 不启用 Headroom。`docs/VPS_DEPLOY_NOTES.md` 新增“Headroom 压缩 worker 限制”操作和验证说明。 | 线上验证：`docker exec headroom-main/headroom-a1 python -c '.../health...'` 显示 `compression_executor.max_workers=2` 且 `source=explicit`；两个容器 env 均为 `HEADROOM_WORKERS=1`、`HEADROOM_COMPRESSION_MAX_WORKERS=2`；`docker inspect sub2api/sub2api-ap1/sub2api-ap2` 确认 only primary -> `headroom-main`、ap1 -> `headroom-a1`、ap2 无 override；`docker ps` 显示 `headroom-main`、`headroom-a1` 和三套 sub2api 均 healthy；短窗口 `docker stats` 中两个 Headroom 约 `0.3% - 0.4%` CPU。 | 后续升级 Headroom 镜像、重建 compose 或同步运维模板时，继续搜索 `headroom_start_with_compression_workers.py`、`HEADROOM_COMPRESSION_MAX_WORKERS`、`compression_executor`、`compression_max_workers`、`HEADROOM_WORKERS`、`headroom-main`、`headroom-a1`。如果新镜像原生支持 `--compression-max-workers` 或正确读取 env，可以移除 wrapper，但必须先用 `/health` 验证 `source=explicit`；不要把 `HEADROOM_WORKERS` 当成压缩线程池限制；a2 仍保持不启用 Headroom。 |
| 2026-06-18 | OpenAI OAuth 账号周限额跑满后，即使上游已重置，在账号管理里点击 quota “次数/查询”或刷新账号列表自动查询用量，仍可能继续显示主套餐 7d 用量 100% 或本地限流倒计时；根因是 `/admin/openai/accounts/:id/quota` 只查询上游 `/wham/usage`，没有同步最新快照到账号 `extra`，也没有在官方 `rate_limit.limit_reached=false` 时清理本地 `rate_limit_reset_at`，而 `/admin/accounts/:id/usage` 原本只走 Codex 探测/本地快照，不会补官方 main quota，同时前端账号用量栏优先读旧 `codex_main_*_used_percent` 原始字段。 | 后端 `OpenAIQuotaService.QueryUsage()` 查询成功后同步 main quota 与 Spark additional quota 到账号 `extra`；当官方主套餐 `rate_limit.limit_reached=false` 时调用 `ClearRateLimit` 清掉本地运行时限流状态，官方仍限额时不清；`AccountUsageService.getOpenAIUsage()` 返回 `codex_main_*` / `codex_*` 字段时统一复用窗口过期归零逻辑，避免过期 `reset_at` 仍暴露旧 100%；当 OpenAI OAuth 账号处于本地限流或主套餐 5h/7d 快照已达 100% 时，账号列表 usage 刷新会限频 1 分钟触发一次官方 quota 同步，随后重读账号 `extra` 重建返回用量，普通账号和非可疑账号不打官方接口；前端 `OpenAIQuotaResetCell` 查询成功后发出 `queried` 事件，`AccountUsageCell` 收到后立即拉取 active usage，并在本地对过期 reset_at 快照做 0% 防守展示。 | `cd backend && go test ./internal/service -run 'TestAccountUsageService_GetOpenAIUsageZerosExpiredMainSnapshotFields|TestAccountUsageService_GetOpenAIUsage(RefreshesOfficialQuotaForSuspiciousMainSnapshot|ThrottlesOfficialQuotaRefresh)|TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow|TestOpenAIQuotaServiceSyncQuotaUsageSnapshotWritesMainAndSpark|TestOpenAIQuotaService(ClearLocalRateLimitIfQuotaRecovered|DoesNotClearLocalRateLimitWhenQuotaStillReached)' -count=1`；`pnpm --dir frontend exec vitest run src/components/account/__tests__/AccountUsageCell.spec.ts`。 | 同步官方后搜索 `syncQuotaUsageSnapshot`、`clearLocalRateLimitIfQuotaRecovered`、`refreshOpenAIMainQuotaSnapshotIfNeeded`、`shouldRefreshOpenAIMainQuotaFromOfficial`、`openAIOfficialQuotaTTL`、`codex_main_7d_used_percent`、`buildCodexUsageProgressFromExtraKeys`、`OpenAIQuotaResetCell`、`handleOpenAIQuotaQueried`、`queryOpenAIQuota`。必须保持“quota 查询会同步最新账号快照”、“官方已恢复时清本地 rate limit”、“账号列表用量刷新对可疑 OpenAI 主套餐状态可触发受限官方同步”和“过期 reset_at 不再显示旧 used_percent”的行为；同时继续保持主套餐 `codex_main_*` 与 Spark `codex_*` 分开展示，不要让两套 quota 互相覆盖。 |

## 2026-06-06 未 push 改动梳理

### 本地未推送提交

1. `1586b1ef feat: improve search suggestions and local frontend build`

   - 管理端与用户端搜索建议输入框统一抽成 `frontend/src/components/common/SearchSuggestInput.vue`，覆盖账号管理、使用记录、订阅管理和用户 API Key 场景，支持输入搜索与下拉建议并存，选中后回填邮箱。
   - `backend/internal/handler/admin/usage_handler.go` / `usage_handler_search_users_test.go` 扩展用户搜索结果展示字段，前端 `frontend/src/api/admin/usage.ts` 和各视图同步消费。
   - 本地前端构建侧继续收口：`frontend/src/i18n/index.ts` 调整动态导入，`frontend/src/views/admin/AccountsView.vue` 若干弹窗改按需加载，`frontend/vite.config.ts` 增加管理端与用户端手动分包，降低首屏包体积。
   - 这轮又继续补了搜索建议交互收口：`AccountsView` 只允许 `change` 触发表格 reload，避免账号管理聚焦搜索框就重复请求 `/api/v1/admin/accounts`；`UsageFilters`、`SubscriptionsView`、`KeysView` 统一补了 `blur` 收起下拉，避免失焦后悬挂。
   - 对应回归测试已覆盖 `SearchSuggestInput`、`AccountTableFilters`、`UsageFilters`、`AccountsView.searchSuggest`；后续若 rebase/upstream 覆盖，需要优先复查 `SearchSuggestInput`、`UsageFilters`、`AccountTableFilters`、`AccountsView`、`SubscriptionsView`、`KeysView` 和 `manualChunks`。

2. `e33e265e feat: update ssh alias docs and prefill key group`

   - 将部署/运维说明统一切换到 SSH 别名 `51token-vps`，涉及 `deploy/local-gzip-binary-deploy.sh`、[docs/VPS_DEPLOY_NOTES.md](/Users/okk/git-projects/sub2api/docs/VPS_DEPLOY_NOTES.md) 和 fork 维护脚本。
   - `frontend/src/views/user/KeysView.vue` 补充默认分组预填逻辑，配合这轮线上登录和部署整理一起落地。
   - 这部分属于非 fork 运行环境约定，后续如果服务器别名或登录方式再次调整，需要同步改脚本、部署文档和维护文档，避免再次出现明文 IP 漏出。

3. `6daca973 fix: prefer spark routing for explicit gpt-5 requests`

   - `backend/internal/service/openai_model_router.go` 新增显式 GPT-5 minor 路由优先逻辑：用户显式选 `gpt-5.3-codex-spark` 时固定走 spark；显式选 `gpt-5.x` 时，优先按 spark 剩余额度判断是否降到 spark。
   - 这是后续继续简化前的中间提交；当前工作区已经在这个基础上继续往“spark 剩余额度大于 5% 就优先 spark，否则走用户选择模型”推进，所以如果将来要回顾本地历史，需要把这个提交和下面“当前未提交工作区”一起看。

### 当前未提交工作区

1. OpenAI 显式模型路由继续简化

   - `backend/internal/service/openai_model_router.go` 已从上面那版继续调整为更直接的规则：
     - 用户显式选 `gpt-5.3-codex-spark`，始终走 spark。
     - 用户显式选非 spark 的 `gpt-5.<minor>`（例如 `gpt-5.4`、`gpt-5.5`、未来 `gpt-5.6/5.7`），只要 spark 剩余额度大于 5%，就自动改走 spark；否则才走用户显式选择模型。
   - 现有实现使用 `isExplicitGPT5MinorModel()` 泛化匹配未来版本，不再只硬编码 `5.4/5.5`。

2. OpenAI 账号额度后台自愈刷新

   - 新增 `backend/internal/service/openai_quota_refresh_service.go` 与 `backend/internal/service/openai_quota_refresh_service_test.go`。
   - 后台会定时巡检 OpenAI OAuth + Responses WebSocket v2 账号：如果 5 小时窗口已重置、7 天窗口已重置，或 `codex_usage_updated_at` 超过 24 小时未刷新，就主动触发一次现有 `GetUsage(..., true)` probe，回写 `codex_*` 快照。
   - 这样账号如果此前因为 quota auto-pause 被调度层软跳过，在额度窗口刷新后即使没有新流量，也能自动恢复可调度状态。
   - `backend/internal/service/wire.go`、`backend/cmd/server/wire.go`、`backend/cmd/server/wire_gen.go`、`backend/cmd/server/wire_gen_test.go` 已同步接入和清理停机逻辑。

3. OpenAI 网关请求上下文与模型路由透传增强

   - `backend/internal/service/openai_gateway_service.go` 目前还有一批未提交改动，主要是为模型路由和账号可用性判定补 `openAIAccountRequestOptions`、`WithModelRouteRequestContext(...)`、`buildOpenAIAccountRequestOptions(...)` 以及按真实上游模型做 eligibility / 限流判断的能力。
   - 这个文件不是本轮新起草的空白文件，而是前面已经存在持续修改的工作区改动；提交时需要特别确认是否与本轮“配额自愈刷新”和“显式 spark 路由”一起提交，避免把更大范围的网关实验性改动误打包。

### 2026-06-06 本地验证

- `go env -w GOTOOLCHAIN=auto`
- `cd backend && go test ./internal/service -run 'Test(ShouldRefreshOpenAIQuotaSnapshotInBackground|OpenAIQuotaRefreshServiceRunOnce|ShouldRefreshOpenAICodexSnapshot|OpenAIGatewayService_SelectAccountForModelWithExclusions_)' -count=1`
- `cd backend && go test ./cmd/server -run 'TestProvide(ServiceBuildInfo|Cleanup_WithMinimalDependencies_NoPanic)' -count=1`
- `git diff --check`

### 2026-06-12 前台 auth refresh 失败后反复请求修复

线上现象：浏览器本地保留已失效 `refresh_token` 时，前台会持续请求 `/api/v1/auth/refresh`，直到后端接口被限流。根因在前端 auth store：`checkAuth()` 恢复本地会话后，如果 token 已过期会立即触发主动 refresh；但 `authAPI.refreshToken()` 失败时原逻辑只打印日志，不清理本地登录态和坏的 `refresh_token`，页面重载或路由重新初始化后会继续使用同一个坏 token 请求 refresh。

本地补丁：

- `frontend/src/stores/auth.ts`：为主动 token refresh 增加单飞保护；refresh 失败时写入 `sessionStorage.auth_expired=1` 并调用 `clearAuth(...)` 清理 `auth_token`、`refresh_token`、`auth_user` 和过期时间，避免下一次启动继续使用坏 token。
- `frontend/src/stores/__tests__/auth.spec.ts`：覆盖“本地 token 已过期且 refresh 返回 401 时，应清理本地登录态且只请求一次 refresh”。
- `frontend/src/api/__tests__/client.spec.ts`：覆盖 axios client 对 `/auth/refresh` 自身 401 不递归刷新，确认拦截器的 `isAuthEndpoint` 边界仍有效。

验证：

```bash
pnpm --dir frontend exec vitest run src/stores/__tests__/auth.spec.ts src/api/__tests__/client.spec.ts
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
git diff --check
```

线上 a2 验证：

- 镜像：`sub2api:subapi-f289f84a-ap2-auth-refresh-fix-20260612104831`
- `sub2api-ap2` 为 `healthy`
- `http://127.0.0.1:8083/health` 与 `https://ap2.upit.top/health` 均返回 ok
- `sub2api` / `sub2api-ap1` 未被切换，仍保持原镜像

同步官方后的复查：

- 搜索 `performTokenRefresh`、`scheduleTokenRefreshAt`、`tokenRefreshPromise`、`/auth/refresh`、`isAuthEndpoint`。
- 复跑上述两个前端测试文件，重点确认坏 refresh token 只触发一次 refresh，并且失败后本地 `auth_token` / `refresh_token` 被清理。
- 如果官方已经在 auth store 中实现等价的失败清理和单飞保护，并且测试覆盖同样边界，可以删除本地补丁；否则保留本 fork 行为，避免刷新风暴再次打到后端限流。

### 2026-06-12 前台短有效期 token 后台 refresh 循环收口

线上现象：登录成功后 `/api/v1/auth/refresh` 仍会持续返回 200 并反复请求；这不是失败重试，而是前台拿到短有效期 token（例如 60 秒）后，主动刷新定时器和每 60 秒用户资料刷新互相触发，导致空闲页面也持续刷新 token。旧二进制还会让 `/api/v1/admin/compliance` 返回 404，原因是本地/线上运行的镜像未包含当前合规路由注册代码；当前源码中的合规 API 应保留，且 fork 默认关闭时返回 `required=false`。

本地补丁：

- `frontend/src/stores/auth.ts`：`expires_in <= TOKEN_REFRESH_BUFFER` 时只保存 `token_expires_at`，不再安排主动 refresh 定时器；自动用户资料刷新在 access token 已过期时直接跳过，不触发 `/auth/me -> 401 -> /auth/refresh`；`checkAuth()` 恢复已过期 token 时不后台请求用户信息或立即 refresh，等待用户真实 API 请求由 axios 401 拦截器刷新；登录、OAuth 回调和外部 `auth-token-refreshed` 事件统一注册 token 刷新事件监听，长有效期 token 仍保留过期前 120 秒主动刷新。
- `frontend/src/api/client.ts`：axios 401 刷新成功后派发 `auth-token-refreshed` 事件，让 Pinia auth store 同步新 access/refresh token 和过期时间，避免 store 继续用旧过期时间重新排后台刷新。
- `frontend/src/stores/__tests__/auth.spec.ts` 与 `frontend/src/api/__tests__/client.spec.ts`：覆盖短有效期登录、外部刷新返回短有效期 token、恢复已过期 token 不后台 refresh、`/auth/refresh` 自身 401 不递归刷新等边界。
- `frontend/src/components/layout/AuthLayout.vue`：登录页 logo 继续使用站点配置的 `site_logo`，但外层补白色圆角背景与浅描边，避免透明底黑色标识在登录背景上不可辨认。

验证：

```bash
pnpm --dir frontend exec vitest run src/stores/__tests__/auth.spec.ts src/api/__tests__/client.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts src/components/__tests__/LoginForm.spec.ts
pnpm --dir frontend run build
git diff --check
```

本地运行验证：

```bash
/opt/homebrew/opt/postgresql@18/bin/pg_ctl -D .local/pg18 -l .local/pg18/server.log -o "-p 5433" start
DATA_DIR=.local/ap2-runtime .local/ap2-runtime/sub2api-current
VITE_DEV_PORT=5173 VITE_DEV_PROXY_TARGET=http://127.0.0.1:18083 pnpm --dir frontend run dev -- --host 127.0.0.1
curl -i http://localhost:5173/setup/status
curl -i http://localhost:5173/api/v1/admin/compliance
```

预期：`/setup/status` 返回 200；未带 token 的 `/api/v1/admin/compliance` 返回 401 而不是 404，说明当前后端路由已注册。管理员有效 token 请求时应返回 `required=false`，不得弹出合规确认。

同步官方后的复查：

- 搜索 `scheduleTokenRefresh`、`startAutoRefresh`、`isAccessTokenExpired`、`auth-token-refreshed`、`AdminComplianceEnabled`、`registerAdminComplianceRoutes`、`AuthLayout.vue`。
- 确认短有效期 token 不再进入后台主动 refresh 循环，页面空闲时不会每 30/60 秒刷 `/api/v1/auth/refresh`；真实用户操作遇到 401 时仍能通过 axios 拦截器刷新并重放原请求。
- 合规 API 需要继续保留，fork 默认关闭时返回 `required=false`，不要通过删路由造成前端 404。
- 登录页 logo 白底属于当前 fork 视觉修复；若官方后续调整登录页布局，继续确认透明 logo 在浅/深背景下可辨认。

### 2026-06-13 前台长有效期 token 触发浏览器定时器上限导致 refresh 风暴

线上现象：a2 登录成功后仍会在极短时间内持续请求 `/api/v1/auth/refresh`，后端先返回 200，随后被 refresh-token 限流打到 429。本地复现时登录响应 `expires_in=2592000`（30 天），后端日志显示登录后没有先出现业务接口 401，却在同一秒内连续出现几十次 `/api/v1/auth/refresh`。

根因：前端 `scheduleTokenRefreshAt()` 直接把 `expires_at - now - 120s` 交给 `setTimeout`。30 天 access token 的延迟超过浏览器 `setTimeout` 最大安全延迟（约 24.8 天），浏览器会把超长延迟夹成近似立即执行，导致“refresh 成功 -> 重新安排 30 天定时器 -> 立即触发 -> 再 refresh”的循环。

本地补丁：

- `frontend/src/stores/auth.ts`：新增 `MAX_TOKEN_REFRESH_DELAY=24h`，超长主动刷新延迟改为分段等待；每段醒来后重新计算是否已进入 120 秒刷新缓冲区，未到期则继续分段等待，到期才调用 `performTokenRefresh()`。
- `frontend/src/stores/__tests__/auth.spec.ts`：覆盖 30 天长有效期 token 登录后 60 秒内不应触发主动 refresh，防止浏览器定时器上限导致回归。

验证：

```bash
pnpm --dir frontend exec vitest run src/stores/__tests__/auth.spec.ts -t '长有效期 token 登录后不会因 setTimeout 上限立即 refresh'
pnpm --dir frontend exec vitest run src/stores/__tests__/auth.spec.ts src/api/__tests__/client.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts src/components/__tests__/LoginForm.spec.ts
pnpm --dir frontend run build
git diff --check
```

本地运行验证：

```bash
/opt/homebrew/opt/postgresql@18/bin/pg_ctl -D .local/pg18 -l .local/pg18/server.log -o "-p 5433" start
DATA_DIR=.local/ap2-runtime .local/ap2-runtime/sub2api-current
VITE_DEV_PORT=5173 VITE_DEV_PROXY_TARGET=http://127.0.0.1:18083 pnpm --dir frontend run dev -- --host 127.0.0.1
```

浏览器登录本地 `http://localhost:5173/login` 后进入 `/dashboard`；后端日志中 `01:19` 这次登录后的业务接口均为 200，且该时间窗没有 `/api/v1/auth/refresh`。

同步官方后的复查：

- 搜索 `MAX_TOKEN_REFRESH_DELAY`、`scheduleTokenRefreshAt`、`TOKEN_REFRESH_BUFFER`、`performTokenRefresh`。
- 如果官方改用短 token 或 cookie 会话，也要继续确认“token 有效期超过 24.8 天时不会把超长毫秒值直接传给浏览器 `setTimeout`”。
- 复跑 auth store 长有效期 token 测试；登录后网络面板不应出现成批 `/api/v1/auth/refresh`。

### 2026-06-13 a2 Headroom 内网上游请求被账号 SOCKS 代理导致 502

线上现象：a2 / ap2 使用 `gpt-5.3-codex-spark` 请求 `/51Token/v1/responses` 时返回 502，后端日志显示已选中 OpenAI OAuth 账号 17 后 `upstream_status=502`，随后因为账号池无可用账号返回 `no available accounts`。同一账号直接测试 Spark 能通，直接从容器访问 `http://headroom-a2:8787/v1/responses` 也能通。

根因：ap2 通过 `GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_URL=http://headroom-a2:8787/v1/responses` 把 OAuth Codex Responses 流量先发到同 Docker network 内的 `headroom-a2`，但普通 Forward 和 passthrough Forward 在发送请求时仍统一套用账号绑定的 `socks5h://172.17.0.1:40001`。`socks5h` 会让代理端解析目标 hostname，而 `headroom-a2` 是 Docker 内网服务名，代理端不可解析或不可达，导致 sub2api 到 Headroom 的第一跳失败。

本地补丁：

- `backend/internal/service/openai_gateway_service.go`：新增 `openAICodexHTTPProxyURL()`，OAuth Codex override 目标为 `http/ws` 且 host 是 localhost、loopback、私网 IP 或无点号的 Docker/service hostname 时，不再使用账号 proxy；外部 `https://chatgpt.com` 等目标仍保留账号 proxy。
- 普通 Forward 和 OpenAI passthrough Forward 都改用同一个 proxy 选择 helper，避免两条路径行为分叉。
- `backend/internal/service/openai_oauth_passthrough_test.go`：新增回归测试，覆盖 `http://headroom-a2:8787/v1/responses` + 账号 SOCKS 代理时传给 HTTP upstream 的 `proxyURL` 必须为空。

验证：

```bash
go test ./internal/service -run 'TestOpenAIGatewayService_OAuthCodexHeadroomOverrideBypassesAccountProxy' -count=1
go test ./internal/service -run 'TestOpenAIGatewayService_(OAuthCodexResponsesURLCanBeOverridden|OAuthCodexHeadroomOverrideBypassesAccountProxy|ResponsesUnknownModelDoesNotFallbackToGPT54)|TestOpenAIBuild(OpenAIResponsesWSURLUsesOAuthCodexOverride|UpstreamRequestCompactForcesJSONAcceptForOAuth|UpstreamRequestOpenAIPassthroughPreservesCompactPath)' -count=1
go test ./internal/service -count=1
git diff --check
```

同步官方后的复查：

- 搜索 `openAICodexHTTPProxyURL`、`isOpenAIInternalCodexOverrideRequest`、`OpenAIOAuthCodexResponsesURL`、`headroom-a2`。
- 如果官方后续也支持 Codex OAuth sidecar / local proxy，要确认内网 sidecar 第一跳不走账号 SOCKS；但直接 `chatgpt.com` 或外部 override URL 仍应保留账号 proxy 能力。
- a2 灰度验证时，用绑定 WARP SOCKS 的 OpenAI OAuth 账号请求 `gpt-5.3-codex-spark`，确认 `/51Token/v1/responses` 不再因 `headroom-a2` 第一跳返回 502。

### 2026-06-15 a1 Chat Completions 转 Headroom 时被账号 SOCKS 代理导致 502

线上现象：a1 / ap1 使用 OpenAI Chat Completions 兼容入口请求 `GPT-5.5` 时，Cloudflare 页面显示 `ap1.upit.top 502 Bad Gateway`。后端日志显示请求已到达 `sub2api-ap1`，路径为 `/v1/chat/completions`，选中 OpenAI OAuth 账号 47 后失败：

```text
upstream request failed: Post "http://headroom-a1:8787/v1/responses": socks connect tcp 172.17.0.1:40001->headroom-a1:8787: unexpected EOF
```

根因：2026-06-13 的 a2 修复已覆盖 OpenAI Responses 普通 Forward / passthrough Forward，但 OpenAI Chat Completions 兼容入口会先把 `/v1/chat/completions` 转成 Responses，再调用 `buildUpstreamRequest()` 指向 `http://headroom-a1:8787/v1/responses`。该发送路径仍直接读取 `account.Proxy.URL()`，没有复用 `openAICodexHTTPProxyURL()`，导致 Docker 内网服务名 `headroom-a1` 被交给账号 SOCKS 代理解析，第一跳失败。

本地补丁：

- `backend/internal/service/openai_gateway_chat_completions.go`：发送 OpenAI Chat Completions 转换后的上游请求时，改用 `s.openAICodexHTTPProxyURL(account, upstreamReq)` 选择代理。内网 Headroom override 不走账号 proxy，外部 `chatgpt.com` / 外部 base URL 仍保留账号 proxy。
- `backend/internal/service/openai_gateway_chat_completions_test.go`：新增 `TestForwardAsChatCompletions_OAuthHeadroomOverrideBypassesAccountProxy`，覆盖 `http://headroom-a1:8787/v1/responses` + OAuth 账号 SOCKS 代理时，传给 HTTP upstream 的 `proxyURL` 必须为空。

验证：

```bash
go test ./internal/service -run 'TestForwardAsChatCompletions_OAuthHeadroomOverrideBypassesAccountProxy|TestOpenAIGatewayService_OAuthCodexHeadroomOverrideBypassesAccountProxy' -count=1
go test ./internal/service -run 'TestForwardAsChatCompletions_|TestOpenAIGatewayService_OAuthCodexHeadroom' -count=1
git diff --check
```

线上 a1 发布记录：

- 修复提交：`c745d7fd fix: bypass account proxy for headroom chat completions`。
- 新镜像：`sub2api:subapi-c745d7fd-a1-headroom-chat-proxy-202606151012`。
- 只滚动 `/opt/sub2api-ap1-deploy` 的 `sub2api-ap1`，未动 primary / a2。
- compose 备份：`/opt/sub2api-ap1-deploy/docker-compose.yml.bak-a1-headroom-chat-proxy-current-20260615101557`。
- 验证：`https://a1.upit.top/health`、`https://ap1.upit.top/health` 均为 200；`https://ap1.upit.top/51Token/v1/models` 未带 key 返回 `401 API_KEY_REQUIRED`；新容器日志无 `socks connect tcp ... ->headroom-a1` 和 migration checksum mismatch。

同步官方后的复查：

- 继续搜索 `ForwardAsChatCompletions`、`openAICodexHTTPProxyURL`、`OpenAIOAuthCodexResponsesURL`、`headroom-a1`、`headroom-a2`。
- 任何新增的 OpenAI OAuth Codex / Responses / Chat Completions 转发路径，只要目标是本机或 Docker 内网 Headroom sidecar，都必须复用同一代理选择规则。
- 回归验证不能只跑 `/v1/responses`；必须同时覆盖 `/v1/chat/completions` 兼容入口。

### 2026-06-14 Headroom 转发增加后台运行开关

背景：a2 需要保留 `GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_URL=http://headroom-a2:8787/v1/responses` 作为 sidecar 能力配置，但不能因为环境变量存在就自动把所有 OpenAI OAuth Codex 请求切到 Headroom。此前排查大请求 502、压缩超时和动态调度时，运维需要一个可以从后台即时关闭 Headroom 路由的开关。

本地补丁：

- 新增全局设置 `openai_headroom_enabled`，默认 `false`。
- `GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_URL` 继续只表示“本环境具备 Headroom sidecar URL”；只有后台全局设置页的 “Headroom 压缩代理” 开关为 `true` 时，OpenAI OAuth Codex Responses 才会使用该 override URL。
- 开关为 `false` 时，即使 env 已配置 Headroom URL，普通 Forward、passthrough Forward 和 WS v2 URL 都回到 `https://chatgpt.com/backend-api/codex/responses` / `wss://chatgpt.com/backend-api/codex/responses`。
- 大请求旁路和 `compression_refused` 后短 TTL 直连记忆只在 Headroom 开关已启用时生效；关闭 Headroom 时不会产生无意义的旁路日志。
- 管理端“系统设置 -> 调度”中，在 “OpenAI 实验调度策略” 下方新增 “Headroom 压缩代理” 开关，并随保存请求写入 `openai_headroom_enabled`。

验证：

```bash
go test ./internal/service -run 'TestSettingService_UpdateSettings_PaymentVisibleMethodsAndAdvancedScheduler|TestOpenAIBuildOpenAIResponsesWSURLUsesOAuthCodexOverride|TestOpenAIGatewayService_OAuthCodex' -count=1
pnpm --dir frontend test:run src/views/admin/__tests__/SettingsView.spec.ts
```

同步官方后的复查：

- 搜索 `SettingKeyOpenAIHeadroomEnabled`、`openai_headroom_enabled`、`isOpenAIHeadroomEnabled`、`openAIOAuthCodexResponsesURL`、`openaiHeadroom`。
- 继续保持“环境变量提供能力、后台开关决定运行启用”的边界；同步或部署后不要仅凭 `.env` 中存在 `GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_URL` 就认为 Headroom 已启用。
- a2 灰度时先在后台确认 `openai_headroom_enabled=false` 是否可直连上游，再显式打开开关验证 Headroom 路径；如出现 502/压缩超时，可直接关闭该开关回退，不需要移除 compose/env。

## 2026-06-13 未 push 提交梳理

当前 `subapi` 分支相对 `origin/subapi` 仍有本地提交未 push。这些提交都属于当前 fork 的运行策略、a2 Pages/Headroom 灰度和 OpenAI Codex/Spark 用量修正，后续同步官方、rebase 或部署前需要按下面清单复查，避免被上游覆盖或误删。

### 本地未推送提交

1. `9d440fc4 fix: avoid oauth refresh loop for short tokens`

   - 修复前台短有效期 access token 导致的后台 refresh 循环：短 token 不再安排主动刷新定时器，恢复已过期 token 时不立即后台 refresh，用户真实请求遇到 401 时仍由 axios 拦截器刷新并重放。
   - `frontend/src/api/client.ts` 增加 `auth-token-refreshed` 事件，确保 axios 刷新成功后 Pinia auth store 同步新 token 和过期时间。
   - `frontend/src/components/layout/AuthLayout.vue` 保留站点配置 logo，但补白色圆角背景，避免透明 logo 在登录页背景上不可辨认。
   - 同步官方后继续搜索 `scheduleTokenRefresh`、`startAutoRefresh`、`isAccessTokenExpired`、`auth-token-refreshed`、`AuthLayout.vue`。重点确认登录成功后空闲页面不会持续刷 `/api/v1/auth/refresh`，真实 401 仍能刷新重放。

2. `57be2738 feat: inject pages public settings`

   - Cloudflare Pages 前台发布前新增 public settings 注入流程：`scripts/cloudflare-pages-config.mjs` 从对应环境 `/api/v1/settings/public` 只取公开 `data` 字段，`scripts/inject-pages-public-settings.mjs` 写入 `backend/internal/web/dist/index.html` 的 `window.__APP_CONFIG__` 并替换 `<title>`。
   - `frontend/public/_worker.js` 和 Pages Worker 测试补齐 a2 / ap2 回源与公开只读接口边缘短缓存，保证 `/api/*`、`/health`、`/51Token/*` 等需要回源的路径继续走 VPS API 域名。
   - 部署文档明确 Pages 静态前台不再执行 VPS Go 内嵌注入；不同服务可以使用多套前台产物，运行时公开配置发布前注入，构建期配置仍要按环境分别打包。
   - 同步官方后继续搜索 `inject-pages-public-settings.mjs`、`cloudflare-pages-config.mjs`、`window.__APP_CONFIG__`、`EDGE_CACHEABLE_PUBLIC_PATHS`、`X-Sub2API-Edge-Cache`。不得把 `.env`、后台配置、Token secret、Turnstile secret 等私有配置写入静态文件。

3. `c78042fb fix: separate OpenAI main usage from Spark display`

   - 将 OpenAI Codex 用量显示拆成主套餐和 Spark 两套来源：主套餐优先读 `codex_main_*`，Spark 展开区只读明确 Spark 字段 `codex_*`，避免 Spark 快照被通用 `five_hour/seven_day` 或 raw fallback 污染。
   - OpenAI header 快照更新时保留真实上游模型归属，按模型族写入不同字段；普通用户响应和普通用量记录继续隐藏真实上游模型。
   - 账号管理表补默认可见账号 ID，便于排查账号 17 这类具体 OAuth 账号的用量错位。
   - 同步官方后继续搜索 `codex_main_5h`、`codex_main_7d`、`codex_usage_updated_at`、`openAIMainFiveHour`、`openAICodexSparkWindows`、`result.UpstreamModel`、`ResponseHeaders`。必须保持“主套餐不吃 Spark 快照，Spark 不吃主套餐或 raw 兜底”的边界。

4. `feat: show Headroom stats and bypass large Codex requests`

   - 管理端运维面板新增 Headroom 统计卡片 `OpsHeadroomStatsCard.vue`，展示 Headroom token 节省、请求数、节省率、命中情况等指标，便于前台直接查看使用 Headroom 后节省了多少 token。
   - `frontend/src/api/admin/ops.ts` 扩展 Headroom 统计类型与接口消费，`OpsDashboard.vue` 挂载卡片，中英文文案和 Vitest 测试同步补齐。
   - 增加 Headroom / Codex 大请求旁路配置和相关测试，确保大 payload 或复杂 streaming 请求可绕过 Headroom 并在短 TTL 内保持同一 session 后续请求直连，降低 Headroom 对大请求的 502/超时风险。
   - 同步官方后继续搜索 `OpsHeadroomStatsCard`、`headroom`、`HeadroomStats`、`getHeadroomStats`、`GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_BYPASS_BODY_BYTES`、`GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_BYPASS_TTL_SECONDS`。如果官方调整 Ops dashboard 或 OpenAI OAuth 转发结构，继续保留“管理员可直接查看 Headroom 节省统计”和“大请求可绕过 Headroom”的入口。

### 复查建议

- 推送前建议重新执行：`git log --oneline origin/subapi..HEAD`、`git status --short`、`git diff --check`。
- 若要部署 a2，继续按 `/opt/sub2api-ap2-deploy/.env` + `sub2api-ap2` 独立 compose 流程，不要把 ap2 误切到 primary/standby 滚动脚本。

## 2026-06-13 相关改动记录

### Auth refresh 与登录页

- 提交：`9d440fc4 fix: avoid oauth refresh loop for short tokens`；上一轮相关提交为 `762df86a fix: prevent auth refresh loop on login`。
- 修复短有效期 access token 登录后反复刷新：短 token 不再安排主动 refresh 定时器，恢复已过期 token 时不后台 refresh，等待真实 API 请求遇到 401 后由 axios 拦截器刷新并重放。
- 修复长有效期 token 触发浏览器 `setTimeout` 上限导致立即 refresh：`frontend/src/stores/auth.ts` 增加分段等待，30 天 token 不会在登录后立刻请求 `/api/v1/auth/refresh`。
- `frontend/src/api/client.ts` 在 refresh 成功后派发 `auth-token-refreshed`，让 Pinia auth store 同步新 token 和过期时间。
- `frontend/src/components/layout/AuthLayout.vue` 给登录页站点 logo 外层补白色圆角背景，避免透明 logo 在登录页背景上不可辨认。
- 验证重点：`frontend/src/stores/__tests__/auth.spec.ts`、`frontend/src/api/__tests__/client.spec.ts`；本地登录后空闲页面不应持续刷 `/api/v1/auth/refresh`，真实 401 仍应能刷新重放。
- 同步官方后继续搜索 `scheduleTokenRefresh`、`MAX_TOKEN_REFRESH_DELAY`、`performTokenRefresh`、`auth-token-refreshed`、`AuthLayout.vue`。

### Cloudflare Pages public settings 注入与边缘短缓存

- 提交：`57be2738 feat: inject pages public settings`。
- 新增 `scripts/cloudflare-pages-config.mjs` 与 `scripts/inject-pages-public-settings.mjs`：Pages 发布前从对应环境 `/api/v1/settings/public` 拉取公开配置，只取 wrapper 里的 `data`，写入 `index.html` 的 `window.__APP_CONFIG__` 并替换 `<title>`。
- `frontend/public/_worker.js` 增加公开只读 GET 接口短缓存，限于 `/api/v1/settings/public`、`/api/status`、`/api/home_page_content`；剥离回源请求中的 `Authorization` 和 `Cookie`，认证/后台/支付/OAuth/模型网关/长流式接口不缓存。
- `frontend/src/cloudflare/__tests__/pages-config-injection.spec.ts` 和 `frontend/src/cloudflare/__tests__/pages-worker.spec.ts` 覆盖注入幂等、公开配置解包、缓存 HIT/MISS、敏感请求头剥离和禁止缓存写/网关接口。
- 文档补充：Pages 静态前台不会执行 VPS Go 内嵌注入；运行时公开配置可以发布前 inject，构建期配置必须按环境分别 build。
- 同步官方后继续搜索 `inject-pages-public-settings.mjs`、`cloudflare-pages-config.mjs`、`window.__APP_CONFIG__`、`EDGE_CACHEABLE_PUBLIC_PATHS`、`X-Sub2API-Edge-Cache`。不得把 `.env`、后台配置、Token secret、Turnstile secret 写入静态文件。

### OpenAI 主套餐与 Spark 用量拆分

- 提交：`c78042fb fix: separate OpenAI main usage from Spark display`。
- 后端按 OpenAI Codex 模型族写入用量快照：Spark 写 `codex_5h_*` / `codex_7d_*`，主套餐写 `codex_main_5h_*` / `codex_main_7d_*`，避免两套窗口互相覆盖。
- 前端 `AccountUsageCell` 主套餐区域优先读 `codex_main_*`，Spark 展开区只读明确 Spark 字段，不再用 raw `codex_primary/secondary` 兜底污染。
- 账号管理列表新增默认可见 ID 列，便于定位账号 17、47 等具体 OAuth 账号。
- 验证重点：`AccountUsageCell.spec.ts` 复现“主套餐 22%、Spark 0%、通用 seven_day 95%”错位场景；`AccountsView.usageWindowsHint.spec.ts` 覆盖 ID 列。
- 同步官方后继续搜索 `codex_main_5h`、`codex_main_7d`、`codex_usage_updated_at`、`openAIMainFiveHour`、`openAICodexSparkWindows`、`result.UpstreamModel`、`ResponseHeaders`。必须保持“主套餐不吃 Spark 快照，Spark 不吃主套餐或 raw 兜底”。

### a2 Headroom 第一跳代理修复

- 现象：a2 / ap2 经 `headroom-a2` 请求 `/51Token/v1/responses` 时出现 502，但同账号直连和容器内访问 `headroom-a2` 可通。
- 根因：OpenAI OAuth Forward / passthrough Forward 对 Docker 内网 sidecar URL 仍套用账号 SOCKS 代理，`socks5h` 在代理端解析 `headroom-a2` 失败。
- 修复：`openAICodexHTTPProxyURL()` 对 localhost、loopback、私网 IP、无点号 Docker/service hostname 的 override URL 不使用账号 proxy；外部 `chatgpt.com` 等目标仍保留账号 proxy。
- 验证：`TestOpenAIGatewayService_OAuthCodexHeadroomOverrideBypassesAccountProxy` 等 OpenAI OAuth passthrough 测试。
- 同步官方后继续搜索 `openAICodexHTTPProxyURL`、`isOpenAIInternalCodexOverrideRequest`、`OpenAIOAuthCodexResponsesURL`、`headroom-a2`。

### a1 Chat Completions Headroom 第一跳代理修复

- 提交：`c745d7fd fix: bypass account proxy for headroom chat completions`。
- 现象：a1 / ap1 走 `/v1/chat/completions` 兼容入口请求 `GPT-5.5` 时，后端已选中 OAuth 账号，但转发到 `http://headroom-a1:8787/v1/responses` 报 `socks connect tcp 172.17.0.1:40001->headroom-a1:8787: unexpected EOF`，Cloudflare 表现为 `ap1.upit.top 502`。
- 根因：2026-06-13 的 Headroom 代理绕过只覆盖 `/v1/responses` 的普通 Forward / passthrough Forward；Chat Completions 转 Responses 的发送路径仍直接使用 `account.Proxy.URL()`。
- 修复：`backend/internal/service/openai_gateway_chat_completions.go` 发送上游请求时复用 `openAICodexHTTPProxyURL(account, upstreamReq)`。
- 验证：`TestForwardAsChatCompletions_OAuthHeadroomOverrideBypassesAccountProxy` 与 `TestOpenAIGatewayService_OAuthCodexHeadroomOverrideBypassesAccountProxy`。
- 同步官方后继续搜索 `ForwardAsChatCompletions`、`openAICodexHTTPProxyURL`、`headroom-a1`、`headroom-a2`；不要只保留 `/v1/responses` 路径的保护，Chat Completions 兼容入口也必须绕过 Docker 内网 Headroom 第一跳的账号 SOCKS。

## 2026-06-14 相关改动记录

### Headroom 统计卡片、后台开关与大请求旁路

- 提交：`7b38f97d feat: show Headroom stats and bypass large Codex requests`。
- 管理端运维面板新增 `OpsHeadroomStatsCard.vue`，展示 Headroom token 节省、请求数、节省率、命中情况和不可用状态；刷新按钮放在 “Headroom 压缩统计” 标题右侧。
- `frontend/src/api/admin/ops.ts` 扩展 Headroom 统计类型与接口消费，`OpsDashboard.vue` 挂载卡片，中英文文案和 Vitest 测试同步补齐。
- 新增后台全局设置 `openai_headroom_enabled` / “Headroom 压缩代理”：`.env` 中的 `GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_URL` 只表示 sidecar 能力，实际转发由后台开关决定。
- 开关关闭时，普通 Forward、passthrough Forward、WS v2 URL 都回官方 Codex endpoint；大请求旁路和 `compression_refused` 短 TTL 直连记忆只在 Headroom 开启时生效。
- 新增 Headroom / Codex 大请求旁路配置和测试，降低大 payload 或复杂 streaming 请求导致的 Headroom 502/超时风险。
- 同步官方后继续搜索 `OpsHeadroomStatsCard`、`HeadroomStats`、`getHeadroomStats`、`SettingKeyOpenAIHeadroomEnabled`、`openai_headroom_enabled`、`GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_BYPASS_BODY_BYTES`、`GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_BYPASS_TTL_SECONDS`。

### 三套 Cloudflare Pages 正式前台与回源映射

- 提交：`612486f1 docs: record Pages routing and Headroom worker ops` 中包含代码和文档更新。
- 三套正式前台都按独立 Pages 项目发布：`sub2api-frontend-main`、`sub2api-frontend-a1`、`sub2api-frontend-a2`。
- `frontend/public/_worker.js` 按正式前台 host 显式映射 API origin：`ai.upit.top -> https://api.upit.top`、`a1.upit.top -> https://ap1.upit.top`、`a2.upit.top -> https://ap2.upit.top`；只有预览域名才读取 `SUB2API_ORIGIN`。
- 三套发布流程固定为 `build -> copy three dist dirs -> inject public settings -> wrangler pages deploy`；后端/API 常规发布默认 `--skip-frontend-build`。
- 验证重点：`pages-worker.spec.ts`、`pages-config-injection.spec.ts`、三套 `/login` HTML 中的 `window.__APP_CONFIG__` 和各自 `api_base_url`、三套 `/health` 和 `api/ap1/ap2` `/health`。
- 同步官方或调整 Pages 时继续搜索 `PRODUCTION_API_ORIGINS_BY_HOST`、`sub2api-frontend-main`、`sub2api-frontend-a1`、`sub2api-frontend-a2`、`inject-pages-public-settings.mjs`。不要把三套正式前台合并到同一个注入后的 `index.html`。

### 新 VPS 迁移、共享数据层与旧 VPS 备份

- 新 VPS 使用 SSH 别名 `51tokens`，旧 VPS 继续用 `51token-vps`；迁移期间保持别名分离。
- 三套 sub2api 后端运行在新 VPS：`sub2api`、`sub2api-ap1`、`sub2api-ap2`，对应本机端口 `8081/8082/8083`。
- 三套环境共用 `sub2api-postgres` 和 `sub2api-redis`：PostgreSQL database 为 `sub2api`、`sub2api_ap1`、`sub2api_ap2`，Redis DB 为 `0/1/2`。
- 旧 VPS 最终备份保留在新 VPS `/opt/migration-backups/old-vps-final-20260614-210956/`，包含 `.tar.zst` 与 `.sha256`，已在新 VPS 手工校验；用户明确不需要下载到本地。
- 旧 VPS 下线前仍需验证 Cloudflare DNS、Pages 前台、API 回源、后端容器、crontab、Caddy、sidecar 和备份状态，不能仅凭新 VPS 有流量就销毁。

### Headroom worker 限制与资源结论

- primary 和 a1 继续启用 Headroom：`sub2api -> headroom-main`、`sub2api-ap1 -> headroom-a1`；a2 不启用 Headroom。
- 当前 Headroom 镜像的 `headroom proxy` CLI 没有把 `HEADROOM_COMPRESSION_MAX_WORKERS` 灌进 `ProxyConfig`，单纯加 env 后 `/health` 仍显示 `max_workers=8/source=auto`。
- 新增 VPS 侧 wrapper `/opt/headroom_start_with_compression_workers.py`，在启动原 `headroom proxy` 前 monkey-patch `ProxyConfig`，把 `HEADROOM_COMPRESSION_MAX_WORKERS=2` 显式传入 `compression_max_workers`；compose 挂载该 wrapper 并改 entrypoint。
- `HEADROOM_WORKERS=1` 保持不变，它只代表 Uvicorn worker 进程数，不是压缩 executor 限制。
- 实测 `/health`：`headroom-main` 与 `headroom-a1` 的 `compression_executor.max_workers=2`、`source=explicit`；a2 无 Headroom override。
- 资源排查结论：Redis/PostgreSQL 已共享且占用不高；当前内存大头是 `headroom-main` 与 `headroom-a1` 两个 Python 进程。大 Codex 请求进入 Headroom 后可能出现 `run_seconds_max` 超过 `compression_timeout_seconds=30s`、`leaked_threads_total` 增长和 `compression_refused`，后续应优先降低进入 Headroom 的请求体阈值、只保留一套 Headroom 或加重启/内存保护。
- 2026-06-15 a1 继续降载：`headroom-a1` 因历史压缩任务出现 `run_seconds_max=983s`、`leaked_threads_total=17`，且 150KB-230KB 请求仍会触发 `compression_refused`。线上将 `/opt/sub2api-ap1-deploy` 调整为 `HEADROOM_COMPRESSION_MAX_WORKERS=1`、`GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES_BYPASS_BODY_BYTES=131072`；验证后 `headroom-a1` 约 `162MiB`、`4` PIDs、`0.3%` CPU。同步官方或重建 compose 时不要把 a1 恢复成 `max_workers=2` / `1MB` 阈值，除非重新评估 2 核 4G VPS 余量。

### 运维监控移除 GPU 指标

- 运维监控面板不再展示 GPU 使用率，保留 CPU、内存、磁盘、DB、Redis、队列等与当前 VPS 直接相关的指标。
- 前端移除 `OpsDashboardHeader.vue` 的 GPU 卡片、`frontend/src/api/admin/ops.ts` 的 `gpu_usage_percent` 类型字段，以及中英文 GPU tooltip。
- 后端移除 `OpsSystemMetricsSnapshot` / `OpsInsertMetricsInput` 的 GPU 字段，`ops_metrics_collector.go` 不再调用 `nvidia-smi`，删除 `parseNvidiaSMIUtilizationCSV` 相关测试。
- repository 写入和读取 `ops_system_metrics` 时不再包含 `gpu_usage_percent`。
- `backend/migrations/145_add_ops_system_disk_gpu_metrics.sql` 保持已执行版本不变，继续包含旧的 GPU 字段定义以匹配线上 checksum；已执行过旧迁移的环境通过新增 `backend/migrations/157_remove_ops_gpu_metrics.sql` 删除遗留 `gpu_usage_percent` 列。不要再修改 145 这类已上线迁移。
- 验证：`go test ./internal/repository ./internal/service -run 'TestOps|TestWriteOpenAIFastPolicyBlockedResponseMarksBusinessLimited|TestOpsMetricsCollector' -count=1`；`pnpm --dir frontend run typecheck`；`git diff --check`。
- 同步官方后继续搜索 `gpu_usage_percent`、`nvidia-smi`、`GPUUsagePercent`、`collectGPUUsagePercent`，只允许迁移/维护文档保留“已移除”说明，不要恢复 GPU 卡片或采集。

### 2026-06-14 后续复查

- 部署前复跑：auth store / client 测试、Pages Worker / injection 测试、AccountUsageCell 测试、SettingsView 测试、Headroom stats 组件测试、`git diff --check`。
- 线上复查同时看：前台 Pages 域名、API 回源域名、容器健康、Headroom `/health`、Redis/PostgreSQL 连接分档、Cloudflare DNS/Pages 项目映射。

## 2026-06-15 相关改动记录

### 运维监控开关回显与宿主机 CPU 原因卡片

- 后台系统设置页修复 `ops_monitoring_enabled` 回显：`GET /api/v1/admin/settings` 返回数据库持久化值，不再和 `opsService.IsMonitoringEnabled()` 做 AND，避免“保存开启后刷新又显示关闭”。
- 运维监控新增宿主机快照接口 `/api/v1/admin/ops/host-health`。后端只读取 `SUB2API_HOST_HEALTH_PATH` 指向的 JSON 文件，默认 `/run/sub2api-ops/host-health.json`；文件缺失时返回 `available=false`，不在请求路径执行 `docker stats` 或 `ps`。
- 新增宿主机 collector：`deploy/sub2api-host-health-collector.py`、`deploy/sub2api-host-health.service`、`deploy/sub2api-host-health.timer`。VPS 宿主机每 15 秒写一次 `/run/sub2api-ops/host-health.json`，业务容器只读挂载。
- 管理端运维面板新增 `OpsHostHealthCard.vue`，展示宿主机 CPU、load、可用内存、swap、top containers、top processes 和诊断文本；随现有 dashboard refresh token 刷新。面板由前台构建变量 `VITE_OPS_HOST_HEALTH_VISIBLE=true` 控制，只在 a1/a2 Pages 构建中打开，主环境默认隐藏且不请求 host-health 接口。
- 本次同步审计后台开关联动：`ops_monitoring_enabled`、`ops_realtime_monitoring_enabled`、`openai_headroom_enabled`、`channel_monitor_enabled`、`available_channels_enabled`、`allow_user_view_error_requests` 均有前端保存、后端 DTO/落库或运行时读取链路；`ops_host_health_visible` 是只读环境字段，不进入后台保存请求。
- 2026-06-15 部署前发现已执行迁移 `145_add_ops_system_disk_gpu_metrics.sql` 被改动会触发线上 checksum mismatch；当前已恢复 145 原内容，GPU 删除只通过 `157_remove_ops_gpu_metrics.sql` 完成。
- 本次部署边界：只部署 a1/a2 后端和 `sub2api-frontend-a1`、`sub2api-frontend-a2` Pages；不要发布主环境 Pages，不要切换 primary 后端，也不要给主环境 compose 增加宿主机快照挂载。
- 线上发布记录：a1/a2 后端切到 `sub2api:subapi-7a7baea8-a1a2-host-health-env-20260615090353`；a1/a2 Pages 使用 `VITE_OPS_HOST_HEALTH_VISIBLE=true` 构建并重新注入各自 `ap1/ap2` public settings 后发布，主环境保持原后端镜像和原 Pages 注入。
- 同步官方后继续搜索 `ops_monitoring_enabled`、`VITE_OPS_HOST_HEALTH_VISIBLE`、`OpsHostHealthCard`、`GetHostHealth`、`SUB2API_HOST_HEALTH_PATH`、`sub2api-host-health`、`145_add_ops_system_disk_gpu_metrics.sql`、`157_remove_ops_gpu_metrics.sql`，保留“宿主机采集在 VPS，业务容器只读 JSON”、“前台构建变量决定 CPU 面板显示”和“已执行迁移不可变”的边界。

### 2026-06-15 a1 Claude Messages 经 Headroom 时被账号 SOCKS 代理导致 502

**现象：**
a1 / ap1 使用 Claude Code 或 Anthropic `/v1/messages` 兼容入口请求 `gpt-5.5` 时，客户端提示所选模型不可用；直接请求 `https://ap1.upit.top/51Token/v1/messages` 返回 Cloudflare `502`。

**原因：**
`/v1/messages` 由 `OpenAIGatewayService.ForwardAsAnthropic()` 转为 OpenAI Responses 后，会按 a1 配置发往 `http://headroom-a1:8787/v1/responses`。修复前该发送路径仍直接使用 `account.Proxy.URL()`，导致 Docker 内网服务名 `headroom-a1` 被交给账号 SOCKS 代理 `172.17.0.1:40001` 解析，出现 `socks connect tcp ... ->headroom-a1:8787` 失败。此前 `0a575f8e` 只覆盖了 `/v1/chat/completions` 兼容入口，未覆盖 `/v1/messages`。

**修改：**
- `backend/internal/service/openai_gateway_messages.go`：发送 Messages bridge 上游请求时复用 `s.openAICodexHTTPProxyURL(account, upstreamReq)`。
- `backend/internal/service/openai_compat_model_test.go`：新增 `TestForwardAsAnthropic_OAuthHeadroomOverrideBypassesAccountProxy`，覆盖 OAuth 账号有 SOCKS proxy 且 Headroom override 指向 Docker 内网时，第一跳必须不走账号 proxy。

**验证：**
```bash
cd backend
go test ./internal/service -run 'TestForwardAsAnthropic_OAuthHeadroomOverrideBypassesAccountProxy|TestForwardAsChatCompletions_OAuthHeadroomOverrideBypassesAccountProxy|TestOpenAIBuildOpenAIResponsesWSURLUsesOAuthCodexOverride|TestForwardAsAnthropic_OAuth' -count=1
git diff --check
```

线上验证必须包含真实 `/51Token/v1/messages` 和 Claude CLI 一次性 prompt；只看 `/v1/models` 不能证明 Messages bridge 已修复。

**同步官方后的复查：**
继续搜索 `ForwardAsAnthropic`、`ForwardAsChatCompletions`、`openAICodexHTTPProxyURL`、`OpenAIOAuthCodexResponsesURL`、`headroom-a1`、`headroom-a2`。任何新增的 OpenAI OAuth Codex / Responses / Anthropic Messages / Chat Completions 转发路径，只要目标是本机或 Docker 内网 Headroom sidecar，都必须复用同一代理选择规则。

### 2026-06-15 OpenAI OAuth Headroom 代理选择全链路审计

**结论：**
不能按外部请求路径是否包含 `/v1/` 来决定是否绕过账号 proxy。`/v1/chat/completions`、`/v1/responses`、`/v1/images/*` 既可能指向外部 OpenAI / 第三方兼容上游，也可能被后端转成 `http://headroom-a1:8787/v1/responses`。正确判断点是已经构造好的 upstream URL：只有最终目标是本机、私网 IP、loopback 或无点号 Docker service hostname，并且 scheme 为 `http` / `ws` 时，才绕过账号 proxy；外部 `https` / `wss` 上游仍必须保留账号 proxy。

**本次补漏：**
- `backend/internal/service/openai_gateway_service.go`：新增 `openAICodexWSProxyURL()`，并把 HTTP/WS 内网判断统一到 `isOpenAIInternalCodexOverrideURL()`。
- `backend/internal/service/openai_images_responses.go`：OpenAI Images 经 Responses/Headroom 的 OAuth 路径改用 `openAICodexHTTPProxyURL()`。
- `backend/internal/service/openai_ws_http_bridge.go`：WS 大帧 HTTP bridge 到 Responses/Headroom 时改用 `openAICodexHTTPProxyURL()`。
- `backend/internal/service/openai_ws_forwarder.go` 和 `backend/internal/service/openai_ws_v2_passthrough_adapter.go`：WS v2 passthrough / connection pool 到 `ws://headroom-*` 时改用 `openAICodexWSProxyURL()`。

**验证：**
```bash
cd backend
go test ./internal/service -run 'Test.*Headroom.*Proxy|Test.*Headroom.*Override|Test.*OAuthCodex.*Override|TestOpenAIWSHTTPBridge|TestOpenAIGatewayServiceForwardImages_OAuth|TestForwardAsAnthropic_OAuth|TestForwardAsChatCompletions_OAuth' -count=1
git diff --check
```

**同步官方后的复查：**
继续搜索 `OpenAIOAuthCodexResponsesURL`、`buildUpstreamRequest`、`buildUpstreamRequestOpenAIPassthrough`、`buildOpenAIResponsesWSURL`、`buildOpenAIImagesRequest`、`httpUpstream.Do`、`httpUpstream.DoWithTLS`、`Dial(`、`Acquire(`、`account.Proxy.URL()`。只要路径可能由 OpenAI OAuth Codex override 指向 Headroom，就必须按最终 upstream URL 调用 `openAICodexHTTPProxyURL()` 或 `openAICodexWSProxyURL()`；不要把外部 `/v1/*` 兼容上游的账号 proxy 去掉。

### 2026-06-18 风控中心开启后配置入口被旧 public settings 缓存拦截

**现象：**
管理员在系统设置里启用“风控中心”后，点击“前往 风控中心 配置内容审计”跳转到 `/admin/risk-control`，页面仍被路由守卫拦回 `/admin/settings`，看起来像开关没有生效。

**原因：**
`/admin/risk-control` 路由通过 `risk_control_enabled` 这个 public settings 开关控制。Cloudflare Pages 前台首屏会使用发布时注入的 `window.__APP_CONFIG__`，当前 SPA 内还会缓存 `cachedPublicSettings`。如果管理员刚保存开关，路由守卫只读旧缓存的 `false`，不会主动请求最新 `/api/v1/settings/public`，因此误判功能关闭。

**修改：**
- `frontend/src/utils/featureFlags.ts`：新增 `refreshAndResolveFeatureFlag()`，当当前缓存不允许访问时，强制刷新一次 public settings 后再解析功能开关。
- `frontend/src/router/index.ts`：`requiresRiskControl` 路由改用 `refreshAndResolveFeatureFlag(FeatureFlags.riskControl)`。
- `frontend/src/utils/__tests__/featureFlags.spec.ts`：覆盖旧缓存为 `false`、刷新后为 `true` 时应放行，以及当前已为 `true` 时不重复刷新的场景。

**验证：**
```bash
pnpm --dir frontend test:run src/utils/__tests__/featureFlags.spec.ts
pnpm --dir frontend test:run src/router/__tests__/wechat-route.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
pnpm --dir frontend typecheck
```

**同步官方后的复查：**
继续搜索 `requiresRiskControl`、`risk_control_enabled`、`refreshAndResolveFeatureFlag`、`FeatureFlags.riskControl`。如果官方引入统一的异步 feature flag 守卫，可以删除本地 helper；否则所有由 public settings opt-in 控制、且可能从设置页保存后立刻跳转的后台入口，都应在拦截前刷新一次 public settings。

### 2026-06-22 订阅级总量上限耗尽预检

**业务目的：**
为订阅套餐增加“总量上限”后，Google/Gemini 风格入口必须在订阅累计用量已经达到上限时拒绝下一次请求；同时仍允许一次请求在预估扣费时刚好用完剩余额度。这个入口主要依赖 `SubscriptionService.ValidateAndCheckLimits()` 中间件预检，否则总量刚好耗尽时可能漏放一次请求。

**修改：**
- `backend/internal/service/user_subscription.go`：抽出 `checkUsageWithinLimit()`，统一 `CheckDailyLimit`、`CheckWeeklyLimit`、`CheckMonthlyLimit`、`CheckTotalLimit` 的边界语义；`additionalCost <= 0` 的请求预检使用 `used < limit`，带预估扣费时仍允许 `used + additionalCost == limit`。
- `frontend/src/views/user/KeysView.vue`：CC Switch 导入脚本计算订阅剩余额度时必须把 `total_limit_usd - total_usage_usd` 纳入 `Math.min(...)`，否则客户端展示的 remaining 可能高于真实可用额度。
- `backend/internal/server/middleware/api_key_auth_google_test.go`、`backend/internal/service/user_subscription_daily_quota_test.go`：覆盖总量刚好耗尽时服务层和 Google/Gemini 入口都返回限额错误。

**验证：**
```bash
cd backend
go test ./internal/service -run 'Test(UserSubscriptionCheckTotalLimit|ValidateAndCheckLimits_TotalLimitExactlyExhausted|ValidateAndCheckLimits_TotalLimitExceeded)' -count=1
go test ./internal/server/middleware -run 'TestApiKeyAuthWithSubscriptionGoogle_(SubscriptionLimitExceededReturns429|SubscriptionTotalLimitExactlyExhaustedReturns429)' -count=1
go test ./internal/service -count=1
go test ./internal/handler ./internal/handler/admin ./internal/server/middleware -count=1
go test ./internal/repository -count=1
go test ./... -run TestNonExistent -count=1
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/admin/__tests__/subscriptionProgressMerge.spec.ts src/stores/__tests__/subscriptions.spec.ts
```

**同步官方后的复查：**
继续搜索 `TotalLimitUSD`、`total_limit_usd`、`total_usage_usd`、`CheckTotalLimit`、`ValidateAndCheckLimits`、`ErrTotalLimitExceeded`、`APIKeyAuthWithSubscriptionGoogle`、`executeCcsImport`。如果官方调整订阅预检、Google/Gemini 鉴权链路或用户侧 Key 导入脚本，必须确认“已用量等于总量上限时下一次请求返回 429”仍成立，且客户端导入脚本的 remaining 会受总量上限约束；同时保留“预估本次扣费刚好用完剩余额度可通过”的行为，避免提前拒绝最后一次合法请求。

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
