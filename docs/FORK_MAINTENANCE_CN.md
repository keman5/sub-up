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

**文档拆分约定：**

- `docs/FORK_MAINTENANCE_CN.md` 保持为主入口，记录维护原则、自动化命令、关键长期补丁表和明细文件索引。
- 较长的每日改动说明、验证命令和同步官方后的复查点写入 `docs/fork-maintenance/YYYY-MM.md`。主文档只保留简短索引，避免所有细节按日期无限追加到同一个文件。
- 现有 `check-doc` / `record` 仍会先写入主文档；提交前如果记录较长，应手动搬到当月明细文件，并在主文档索引中补一行。

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
| 2026-06-02 | `ap2/a2` 灰度环境并不复用 `sub2api-standby`，而是单独运行 `sub2api-ap2` compose；如果沿用 `--deploy` 的双环境滚动脚本，会误更新 standby / primary，而不是只更新 ap2。 | 线上确认 `ap2` 目录为 `/opt/sub2api-ap2-deploy`，通过 `.env` 中的 `IMAGE_TAG=` 控制镜像；单独部署 ap2 时，先用 `HOST=51tokens deploy/local-gzip-binary-deploy.sh --apply` 完成本地构建、gzip 分块上传、远端解压和打镜像，再仅修改 `sub2api-ap2` 的 `.env` 并执行 `docker compose up -d sub2api-ap2`；2026-06-02 实际将 ap2 从 `sub2api:subapi-a5e4b0c6-ap2-oauth-adaptive-v2-202606021754` 升级到 `sub2api:subapi-9d75fb6b-ap2-redeploy-20260602232707`。 | `ssh 51tokens 'grep ^IMAGE_TAG= /opt/sub2api-ap2-deploy/.env'`；`ssh 51tokens 'docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" sub2api-ap2'`；`ssh 51tokens 'curl -fsS http://127.0.0.1:8083/health'`；`ssh 51tokens 'docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep sub2api-ap2'`。 | 同步官方或后续重构部署脚本后，优先确认 `ap2` 是否仍保持独立 compose 目录 `/opt/sub2api-ap2-deploy` 与 `IMAGE_TAG` 控制方式；如果未来把 ap2 并回 standby 或纳入统一脚本，先更新 `docs/VPS_DEPLOY_NOTES.md` 的单独部署步骤，再删除这条本地差异说明。 |
| 2026-06-03 | 首页和 API Key 使用弹窗里的 Claude/Anthropic 配置不能直接复用带路径的 OpenAI/Codex `api_base_url`；否则 Claude Code 会在该路径后拼 `/v1/messages` 并检查失败。 | 前台配置生成逻辑从公开设置里的 OpenAI/Codex base 派生 Claude base：`buildClaudeBaseUrl()` 和 `buildAnthropicBaseUrl()` 对绝对 URL 保留 51token 业务前缀并去掉末尾 `/v1`，因此 `https://ap1.upit.top/51Token/v1` 会生成 `ANTHROPIC_BASE_URL=https://ap1.upit.top/51Token`；只有输入纯域名时才补 `/51Token`。Antigravity 则生成 `https://<host>/51Token/antigravity`。这属于前台配置问题，不需要修改 sub2api Go 路由，也不硬编码某个业务域名。 | `pnpm --dir frontend run typecheck`；打开首页和 API Key 使用弹窗，确认 Claude/Anthropic 配置中的 `ANTHROPIC_BASE_URL` 是当前域名加 `/51Token` 且不带 `/v1`，Codex/OpenAI 配置仍保留公开设置中的完整 API base。 | 同步官方后搜索 `buildClaudeBaseUrl` 和 `buildAnthropicBaseUrl`。如果官方改动了首页示例或 API Key 使用弹窗，必须继续确认 Claude/Anthropic 配置从带路径的 OpenAI/Codex base 派生时会保留 `/51Token` 并去掉末尾 `/v1`。 |
| 2026-06-03 | Claude Code 配置示例缺少模型环境变量，用户复制 `~/.claude/settings.json` 后会落到 Claude 默认模型或错误模型，无法稳定使用 51token 的 GPT 模型。 | 首页 `frontend/src/views/home/components/homeData.ts` 与 API Key 使用弹窗 `frontend/src/components/keys/UseKeyModal.vue` 的 Claude 配置示例统一加入 `ANTHROPIC_MODEL=gpt-5.5`、`ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-5.5`、`ANTHROPIC_DEFAULT_HAIKU_MODEL=gpt-5.5`、`ANTHROPIC_DEFAULT_OPUS_MODEL=gpt-5.5`、`ANTHROPIC_REASONING_MODEL=gpt-5.5`，并保留 `CLAUDE_CODE_ATTRIBUTION_HEADER=0` 与 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`；三语 README 的普通 Claude 与 Antigravity Claude 示例至少同步补充基础模型变量。 | `pnpm --dir frontend run typecheck`；打开首页和 API Key 使用弹窗，确认 `~/.claude/settings.json`、Terminal、Command Prompt、PowerShell 示例都包含完整模型相关字段和 Claude Code 非必要流量/归因开关。 | 同步官方后搜索 `ANTHROPIC_MODEL`、`ANTHROPIC_DEFAULT_SONNET_MODEL`、`ANTHROPIC_DEFAULT_HAIKU_MODEL`、`ANTHROPIC_DEFAULT_OPUS_MODEL`、`ANTHROPIC_REASONING_MODEL`、`CLAUDE_CODE_ATTRIBUTION_HEADER`、`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`、`generateAnthropicFiles` 和 `buildClaudeConfigJson`，确认模型字段没有被上游示例覆盖丢失。 |
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
| 2026-06-08 | a1 环境长期沿用 `standby` 命名，容易和真正热备/回滚环境混淆；后续部署脚本也会继续寻找旧 `/opt/sub2api-standby-deploy`。 | 线上将 `/opt/sub2api-standby-deploy` 无损迁移为 `/opt/sub2api-ap1-deploy`，容器改名为 `sub2api-ap1`、`sub2api-ap1-postgres`、`sub2api-ap1-redis`，保留宿主机 `8082` 与 `a1.upit.top` 路由不变；`deploy/local-gzip-binary-deploy.sh` 默认滚动目标改为 ap1 + primary，并保留旧 `STANDBY_*` 环境变量兼容；`tools/fork-maintenance/fork_maintenance.py` 的登录条款恢复目标同步改为 `sub2api-ap1-postgres`；`docs/VPS_DEPLOY_NOTES.md` 和本文档更新当前命名。 | `deploy/local-gzip-binary-deploy.sh --skip-frontend-build --skip-backend-build --deploy` dry-run 确认目标为 `/opt/sub2api-ap1-deploy` 与 `sub2api-ap1`；`ssh 51tokens 'docker inspect ... sub2api-ap1 sub2api-ap1-postgres sub2api-ap1-redis'` 均为 `healthy`；`curl -fsS http://127.0.0.1:8082/health`；`curl -fsS https://a1.upit.top/health`；`bash -n deploy/local-gzip-binary-deploy.sh`；`python3 -m py_compile tools/fork-maintenance/fork_maintenance.py`；`git diff --check`。 | 同步官方或后续调整部署拓扑后，继续搜索 `sub2api-standby`、`STANDBY_COMPOSE_DIR` 和 `/opt/sub2api-standby-deploy`。历史记录可保留旧名，但当前部署脚本、恢复脚本和线上操作说明必须默认指向 `sub2api-ap1`。 |
| 2026-06-08 | 账号管理页默认筛选为“全部状态”，运营首次进入时会混入停用、异常和不可调度账号，不利于优先处理正常可用账号。 | `frontend/src/views/admin/AccountsView.vue` 将账号列表初始 `status` 改为 `active`，让状态筛选默认选中“正常”；保留筛选下拉里的“全部状态”选项，运营仍可手动切回全量。`frontend/src/views/admin/__tests__/AccountsView.searchSuggest.spec.ts` 增加首次加载默认带 `status: active` 的回归测试。 | `pnpm --dir frontend exec vitest run src/views/admin/__tests__/AccountsView.searchSuggest.spec.ts`；`pnpm --dir frontend run typecheck`；`git diff --check`。 | 同步官方后搜索 `AccountsView.vue` 的 `initialParams.status` 和 `AccountsView.searchSuggest.spec.ts`。如果官方重构账号筛选或表格加载逻辑，继续保持“首次进入默认只看正常账号，但允许手动切回全部状态”的运营口径。 |
| 2026-06-08 | 账号管理用量窗口里 Spark 使用量仍和主套餐显示相同；cliproxyapi 可单独查询 Spark，说明本地把两种套餐快照混到同一组字段。 | 将 OpenAI Codex quota 快照按模型族分开：请求/探测模型为 `gpt-5.3-codex-spark` 时写 `codex_5h_*` / `codex_7d_*` 与 `codex_usage_updated_at`，非 Spark Codex 主套餐模型写 `codex_main_5h_*` / `codex_main_7d_*` 与 `codex_main_usage_updated_at`。`AccountUsageService` 主套餐区域只优先从 `codex_main_*` 构造 `five_hour/seven_day`，不得把 Spark 的 `codex_*` 历史快照提升为主套餐；Spark 展开区读取明确 Spark 字段，并避免在缺少 Spark 更新时间时被 raw primary/secondary 兜底污染。 | `go test ./internal/service -run 'TestBuildCodexUsageExtraUpdates|TestAccountUsageService_GetOpenAIUsage|TestExtractOpenAICodexProbeUpdates|TestHandle429_OpenAI|TestOpenAIModelRouter|TestBuildOpenAICodexProbePayload|TestCodexSnapshotBaseTime|TestCodexResetAtRFC3339' -count=1`；`go test ./internal/service -count=1`；`pnpm --dir frontend exec vitest run src/components/account/__tests__/AccountUsageCell.spec.ts src/views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts`；`pnpm --dir frontend run typecheck`；`git diff --check`。 | 同步官方后搜索 `codex_main_5h`、`codex_main_7d`、`buildCodexUsageExtraUpdatesForFamily`、`openAICodexUsageFamilyForModel`、`openAIMainFiveHour` 和 `openAICodexSparkWindows`。如果官方提供独立 Spark/主套餐管理接口，可收敛主动探测实现，但必须保留“模型族决定写入字段、Spark 不被主套餐或 raw 兜底覆盖、主套餐不吃 Spark 快照”的边界。 |
| 2026-06-09 | OpenAI 路由内二次账单检查仍有“额度已超限后直接拒绝”问题：`/v1/chat/completions`、`/v1/images/*`、`/v1/embeddings` 分支未复用中间件接力结果，导致同一请求仍被 2 次额度拦截。 | 在 `backend/internal/handler/openai_gateway_handler.go` 增加统一兜底 `enforceBillingEligibilityWithFallback` 与 `billingErrorDetailsWithFallback`，并在 `NewOpenAIGatewayHandler` 注入同一 `subscriptionService`（`backend/cmd/server/wire_gen.go`）；OpenAIGateway `ChatCompletions`、`Embeddings`、`Images` 改为统一调用该兜底路径，在二次检查失败且命中限额错误时触发 `ResolveQuotaFallback` 后重试。 | `go test ./internal/handler -run TestEnforceBillingEligibilityWithFallback -count=1`；`go test ./internal/handler -count=1`；`go test ./... -run TestNonExistent -count=1`；`ssh 51tokens 'grep ^IMAGE_TAG= /opt/sub2api-ap2-deploy/.env'`；`ssh 51tokens 'docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" sub2api-ap2'`；`ssh 51tokens 'docker inspect -f "{{.Config.Image}}" sub2api-ap2'`；`ssh 51tokens 'curl -fsS http://127.0.0.1:8083/health'`；`git diff --check`。| 已在 ap2 执行：`IMAGE_TAG=sub2api:subapi-aebdd6f1-a2-redeploy-20260609075235`；`docker inspect -f` 健康检查返回 `healthy`；`docker inspect -f '{{.Config.Image}}'` 返回同镜像；`curl` 返回 `{"status":"ok"}`；`docker compose` 为 `sub2api-ap2` 使用该镜像。 | 同步官方后核对 `OpenAIGatewayHandler` 在 `ChatCompletions`/`Embeddings`/`Images` 二次检查路径是否仍有接力重试逻辑；并复核 `subscriptionService` 注入链路（`wire_gen.go`）、`ResolveQuotaFallback` 与 `GetGroup`/`GetQuotaFallback` 上游行为。新增测试文件 `backend/internal/handler/openai_chat_completions_billing_fallback_test.go` 建议保留作为回归。 |

## 2026-06-06 未 push 改动梳理

### 本地未推送提交

1. `1586b1ef feat: improve search suggestions and local frontend build`

   - 管理端与用户端搜索建议输入框统一抽成 `frontend/src/components/common/SearchSuggestInput.vue`，覆盖账号管理、使用记录、订阅管理和用户 API Key 场景，支持输入搜索与下拉建议并存，选中后回填邮箱。
   - `backend/internal/handler/admin/usage_handler.go` / `usage_handler_search_users_test.go` 扩展用户搜索结果展示字段，前端 `frontend/src/api/admin/usage.ts` 和各视图同步消费。
   - 本地前端构建侧继续收口：`frontend/src/i18n/index.ts` 调整动态导入，`frontend/src/views/admin/AccountsView.vue` 若干弹窗改按需加载，`frontend/vite.config.ts` 增加管理端与用户端手动分包，降低首屏包体积。
   - 这轮又继续补了搜索建议交互收口：`AccountsView` 只允许 `change` 触发表格 reload，避免账号管理聚焦搜索框就重复请求 `/api/v1/admin/accounts`；`UsageFilters`、`SubscriptionsView`、`KeysView` 统一补了 `blur` 收起下拉，避免失焦后悬挂。
   - 对应回归测试已覆盖 `SearchSuggestInput`、`AccountTableFilters`、`UsageFilters`、`AccountsView.searchSuggest`；后续若 rebase/upstream 覆盖，需要优先复查 `SearchSuggestInput`、`UsageFilters`、`AccountTableFilters`、`AccountsView`、`SubscriptionsView`、`KeysView` 和 `manualChunks`。

2. `e33e265e feat: update ssh alias docs and prefill key group`

   - 将部署/运维说明统一切换到 SSH 别名 `51tokens`，涉及 `deploy/local-gzip-binary-deploy.sh`、[docs/VPS_DEPLOY_NOTES.md](/Users/okk/git-projects/sub2api/docs/VPS_DEPLOY_NOTES.md) 和 fork 维护脚本。
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

## 明细记录文件

较长的每日补丁记录从 2026-06-10 之后拆到月度文件，主文档只保留索引。

| 月份 | 明细文件 | 说明 |
| --- | --- | --- |
| 2026-06 | `docs/fork-maintenance/2026-06.md` | 2026-06-10 之后的本地补丁表格行、长篇专题记录、账号管理弹窗文案与 Compact 专属模型映射等明细。 |

### 2026-07-06: 自动记录本地改动

**自动记录：**

- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。
- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。

**涉及文件：**

- `frontend/src/views/admin/__tests__/SubscriptionsView.duplicateAssign.spec.ts`

**验证：**

```bash
TODO: 填写验证命令
```

**同步官方后的复查：**

- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。

### 2026-07-07: 移除 Headroom 代理残留

**业务目的：**

- 线上已不再使用 Headroom，代码和部署侧都不应继续保留可被误开启的 Headroom sidecar、Responses override、后台开关或统计入口。
- 本次补清 `gateway.openai_oauth_codex_responses_*` 默认配置，避免字段本体删除后仍留下不可见的 Viper 默认 key。

**涉及文件：**

- `backend/internal/config/config.go`

**验证：**

```bash
rg -n "OpenAIOAuthCodexResponses|openAIOAuthCodexResponses|openai_oauth_codex_responses|HeadroomStats|headroom/stats|OpsHeadroomStatsCard|openai_headroom_enabled|SettingKeyOpenAIHeadroomEnabled|isOpenAIHeadroomEnabled|HEADROOM_STATS|HEADROOM_COMPRESSION|HEADROOM_WORKERS|compression_refused|headroom proxy" backend frontend deploy .github scripts -g '!docs/**'
cd backend && go test ./internal/config ./internal/service
cd backend && go test ./internal/handler/admin -run 'TestOpsHostHealth|TestSettingHandler_GetSettings_ReturnsPersistedOpsMonitoringEnabledWithoutOpsService|TestSettingHandler_UpdateSettings_DoesNotPersistPartialSystemSettingsWhenAuthSourceDefaultsFail' -count=1
pnpm --dir frontend exec vitest run src/views/admin/ops/__tests__/OpsDashboard.hostHealth.spec.ts src/views/admin/ops/components/__tests__/OpsHostHealthCard.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
git diff --check
```

**同步官方后的复查：**

- 继续搜索 `headroom-main`、`headroom-a1`、`GATEWAY_OPENAI_OAUTH_CODEX_RESPONSES`、`HEADROOM_STATS`、`openai_headroom_enabled`、`OpsHeadroomStatsCard`、`HeadroomStatsService`。
- 当前生产和代码均不得恢复 Headroom sidecar、Responses override、后台开关或 stats API；如果官方也移除了相关能力，本地补丁可以删除。

### 2026-07-13: 自动记录本地改动

**自动记录：**

- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。
- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。

**涉及文件：**

- `backend/internal/service/client_error_localization.go`
- `backend/internal/service/client_error_localization_test.go`
- `README.md`
- `backend/cmd/server/VERSION`
- `backend/ent/group.go`
- `backend/ent/group/group.go`
- `backend/ent/group/where.go`
- `backend/ent/group_create.go`
- `backend/ent/group_update.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/schema/group.go`
- `backend/go.mod`
- `backend/internal/handler/admin/grok_oauth_handler_test.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/endpoint.go`
- `backend/internal/handler/endpoint_test.go`
- `backend/internal/handler/no_account_error.go`
- `backend/internal/handler/no_account_error_test.go`
- `backend/internal/handler/openai_alpha_search.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_gateway_compact_body_signal_test.go`
- `backend/internal/handler/openai_gateway_count_tokens.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/ops_capture_writer_nil_test.go`
- `backend/internal/pkg/apicompat/responses_stream_event_wire_test.go`
- `backend/internal/pkg/apicompat/types.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/repository/account_repo_integration_test.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/group_repo.go`
- `backend/internal/repository/http_upstream.go`
- `backend/internal/repository/http_upstream_test.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/routes/gateway_test.go`
- `backend/internal/service/account.go`
- `backend/internal/service/account_base_url_test.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/service/account_test_service_grok_test.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/api_key_auth_cache.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/billing_service_test.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/service/grok_oauth_service.go`
- `backend/internal/service/grok_oauth_service_test.go`
- `backend/internal/service/grok_quota_service.go`
- `backend/internal/service/grok_quota_service_test.go`
- `backend/internal/service/group.go`
- `backend/internal/service/media_price_config.go`
- `backend/internal/service/openai_alpha_search.go`
- `backend/internal/service/openai_alpha_search_billing_test.go`
- `backend/internal/service/openai_alpha_search_test.go`
- `backend/internal/service/openai_codex_function_call_id_test.go`
- `backend/internal/service/openai_codex_identity.go`
- `backend/internal/service/openai_codex_identity_test.go`
- `backend/internal/service/openai_codex_message_item_id_test.go`
- `backend/internal/service/openai_codex_transform.go`
- `backend/internal/service/openai_compact_body_signal.go`
- `backend/internal/service/openai_compact_sse_keepalive.go`
- `backend/internal/service/openai_compact_sse_keepalive_test.go`
- `backend/internal/service/openai_compat_model_test.go`
- `backend/internal/service/openai_gateway_cc_pipeline.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/openai_gateway_chat_completions_raw.go`
- `backend/internal/service/openai_gateway_forward.go`
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/openai_gateway_grok_cache.go`
- `backend/internal/service/openai_gateway_grok_cache_test.go`
- `backend/internal/service/openai_gateway_grok_chat_bridge.go`
- `backend/internal/service/openai_gateway_grok_chat_bridge_test.go`
- `backend/internal/service/openai_gateway_grok_test.go`
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback_test.go`
- `backend/internal/service/openai_gateway_responses_chat_fallback.go`
- `backend/internal/service/openai_gateway_scheduling.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/openai_gpt56_max_test.go`
- `backend/internal/service/openai_oauth_passthrough_test.go`
- `backend/internal/service/openai_ws_forwarder_ingress.go`
- `backend/internal/service/openai_ws_forwarder_payload.go`
- `backend/internal/service/openai_ws_forwarder_success_test.go`
- `backend/internal/service/openai_ws_http_bridge.go`
- `backend/internal/service/openai_ws_http_bridge_test.go`
- `backend/internal/service/openai_ws_pool.go`
- `backend/internal/service/openai_ws_pool_test.go`
- `backend/migrations/174_group_web_search_price_per_call.sql`
- `frontend/src/components/account/AccountUsageCell.vue`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/UsageProgressBar.vue`
- `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts`
- `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts`
- `frontend/src/components/account/__tests__/CreateAccountModal.grok.spec.ts`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/account/__tests__/UsageProgressBar.spec.ts`
- `frontend/src/components/keys/UseKeyModal.vue`
- `frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`
- `frontend/src/composables/__tests__/useGrokOAuth.spec.ts`
- `frontend/src/composables/useGrokOAuth.ts`
- `frontend/src/i18n/__tests__/openaiFastPolicyLocales.spec.ts`
- `frontend/src/i18n/__tests__/opsLocaleKeys.spec.ts`
- `frontend/src/i18n/locales/en/admin/overview.ts`
- `frontend/src/i18n/locales/en/admin/settings.ts`
- `frontend/src/i18n/locales/en/dashboard.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/i18n/locales/zh/admin/settings.ts`
- `frontend/src/i18n/locales/zh/dashboard.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/settings/OpenAIFastPolicyUserSelector.vue`
- `frontend/src/views/admin/settings/__tests__/OpenAIFastPolicyUserSelector.spec.ts`

**验证：**

```bash
TODO: 填写验证命令
```

**同步官方后的复查：**

- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。

### 2026-07-13: 自动记录本地改动

**自动记录：**

- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。
- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。

**涉及文件：**

- `.github/workflows/backend-ci.yml`
- `.gitignore`
- `README.md`
- `README_CN.md`
- `README_JA.md`
- `backend/cmd/server/VERSION`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/internal/handler/endpoint.go`
- `backend/internal/handler/failover_loop.go`
- `backend/internal/handler/failover_loop_test.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/gateway_handler_chat_completions.go`
- `backend/internal/handler/gateway_handler_responses.go`
- `backend/internal/handler/gateway_handler_usage_test.go`
- `backend/internal/handler/gateway_helper.go`
- `backend/internal/handler/gateway_helper_fastpath_test.go`
- `backend/internal/handler/gemini_v1beta_handler.go`
- `backend/internal/handler/grok_media.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `backend/internal/handler/payment_handler.go`
- `backend/internal/handler/payment_handler_resume_test.go`
- `backend/internal/pkg/apicompat/anthropic_responses_test.go`
- `backend/internal/pkg/apicompat/anthropic_to_responses_response.go`
- `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go`
- `backend/internal/pkg/apicompat/chatcompletions_responses_bridge_custom_tools_test.go`
- `backend/internal/pkg/apicompat/responses_to_anthropic.go`
- `backend/internal/pkg/apicompat/responses_to_anthropic_read_tool_test.go`
- `backend/internal/pkg/apicompat/responses_to_chatcompletions.go`
- `backend/internal/pkg/apicompat/streaming_stop_reason_test.go`
- `backend/internal/pkg/pagination/pagination.go`
- `backend/internal/pkg/pagination/pagination_test.go`
- `backend/internal/pkg/xai/oauth.go`
- `backend/internal/pkg/xai/oauth_test.go`
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/repository/api_key_repo_last_used_unit_test.go`
- `backend/internal/repository/concurrency_cache.go`
- `backend/internal/repository/concurrency_cache_integration_test.go`
- `backend/internal/repository/migrations_runner.go`
- `backend/internal/repository/migrations_runner_notx_test.go`
- `backend/internal/repository/scheduler_cache.go`
- `backend/internal/repository/scheduler_cache_unit_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/routes/gateway_test.go`
- `backend/internal/server/routes/payment.go`
- `backend/internal/service/account.go`
- `backend/internal/service/account_base_url_test.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/service/account_test_service_openai_test.go`
- `backend/internal/service/concurrency_service.go`
- `backend/internal/service/concurrency_service_test.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/service/model_not_found_error.go`
- `backend/internal/service/model_not_found_error_test.go`
- `backend/internal/service/openai_gateway_grok_test.go`
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/openai_gateway_responses_chat_fallback.go`
- `backend/internal/service/openai_ws_client.go`
- `backend/internal/service/openai_ws_client_test.go`
- `backend/internal/service/openai_ws_forwarder_ingress.go`
- `backend/internal/service/openai_ws_forwarder_ingress_session_test.go`
- `backend/internal/service/openai_ws_http_bridge_test.go`
- `backend/internal/service/openai_ws_pool.go`
- `backend/internal/service/openai_ws_pool_test.go`
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/ratelimit_service_model_not_found_test.go`
- `backend/internal/service/upstream_models.go`
- `backend/internal/service/upstream_models_test.go`
- `backend/internal/web/embed_on.go`
- `backend/internal/web/embed_test.go`
- `backend/internal/web/static_cache.go`
- `backend/internal/web/static_cache_test.go`
- `backend/migrations/174_add_usage_logs_api_key_latest_ip_index_notx.sql`
- `backend/migrations/latest_api_key_ip_index_test.go`
- `deploy/.env.example`
- `deploy/APPLE_CONTAINER.md`
- `deploy/README.md`
- `deploy/apple-container.sh`
- `deploy/config.example.yaml`
- `deploy/tests/apple-container-test.sh`
- `deploy/tests/fixtures/bin/container`
- `deploy/tests/fixtures/bin/curl`
- `frontend/src/api/payment.ts`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts`
- `frontend/src/components/account/__tests__/credentialsBuilder.spec.ts`
- `frontend/src/components/account/credentialsBuilder.ts`
- `frontend/src/components/common/DataTable.vue`
- `frontend/src/components/common/__tests__/DataTable.spec.ts`
- `frontend/src/composables/__tests__/useSwipeSelect.spec.ts`
- `frontend/src/composables/useSwipeSelect.ts`
- `frontend/src/i18n/locales/en/admin/accounts.ts`
- `frontend/src/i18n/locales/zh/admin/accounts.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/utils/__tests__/formatDateLocalInput.spec.ts`
- `frontend/src/utils/format.ts`
- `frontend/src/views/KeyUsageView.vue`
- `frontend/src/views/__tests__/KeyUsageView.spec.ts`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/user/DashboardView.vue`

**验证：**

```bash
TODO: 填写验证命令
```

**同步官方后的复查：**

- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。

### 2026-07-16: 自动记录本地改动

**自动记录：**

- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。
- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。

**涉及文件：**

- `frontend/src/views/admin/SubscriptionsView.vue`

**验证：**

```bash
TODO: 填写验证命令
```

**同步官方后的复查：**

- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。

### 2026-07-20: 自动记录本地改动

**自动记录：**

- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。
- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。

**涉及文件：**

- `README.md`
- `README_CN.md`
- `README_JA.md`
- `assets/logo.svg`
- `backend/cmd/server/VERSION`
- `backend/cmd/server/main.go`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/internal/config/env_reachability_test.go`
- `backend/internal/config/image_storage_env_test.go`
- `backend/internal/handler/admin/account_codex_agent_identity_import_test.go`
- `backend/internal/handler/admin/account_codex_import.go`
- `backend/internal/handler/admin/backup_handler.go`
- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/handler/admin/setting_handler_audit.go`
- `backend/internal/handler/admin/setting_handler_stepup_switch_test.go`
- `backend/internal/handler/admin/setting_handler_update.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_test.go`
- `backend/internal/handler/api_key_handler.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/failover_loop.go`
- `backend/internal/handler/failover_loop_test.go`
- `backend/internal/handler/image_task_admin_toggle_test.go`
- `backend/internal/handler/image_task_handler.go`
- `backend/internal/handler/openai_codex_models_handler.go`
- `backend/internal/handler/openai_gateway_count_tokens.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `backend/internal/handler/openai_responses_image_intent_benchmark_test.go`
- `backend/internal/pkg/apicompat/anthropic_responses_test.go`
- `backend/internal/pkg/apicompat/anthropic_to_responses.go`
- `backend/internal/pkg/apicompat/anthropic_to_responses_response.go`
- `backend/internal/pkg/apicompat/chatcompletions_anthropic_bridge.go`
- `backend/internal/pkg/apicompat/chatcompletions_anthropic_bridge_test.go`
- `backend/internal/pkg/apicompat/responses_anthropic_cache_creation_test.go`
- `backend/internal/pkg/apicompat/responses_to_anthropic.go`
- `backend/internal/pkg/apicompat/types.go`
- `backend/internal/pkg/ip/ip.go`
- `backend/internal/pkg/ip/ip_test.go`
- `backend/internal/pkg/xai/quota.go`
- `backend/internal/pkg/xai/quota_test.go`
- `backend/internal/repository/github_release_service.go`
- `backend/internal/repository/github_release_service_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/securityaudit/prompt_config.go`
- `backend/internal/securityaudit/prompt_config_store.go`
- `backend/internal/securityaudit/prompt_config_test.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/http.go`
- `backend/internal/server/http_ingress_test.go`
- `backend/internal/server/middleware/api_key_auth.go`
- `backend/internal/server/middleware/api_key_auth_test.go`
- `backend/internal/server/middleware/middleware.go`
- `backend/internal/server/middleware/session_binding.go`
- `backend/internal/server/middleware/session_binding_test.go`
- `backend/internal/server/router.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/routes/gateway_test.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/service/account_usage_service.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/admin_service_proxy_quality_test.go`
- `backend/internal/service/api_key_service.go`
- `backend/internal/service/backup_service.go`
- `backend/internal/service/backup_service_test.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/gateway_forward_as_chat_completions.go`
- `backend/internal/service/gateway_forward_as_responses.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/gemini_chat_completions_compat_service.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/service/grok_media_content_test.go`
- `backend/internal/service/grok_quota_fetcher.go`
- `backend/internal/service/grok_quota_fetcher_test.go`
- `backend/internal/service/grok_quota_service.go`
- `backend/internal/service/grok_quota_service_test.go`
- `backend/internal/service/grok_token_provider.go`
- `backend/internal/service/grok_token_provider_test.go`
- `backend/internal/service/image_storage_settings.go`
- `backend/internal/service/image_storage_settings_test.go`
- `backend/internal/service/image_task.go`
- `backend/internal/service/notification_email_service.go`
- `backend/internal/service/notification_email_service_test.go`
- `backend/internal/service/openai_codex_models_service.go`
- `backend/internal/service/openai_codex_models_service_test.go`
- `backend/internal/service/openai_codex_transform.go`
- `backend/internal/service/openai_codex_transform_test.go`
- `backend/internal/service/openai_compat_model_test.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/openai_gateway_chat_completions_test.go`
- `backend/internal/service/openai_gateway_count_tokens.go`
- `backend/internal/service/openai_gateway_count_tokens_test.go`
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/openai_gateway_grok_cache.go`
- `backend/internal/service/openai_gateway_grok_cache_test.go`
- `backend/internal/service/openai_gateway_grok_chat_bridge.go`
- `backend/internal/service/openai_gateway_grok_chat_bridge_test.go`
- `backend/internal/service/openai_gateway_grok_test.go`
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/openai_gateway_response_flush_test.go`
- `backend/internal/service/openai_gateway_response_handling.go`
- `backend/internal/service/openai_gateway_response_handling_type_test.go`
- `backend/internal/service/openai_ws_http_bridge.go`
- `backend/internal/service/openai_ws_http_bridge_test.go`
- `backend/internal/service/ops_scheduled_report_service.go`
- `backend/internal/service/ops_scheduled_report_service_test.go`
- `backend/internal/service/setting_parse.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/service/setting_service_update_test.go`
- `backend/internal/service/setting_update.go`
- `backend/internal/service/settings_view.go`
- `backend/internal/service/subscription_calculate_progress_test.go`
- `backend/internal/service/user_subscription.go`
- `backend/internal/service/user_subscription_days_remaining_test.go`
- `backend/internal/service/wire.go`
- `deploy/.env.example`
- `deploy/EDGE_SECURITY.md`
- `deploy/README.md`
- `deploy/config.example.yaml`
- `deploy/docker-compose.dev.yml`
- `deploy/docker-compose.local.yml`
- `deploy/docker-compose.standalone.yml`
- `deploy/docker-compose.yml`
- `deploy/install.sh`
- `deploy/tests/install-github-token-test.sh`
- `docs/ASYNC_IMAGE_TASKS.md`
- `frontend/src/api/admin/backup.ts`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/api/admin/system.ts`
- `frontend/src/components/account/AccountUsageCell.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/OpenAIQuotaResetCell.vue`
- `frontend/src/components/account/UpstreamBillingRateCell.vue`
- `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts`
- `frontend/src/components/account/__tests__/EditAccountModal.grokUpstream.spec.ts`
- `frontend/src/components/admin/user/UserBalanceModal.vue`
- `frontend/src/components/channels/AvailableChannelsTable.vue`
- `frontend/src/components/channels/__tests__/AvailableChannelsTable.spec.ts`
- `frontend/src/components/charts/EndpointDistributionChart.vue`
- `frontend/src/components/charts/GroupDistributionChart.vue`
- `frontend/src/components/charts/ModelDistributionChart.vue`
- `frontend/src/components/charts/UserBreakdownSubTable.vue`
- `frontend/src/components/common/AutoRefreshButton.vue`
- `frontend/src/components/layout/AppHeader.vue`
- `frontend/src/components/layout/AuthLayout.vue`
- `frontend/src/components/payment/PaymentProviderDialog.vue`
- `frontend/src/components/payment/SubscriptionPlanCard.vue`
- `frontend/src/components/payment/__tests__/PaymentProviderDialog.spec.ts`
- `frontend/src/components/payment/__tests__/SubscriptionPlanCard.spec.ts`
- `frontend/src/components/user/dashboard/UserDashboardCharts.vue`
- `frontend/src/i18n/locales/en/admin/accounts.ts`
- `frontend/src/i18n/locales/en/admin/overview.ts`
- `frontend/src/i18n/locales/en/admin/settings.ts`
- `frontend/src/i18n/locales/en/batchImage.ts`
- `frontend/src/i18n/locales/en/common.ts`
- `frontend/src/i18n/locales/en/index.ts`
- `frontend/src/i18n/locales/zh/admin/accounts.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/i18n/locales/zh/admin/settings.ts`
- `frontend/src/i18n/locales/zh/batchImage.ts`
- `frontend/src/i18n/locales/zh/common.ts`
- `frontend/src/i18n/locales/zh/index.ts`
- `frontend/src/types/index.ts`
- `frontend/src/utils/__tests__/branding.spec.ts`
- `frontend/src/utils/__tests__/formatDateTimeToMinute.spec.ts`
- `frontend/src/utils/format.ts`
- `frontend/src/views/KeyUsageView.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/BackupView.vue`
- `frontend/src/views/admin/ProxiesView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/SubscriptionsView.vue`
- `frontend/src/views/admin/__tests__/SettingsView.spec.ts`
- `frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`
- `frontend/src/views/admin/orders/AdminPaymentPlansView.vue`
- `frontend/src/views/admin/orders/__tests__/AdminPaymentPlansView.spec.ts`
- `frontend/src/views/admin/settings/EmailTemplateEditor.vue`
- `frontend/src/views/public/LegalDocumentView.vue`
- `frontend/src/views/user/BatchImageGuideView.vue`
- `frontend/src/views/user/CustomPageView.vue`
- `frontend/src/views/user/SubscriptionsView.vue`

**验证：**

```bash
TODO: 填写验证命令
```

**同步官方后的复查：**

- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。

### 2026-07-21: 自动记录本地改动

**自动记录：**

- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。
- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。

**涉及文件：**

- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`

**验证：**

```bash
TODO: 填写验证命令
```

**同步官方后的复查：**

- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。

### 2026-07-23: 自动记录本地改动

**自动记录：**

- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。
- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。

**涉及文件：**

- `backend/ent/group.go`
- `backend/ent/group/group.go`
- `backend/ent/group/where.go`
- `backend/ent/group_create.go`
- `backend/ent/group_update.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/schema/group.go`
- `backend/ent/schema/user_subscription.go`
- `backend/ent/usersubscription.go`
- `backend/ent/usersubscription/usersubscription.go`
- `backend/ent/usersubscription/where.go`
- `backend/ent/usersubscription_create.go`
- `backend/ent/usersubscription_update.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/server/http.go`
- `backend/internal/service/openai_gateway_grok_cache.go`

**验证：**

```bash
TODO: 填写验证命令
```

**同步官方后的复查：**

- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。

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
