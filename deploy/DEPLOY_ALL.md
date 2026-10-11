# 全量发布

2026-10-11 21:16 已发布订阅执行状态修复：三个活动后端均为 `sub2api:subapi-9fddec9ff-all-20261011205921` 且 healthy；Pages a1 `12f86d50`、3dbooks `ade8039b`、main `9cf57ccc`；消费者二进制 `/opt/a1-sync-release/20261011211529/a1-user-sync`。`bash deploy/verify-subscription-sync.sh` 验证主环境和 3dbooks 各 26 条映射回执与 a1 一致，用户 24/41/71 的取消请求已执行。Edge 核验 a1 #595 显示未同步且没有取消按钮，#734 显示已同步且只有取消按钮。

上传后中断且未切换服务时，可在同一份已验证源码下使用 `PREBUILT_IMAGE=<原镜像> UPLOADED_RELEASE=<原时间戳> npm run deploy` 复用本地镜像与远端完整分块；压缩包校验失败会在替换服务前停止。

订阅同步状态修复发布还会调用 `deploy/a1-sync-release.sh --apply`：在本地测试/编译同步消费者，上传并替换现有服务的二进制挂载。消费者读取真实执行回执，按 `lower(trim(email))` 将状态映射到主环境与 3dbooks，并转发这些环境的新请求回 a1。未找到对应用户的环境不创建用户。旧配置用户只在实际复制成功后补回执；取消不删除已复制的数据。test 保持退役。

回归检查：`bash deploy/check-subscription-sync.sh`。脚本检测到本机 Go 1.26 时会使用 jsonv2 和临时 modfile 保持可运行，不改仓库版本声明；Go 1.27 则按仓库版本直接构建。

在仓库根目录执行 `npm run deploy`，或先用 `npm run deploy -- --dry-run` 查看目标；`npm run deploy:check` 检查脚本。

脚本先在本机验证 Pages 测试和后端 admin/server 包，构建前端、Linux amd64 Go 二进制和 Docker 镜像，再分块上传至 SSH 别名 `51tokens`。VPS 只加载镜像并启动应用。前端 a1 开启 CPU 面板，main/3dbooks 关闭；各环境分别注入公开 settings。

发布顺序为 a1 → 3dbooks → main，每套后端和 Pages 健康通过后才进入下一套。端口分别为 8082、8083、8081。test 已退役，不得重新创建其服务或 Pages 项目；旧 `local-gzip-binary-deploy.sh --deploy` 已禁用。

每套使用自动加载的 `docker-compose.override.yml` 指定应用镜像和重启策略，原有 Compose、数据库和 Redis 不重建。既有 override 会备份为 `.bak-<时间戳>`；test Caddy 备份在 `/opt/cf-origin-ssl/Caddyfile.bak-deploy-<时间戳>`。后续手动 Compose 操作也会加载该 override。

发布前检查应用不能挂载 `/app/sub2api`，否则挂载文件会覆盖新镜像的程序。2026-10-11 a1 的订阅同步 POST 返回 404，原因是旧 `/opt/a1-sync-release/20261011015401/sub2api` 挂载仍在；通过 `bash deploy/fix-a1-binary-mount.sh` 备份并移除后，a1 实际运行镜像中的 `50119f2e2`，健康检查通过，公网订阅同步接口未登录响应为 401。备份为 `/opt/sub2api-ap1-deploy/docker-compose.yml.bak-binary-mount-20261011202450`。修复脚本仅适用于该旧挂载，重复运行会拒绝修改。

需要本机 Node/npm、Go、Docker buildx（本地 unix socket）、SSH、Python3，以及已登录且有三套 Pages 发布权限的 Wrangler。脚本通过 npx 固定使用 pnpm 9.15.9，并以冻结锁文件安装依赖，防止新版 pnpm 忽略安全覆盖配置。默认使用 `subapi` Pages 生产分支。失败即停止，已发布环境保留新版本，不自动回滚数据库迁移。

2026-10-11 实际发布成功：镜像 `sub2api:subapi-847e4dad4-all-20261011102511`；Pages 部署 test `3e14318b`、a1 `9496907c`、3dbooks `06dee190`、main `e4eed946`。

回退应用时恢复旧 override（首次部署则移走新增 override），在该目录执行 `docker compose up -d --no-deps <应用服务>`；Pages 使用 Cloudflare 旧部署回滚。回退 test 入口时恢复对应 Caddy 备份并 validate/reload。不要回改数据库迁移。
