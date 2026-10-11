# 订单订阅同步通知

订单管理中的“同步到主环境 / 取消同步”仅保存通知，不复制用户、订阅或 API Token，不执行 shell，不重启同步服务。
仅通过 JWT 登录的 `macseek@upit.top` 管理员可读写通知；管理员 API Key 不可代替该用户操作。

- `GET /api/v1/admin/payment/subscription-sync`：读取用户级最新意愿。
- `POST /api/v1/admin/payment/orders/:id/subscription-sync`：JSON `{"enabled":true}` 或 `{"enabled":false}`。
- 开启须为已完成订阅订单，且该用户至少有一个当前有效订阅；取消可用于任意订阅订单。
- 同一用户的不同订单共享一个同步开关；重复点击覆盖最新意愿，重启后保留。

## a1-user-sync 消费约定

服务使用已有源数据库连接读取 `settings`：

```sql
SELECT key, value FROM settings WHERE key LIKE 'a1_user_sync.request.%';
```

key 为 `a1_user_sync.request.<源用户ID>`，value 示例：

```json
{"user_id":21,"order_id":100,"enabled":true,"requested_by":1,"updated_at":"2026-10-11T00:00:00Z"}
```

服务按用户读取最新值，`enabled=true` 表示同步该用户及相关订阅、API Token 到主环境；`false` 表示从后续用户、订阅、Token 同步范围中排除，优先于旧配置名单。不删除主环境已有数据。实际映射、复制、重试、结果记录由服务负责；执行前须再次检查最新意愿，避免已取消的旧请求继续运行。不要消费后删除记录，否则会丢失取消状态。

## 已接入的消费实现

源码在 `services/a1-user-sync-service`，每 5 秒检查通知，失败保留待重试。成功消费后写入 `a1_user_sync.result.<源用户ID>`，其中 `updated_at` 和 `enabled` 对应已执行的最新请求。“已提交通知”仅表示请求已保存，不等于消费成功。

通知消费和原有定时同步共用数据库 advisory lock；复制前对请求行加共享锁，取消操作会等待正在执行的复制完成。取消接口返回后，旧配置名单也不能再次复制该用户。不删除主环境已有用户、订阅和 Token。

用户、有效订阅和 API Token 在主库同一事务中写入。分组以名称和平台匹配，缺失或有歧义时失败重试，不按不同环境的分组 ID 直接关联。Token 冲突时不能覆盖其他用户的 Token；源端已删除的 Token 或绑定已删除分组的历史 Token，只撤销主环境同用户已有记录，不新建不可用记录。此处同步 API Token 及其配额字段，不复制请求日志。

## 构建、部署和验证

- `bash deploy/a1-sync-build.sh`：本地测试、前后端构建和 Linux 服务交叉编译；`--service-only` 仅构建同步服务。
- `bash deploy/a1-sync-release.sh --apply`：上传本地产物，部署 A1 后端和同步服务；`--skip-build` 使用已构建产物。VPS 不构建镜像。
- `bash deploy/a1-sync-pages.sh`：发布 A1 Cloudflare Pages 前端。
- `bash deploy/a1-sync-verify.sh`：以临时用户、订阅、Token 和订单验证真实管理员 API、消费、取消优先级、重新启用和 Token 撤销，并清理测试数据。

VPS 版本产物及配置备份位于 `/opt/a1-sync-release/<版本时间>/`。安装脚本保留原镜像，通过只读挂载替换二进制，并进行健康检查；失败恢复配置。旧 `a1-subscription-sync.timer` 被停用，订阅同步统一交由 `a1-user-sync`，避免旧服务绕过取消状态。
