# 订阅管理同步通知

订阅管理中的“同步到主环境 / 取消同步”保存用户级通知，由 VPS 的 a1-user-sync 消费执行；页面本身不复制数据或重启服务。
仅通过 JWT 登录的 `macseek@upit.top` 管理员可读写通知；管理员 API Key 不可代替该用户操作。

- `GET /api/v1/admin/payment/subscription-sync`：读取用户级最新意愿。
- `POST /api/v1/admin/payment/orders/:id/subscription-sync`：JSON `{"enabled":true}` 或 `{"enabled":false}`。
- `POST /api/v1/admin/payment/subscriptions/:id/subscription-sync`：订阅管理入口，使用订阅 ID；开启要求该订阅当前有效，取消支持失效或撤销的订阅。手动分配的订阅也支持，通知中的 `order_id` 为 `0`。旧订单接口保留兼容。
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

2026-10-11 检查确认 VPS 的 `a1-user-sync.service` active、容器 running，运行二进制包含 `a1_user_sync.request.` 消费键；日志已记录 `notification synced user_id=81 subscriptions=1` 和 `notification cancelled user_id=81`。服务已接入消费逻辑。“已提交通知”仍只表示通知保存成功，不代表本次请求已执行完成。

2026-10-11 修复订阅 ID 同步接口的线上 404：a1 的旧程序挂载覆盖了已部署镜像，运行的 `847e4dad4-a1sync` 仅包含订单入口。移除旧挂载并重建 a1 应用后实际运行 `50119f2e2`；`POST /api/v1/admin/payment/subscriptions/595/subscription-sync` 在未登录时返回正常 401，容器 healthy。未代用户提交取消通知。
