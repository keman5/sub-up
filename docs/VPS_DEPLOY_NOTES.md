# VPS 部署与更新排障记录

本文记录将当前 `sub2api` 服务部署到 VPS 时遇到的问题、判断方法和可复用处理方案。当前线上以后端容器 `sub2api`、`sub2api-ap1`、`sub2api-test` 和 Cloudflare Pages 前台为准。

> 注意：不要把 `SQL_DSN`、`SESSION_SECRET`、`CRYPTO_SECRET`、Redis 密码等环境变量写入文档、聊天记录或公开日志。排障时只确认变量是否存在，避免展开具体值。

***

## 一、部署前检查

### 0. 统一使用 SSH 别名，避免在仓库中暴露服务器地址

推荐在本机 `~/.ssh/config` 中维护别名，例如：

```bash
ssh 51tokens
```

后续文档里的 `ssh`、`scp` 和部署脚本示例统一使用 `51tokens`，不要把真实 IP、私钥路径等敏感连接信息写进仓库。

### 1. 确认本地代码已经提交并推送

如果要让 VPS 直接从 GitHub 拉代码构建，必须先确认本地提交已经推送到 `origin/main`：

```bash
git status -sb
git log -1 --oneline
git ls-remote origin refs/heads/main
```

曾遇到的问题：

```text
fatal: could not read Username for 'https://github.com': Device not configured
```

原因是本机没有 GitHub HTTPS 凭据。处理方式：

- 配置 GitHub PAT 或 SSH key 后重新 `git push origin main`
- 或者不用 VPS 从 GitHub 拉取，改用 `rsync` 把本地已验证代码同步到 VPS 构建目录

### 2. 确认线上容器的启动参数

如果 VPS 上已有旧容器，不要直接覆盖。先记录当前容器的镜像、端口、挂载、网络和重启策略：

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
docker inspect sub2api --format '{{json .Mounts}}'
docker inspect sub2api --format '{{json .HostConfig.PortBindings}}'
docker inspect sub2api --format '{{json .NetworkSettings.Networks}}'
docker inspect sub2api --format '{{.HostConfig.RestartPolicy.Name}}'
```

当前线上后端容器：

- 主环境：`sub2api`
- a1：`sub2api-ap1`
- test：`sub2api-test`
- 共享依赖：`sub2api-postgres`、`sub2api-redis`

***

## 二、网络不稳定时的可用替代部署方案

当 VPS 无法稳定访问 npm、apt、Go proxy 时，可以采用“本机构建前端和后端，VPS 只打运行镜像”的方式。

### 1. sub2api 本地构建产物的 gzip 分块上传与远端解压方案

当前 sub2api 嵌入前端后的 Linux amd64 二进制约 86 MB。VPS 出口和本地到 VPS 链路不稳定时，不要直接 `scp` 原始二进制，也不要在 VPS 上重新拉取 npm / Go / apt 依赖。gzip 分块上传流程是：

1. 本机交叉编译 Linux amd64 后端。
2. 本地使用 `gzip -c` 生成 `.gz` 产物。
3. 将 `.gz` 切成 1 MB 小块，通过 SSH 标准输入逐块写入远端 `/tmp/sub2api-upload-<timestamp>`。
4. 远端按文件名顺序拼接为临时 `.gz`，执行 `gzip -t` 校验。
5. 远端解压到临时可执行文件。
6. `chmod +x` 后原子 `mv` 为 `/opt/sub2api-runtime-build/sub2api`。
7. 基于上一版运行镜像只替换 `/app/sub2api`，再按 test、ap1、primary 顺序滚动。

2026-06-14 起，`ai.upit.top`、`a1.upit.top`、`test.upit.top` 三套前台静态资源都按 Cloudflare Pages 发布。以后如果只是后端/API 变更，VPS 发布不需要重新构建前台，使用 `--skip-frontend-build`。前台改动应走 Cloudflare Pages 发布流程：

```bash
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

pnpm dlx wrangler pages deploy /tmp/sub2api-pages-main --project-name sub2api-frontend-main --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-a1 --project-name sub2api-frontend-a1 --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-test --project-name sub2api-frontend-test --branch subapi
```

注意：Pages 只托管静态 HTML，不会执行 VPS 内嵌前台的 Go 注入逻辑。部署前必须把对应环境的 `/api/v1/settings/public` 公开 `data` 字段写入 `backend/internal/web/dist/index.html` 的 `window.__APP_CONFIG__`，否则站点标题、logo、Turnstile 开关、模型 API base URL 等线上配置会退回构建默认值。不要把 `.env`、后台配置或密钥写入静态文件。

多环境前台发布边界：

- 运行时 public settings 可以在发布前注入到各环境自己的 `index.html`。
- 构建期配置如果会进入 Vite/JS/CSS bundle，必须按环境分别打包，不要用一套 bundle 覆盖多套服务。
- 当前三套正式前台使用独立 Cloudflare Pages 静态产物，不依赖 Worker 动态 HTML 注入。项目名分别是 `sub2api-frontend-main`、`sub2api-frontend-a1`、`sub2api-frontend-test`。
- 只有在确认差异全是运行时公开配置时，才可以复用同一次 build 的 assets，复制产物目录后分别 inject；只要差异包含构建期变量，就必须重新 build。

注意：当前 VPS 后端二进制仍使用 `go build -tags embed`，会把 `backend/internal/web/dist` 中已有的前端产物嵌入作为回滚和源站兜底。不要现在默认去掉 `-tags embed`；等三套前台 Pages 迁移稳定并确认不再需要 VPS 内嵌前台兜底后，再评估增加 no-embed 后端发布模式。

说明：2026-06-01 线上重部署时，单条 gzip 管道和单文件 scp 在当前 VPS SSH 链路上都出现过中途停住；脚本已改为小块短连接上传，并带 5 次退避重试。

仓库已提供脚本：

```bash
# 只打印计划，不改远端
deploy/local-gzip-binary-deploy.sh

# 使用已有本地产物做 dry-run
deploy/local-gzip-binary-deploy.sh --skip-frontend-build --skip-backend-build

# 后端/API 常规发布：跳过前台构建，gzip 上传、远端解压、打镜像，但不重启服务
deploy/local-gzip-binary-deploy.sh --apply --skip-frontend-build

# 后端/API 常规发布：跳过前台构建，并滚动部署 test + ap1 + primary
deploy/local-gzip-binary-deploy.sh --apply --deploy --skip-frontend-build

# 只有在需要刷新 VPS 内嵌前台兜底时，才省略 --skip-frontend-build
deploy/local-gzip-binary-deploy.sh --apply --deploy
```

可用环境变量或参数覆盖目标：

```bash
HOST=51tokens \
BASE_IMAGE=sub2api:subapi-6b800b77-logo-dbterms-20260530222347 \
IMAGE_TAG=sub2api:subapi-$(git rev-parse --short HEAD)-gzip-$(date +%Y%m%d%H%M%S) \
deploy/local-gzip-binary-deploy.sh --apply --deploy --skip-frontend-build
```

脚本关键安全点：

- 默认 dry-run；必须显式 `--apply` 才会改远端。
- gzip 上传后远端先拼接并 `gzip -t` 校验，再解压。
- 默认分块大小为 `UPLOAD_CHUNK_SIZE=1m`，可按链路情况覆盖。
- 解压到 `sub2api.<timestamp>.tmp`，校验权限后再 `mv` 覆盖正式二进制。
- compose/env 更新前会备份为 `docker-compose.yml.bak-gzip-<timestamp>` / `.env.bak-gzip-<timestamp>`。
- `--deploy` 默认更新三套线上后端环境：先更新 `sub2api-test`，确认 healthy 后更新 `sub2api-ap1`，最后更新 `sub2api`。
- primary/ap1 通过 compose `image:` 切换镜像；test 通过 `/opt/sub2api-test-deploy/.env` 的 `IMAGE_TAG` 切换镜像。
- 最后验证 test、ap1、primary 和公开 `/health`。

### 3.2 动态模型路由上线核对

动态模型路由的代码默认配置是关闭的，线上必须通过 compose 环境变量显式开启，否则即使账号 `extra` 中已有 Codex 5h / 7d 用量快照，请求也仍会按调用方原始模型转发。

生产 compose 需要保留以下环境变量：

```yaml
- GATEWAY_MODEL_ROUTER_ENABLED=${GATEWAY_MODEL_ROUTER_ENABLED:-false}
- GATEWAY_MODEL_ROUTER_OAUTH_MODE=${GATEWAY_MODEL_ROUTER_OAUTH_MODE:-passthrough}
- GATEWAY_MODEL_ROUTER_DEFAULT_MODEL=${GATEWAY_MODEL_ROUTER_DEFAULT_MODEL:-gpt-5.3-codex-spark}
- GATEWAY_MODEL_ROUTER_BALANCED_MODEL=${GATEWAY_MODEL_ROUTER_BALANCED_MODEL:-gpt-5.4}
- GATEWAY_MODEL_ROUTER_PREMIUM_MODEL=${GATEWAY_MODEL_ROUTER_PREMIUM_MODEL:-gpt-5.5}
- GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_CHARS=${GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_CHARS:-12000}
- GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_ITEMS=${GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_ITEMS:-20}
- GATEWAY_MODEL_ROUTER_PRESSURE_LOW_REMAINING_PERCENT=${GATEWAY_MODEL_ROUTER_PRESSURE_LOW_REMAINING_PERCENT:-40}
- GATEWAY_MODEL_ROUTER_PRESSURE_MEDIUM_REMAINING_PERCENT=${GATEWAY_MODEL_ROUTER_PRESSURE_MEDIUM_REMAINING_PERCENT:-70}
- GATEWAY_MODEL_ROUTER_IMAGE_OR_VISION_FORCE_PREMIUM=${GATEWAY_MODEL_ROUTER_IMAGE_OR_VISION_FORCE_PREMIUM:-false}
```

`GATEWAY_MODEL_ROUTER_OAUTH_MODE` 默认 `passthrough`，OpenAI OAuth / Codex Pro 账号保持调用方原始模型。动态模型路由建议默认关闭：除了环境变量 `GATEWAY_MODEL_ROUTER_ENABLED` 以外，实际放量还受后台全局设置里的 `OpenAI 实验调度策略` 开关控制；只有两者都开启时，自适应路由才真正生效。若要让 OAuth 账号参与 5.3 Spark / 5.4 / 5.5 自适应路由，先仅在 ap2/a2 灰度设置为 `adaptive_codex` 并在后台显式打开开关；ap1 和主环境保持 `passthrough` 或关闭动态路由，等 a2 验证稳定后再推进。

路由分层规则：普通文本优先 `GATEWAY_MODEL_ROUTER_DEFAULT_MODEL`，中等复杂文本走 `GATEWAY_MODEL_ROUTER_BALANCED_MODEL`，超过 `GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_*` 的大请求、图片/视觉或连续能力失败升级到 `GATEWAY_MODEL_ROUTER_PREMIUM_MODEL`。当账号 5h/7d 剩余额度进入压力区间时，会优先压回 economy/balanced 以节省额度。

对外隐藏规则：动态路由只改变内部上游 `upstream_model`；客户端响应体、流式 SSE `model` 字段和普通用户用量记录继续显示调用方请求模型。真实上游模型仅在管理员用量/运维日志中保留，用于排障和成本核对。

上线后检查：

```bash
ssh 51tokens 'docker exec sub2api env | grep GATEWAY_MODEL_ROUTER'
ssh 51tokens 'docker exec sub2api-ap1 env | grep GATEWAY_MODEL_ROUTER'
curl -fsS https://ai.upit.top/health
curl -fsS https://a1.upit.top/health
```

若用户反馈高压账号仍未切到 `gpt-5.3-codex-spark`，先看容器环境变量，再查对应 OpenAI 账号 `extra` 中的 `codex_5h_used_percent`、`codex_7d_used_percent` 和最近 `usage_logs.model/upstream_model`。

### 3.3 test / a2t 灰度环境单独重部署

`test` 不是 `sub2api-ap1`。当前线上单独存在一套灰度环境：

- compose 目录：`/opt/sub2api-test-deploy`
- compose project：`sub2api-test-deploy`
- 服务名 / 容器名：`sub2api-test`
- 本地健康检查：`http://127.0.0.1:8083/health`
- 镜像来源：`/opt/sub2api-test-deploy/.env` 中的 `IMAGE_TAG=...`

2026-07-06 起，`deploy/local-gzip-binary-deploy.sh --apply --deploy` 默认会同时滚动 `test`、`ap1` 和 `primary` 三套线上后端环境。如果只是重新部署 `test/a2t`，不要使用默认 `--deploy`，因为它会连同 `sub2api-ap1` 和 `sub2api` 一起更新。正确做法是：

1. 先用脚本仅完成“本地构建 + gzip 上传 + 远端替换 `/opt/sub2api-runtime-build/sub2api` + 打新镜像”，不要滚动主环境：

```bash
IMAGE_TAG="sub2api:subapi-<git-sha>-ap2-redeploy-$(date +%Y%m%d%H%M%S)" \
HOST=51tokens \
BASE_IMAGE="$(ssh 51tokens \"grep '^IMAGE_TAG=' /opt/sub2api-test-deploy/.env | cut -d= -f2-\")" \
deploy/local-gzip-binary-deploy.sh --apply
```

2. 然后仅修改 `test` 目录下的 `.env` 并重启 `sub2api-test`：

```bash
ssh 51tokens '
  set -eu
  cd /opt/sub2api-test-deploy
  TS=$(date +%Y%m%d%H%M%S)
  cp .env .env.bak.$TS.redeploy-ap2
  sed -i "s#^IMAGE_TAG=.*#IMAGE_TAG=sub2api:subapi-<git-sha>-ap2-redeploy-<timestamp>#\" .env
  docker compose up -d sub2api-test
  curl -fsS http://127.0.0.1:8083/health
'
```

3. 再确认容器与镜像：

```bash
ssh 51tokens '
  docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" sub2api-test
  docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep sub2api-test
'
```

2026-06-02 实测：

- 旧镜像：`sub2api:subapi-a5e4b0c6-ap2-oauth-adaptive-v2-202606021754`
- 新镜像：`sub2api:subapi-9d75fb6b-ap2-redeploy-20260602232707`
- `sub2api-test` 更新后状态为 `healthy`
- `http://127.0.0.1:8083/health` 返回 `{"status":"ok"}`

如果只想验证灰度 OAuth/Codex 自适应路由，不要动 `sub2api-ap1` 或 `sub2api`，保持 `test` 独立切换即可。

底层等价命令示例：

```bash
gzip -c /tmp/sub2api-build-output/sub2api | ssh 51tokens '
  set -eu
  mkdir -p /opt/sub2api-runtime-build
  gz=/opt/sub2api-runtime-build/sub2api.$(date +%Y%m%d%H%M%S).gz
  tmp=${gz%.gz}.tmp
  cat > "$gz"
  gzip -t "$gz"
  gzip -dc "$gz" > "$tmp"
  chmod +x "$tmp"
  mv "$tmp" /opt/sub2api-runtime-build/sub2api
  rm -f "$gz"
  ls -lh /opt/sub2api-runtime-build/sub2api
  file /opt/sub2api-runtime-build/sub2api
'
```

### 3.4 2026-06-11 线上共享数据层、Grafana 移除与 sysstat 监控

当前 1 vCPU / 约 2 GiB 内存 VPS 不再为 `ap1`、`test` 各自运行独立 PostgreSQL 和 Redis。三套 sub2api 应用共享主 compose 的 PostgreSQL / Redis，以减少常驻容器数量、healthcheck 和后台连接池开销。

当前线上拓扑：

| 环境 | compose 目录 | 应用容器 | 本机端口 | PostgreSQL database | Redis DB | 说明 |
| --- | --- | --- | --- | --- | --- | --- |
| primary | `/opt/sub2api-deploy` | `sub2api` | `127.0.0.1:8081` | `sub2api` | `0` | 持有共享 `sub2api-postgres` / `sub2api-redis` |
| ap1 / a1 | `/opt/sub2api-ap1-deploy` | `sub2api-ap1` | `127.0.0.1:8082` | `sub2api_ap1` | `1` | compose 只保留应用容器，接入共享网络 |
| test / a2t | `/opt/sub2api-test-deploy` | `sub2api-test` | `127.0.0.1:8083` | `sub2api_test` | `2` | compose 只保留应用容器，接入共享网络 |

共享基础设施：

- `sub2api-postgres`：唯一 PostgreSQL 容器。
- `sub2api-redis`：唯一 Redis 容器。
- `sub2api-deploy_sub2api-network`：共享 Docker network；`ap1` / `test` compose 通过 `external: true` 接入。

已废弃并移除的容器：

- `sub2api-ap1-postgres`
- `sub2api-ap1-redis`
- `sub2api-test-postgres`
- `sub2api-test-redis`
- `grafana`
- `pdc-agent-sub2api`

> 注意：旧数据目录、卷和迁移备份不要立即删除。2026-06-11 迁移备份位于 `/root/sub2api-migration-backup-20260611-200522`，包含 compose / `.env` 备份和 PostgreSQL dump。

`ap1` / `test` compose 关键要求：

```yaml
networks:
  shared-sub2api-network:
    external: true
    name: sub2api-deploy_sub2api-network
```

`ap1` 应用环境变量必须指向共享数据层：

```yaml
- DATABASE_HOST=sub2api-postgres
- DATABASE_DBNAME=sub2api_ap1
- REDIS_HOST=sub2api-redis
- REDIS_DB=1
```

`test` 应用环境变量必须指向共享数据层：

```yaml
- DATABASE_HOST=sub2api-postgres
- DATABASE_DBNAME=sub2api_test
- REDIS_HOST=sub2api-redis
- REDIS_DB=2
```

以后重部署 `ap1` / `test` 时，不要再把 `postgres` / `redis` 服务加回各自 compose；否则会重新引入三套数据库和缓存，1 核 VPS 很容易再次 CPU 100%。

迁移或恢复时的基本顺序：

1. 备份 `/opt/sub2api-deploy`、`/opt/sub2api-ap1-deploy`、`/opt/sub2api-test-deploy` 中的 `docker-compose.yml` 和 `.env`。
2. 分别对旧库执行 `pg_dump -Fc`，不要在聊天记录或文档展开数据库密码。
3. 短暂停止 `sub2api-ap1` 和 `sub2api-test` 应用容器，避免迁移期间继续写旧库。
4. 在 `sub2api-postgres` 中创建或重建 `sub2api_ap1`、`sub2api_test`，再用 `pg_restore --no-owner` 导入。
5. 将旧 `ap1` Redis DB0 迁到主 `sub2api-redis` DB1，将旧 test Redis DB0 迁到主 DB2；Redis 没有数据库名称，文档中统一称为“test 使用 Redis DB2”。
6. 修改 `ap1` / `test` compose，接入 `sub2api-deploy_sub2api-network`，并更新 `DATABASE_*` / `REDIS_*` 环境变量。
7. `docker compose up -d` 重建 `sub2api-ap1`、`sub2api-test`。
8. 健康检查通过后再移除旧 `ap1/test` PostgreSQL / Redis orphan 容器和 Grafana / PDC 容器。

验证命令：

```bash
ssh 51tokens '
  docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"
  docker ps -a --format "{{.Names}}" | egrep "grafana|pdc-agent|sub2api-ap1-postgres|sub2api-ap1-redis|sub2api-test-postgres|sub2api-test-redis" || echo none

  for port in 8081 8082 8083; do
    echo -n "$port "
    curl -fsS --max-time 8 "http://127.0.0.1:$port/health"
    echo
  done

  docker exec sub2api-test sh -lc "env | grep -E '\''^(DATABASE_DBNAME|REDIS_DB)='\'' | sort"

  for db in sub2api sub2api_ap1 sub2api_test; do
    echo -n "$db "
    docker exec sub2api-postgres psql -U sub2api -d "$db" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema = '\''public'\'';"
  done

  for n in 0 1 2; do
    echo -n "redis-db$n "
    docker exec sub2api-redis sh -lc "env -u REDISCLI_AUTH redis-cli -n $n DBSIZE"
  done
'
```

2026-06-11 实测结果：

- `8081`、`8082`、`8083` 的 `/health` 均返回 `{"status":"ok"}`。
- `sub2api-test` 环境变量显示 `DATABASE_DBNAME=sub2api_test`、`REDIS_DB=2`。
- `sub2api`、`sub2api-ap1`、`sub2api-test`、`sub2api-postgres`、`sub2api-redis` 均为 `healthy`。
- `sub2api_ap1`、`sub2api_test` 各恢复出 86 张 public 表。
- 稳定后 `sar -u 1 5` 平均 CPU idle 约 58%，比迁移前持续 idle 0% 明显下降。

#### 3.4.1 2026-06-13 连接池按环境分档

三套实例都是线上环境，但流量和用途不同。1 vCPU / 约 2 GiB 内存 VPS 上不要继续使用默认大连接池，否则三套应用会长期保留过多 PostgreSQL / Redis 连接，增加共享数据层常驻开销。

当前分档：

| 环境 | 定位 | `DATABASE_MAX_OPEN_CONNS` | `DATABASE_MAX_IDLE_CONNS` | `REDIS_POOL_SIZE` | `REDIS_MIN_IDLE_CONNS` |
| --- | --- | ---: | ---: | ---: | ---: |
| primary | 线上环境 | `12` | `2` | `128` | `2` |
| ap1 / a1 | 线上环境 | `12` | `2` | `128` | `2` |
| test / a2t | 内测环境 | `4` | `1` | `32` | `1` |

调整和回滚前必须先备份 `.env`：

```bash
ssh 51tokens '
  stamp=$(date +%Y%m%d%H%M%S)
  for d in /opt/sub2api-deploy /opt/sub2api-ap1-deploy /opt/sub2api-test-deploy; do
    cp "$d/.env" "$d/.env.bak-pool-tuning-$stamp"
  done
'
```

修改后按低风险顺序滚动重启：先内测 `test`，再 `ap1`，最后 `primary`。每个容器健康后再继续下一个：

```bash
ssh 51tokens '
  cd /opt/sub2api-test-deploy && docker compose up -d --no-deps sub2api-test
  curl -fsS http://127.0.0.1:8083/health
  docker inspect -f "{{.State.Health.Status}}" sub2api-test

  cd /opt/sub2api-ap1-deploy && docker compose up -d --no-deps sub2api
  curl -fsS http://127.0.0.1:8082/health
  docker inspect -f "{{.State.Health.Status}}" sub2api-ap1

  cd /opt/sub2api-deploy && docker compose up -d --no-deps sub2api
  curl -fsS http://127.0.0.1:8081/health
  docker inspect -f "{{.State.Health.Status}}" sub2api
'
```

验证运行时环境变量和 PostgreSQL idle 连接数：

```bash
ssh 51tokens '
  for c in sub2api sub2api-ap1 sub2api-test; do
    echo "### $c"
    docker exec "$c" sh -c "env | grep -E \"^(DATABASE_MAX_OPEN_CONNS|DATABASE_MAX_IDLE_CONNS|REDIS_POOL_SIZE|REDIS_MIN_IDLE_CONNS)=\" | sort"
  done

  docker exec sub2api-postgres psql -U sub2api -d postgres -Atc "
    select datname,state,count(*)
    from pg_stat_activity
    where datname in ('\''sub2api'\'','\''sub2api_ap1'\'','\''sub2api_test'\'')
    group by datname,state
    order by datname,state;
  "
'
```

2026-06-13 实测结果：

- 运行环境变量已生效：primary / ap1 为 `12/2/128/2`，test 为 `4/1/32/1`。
- PostgreSQL idle 连接从每套约 `10` 条降为：`sub2api=2`、`sub2api_ap1=2`、`sub2api_test=1`。
- `8081`、`8082`、`8083` 的 `/health` 均返回 `{"status":"ok"}`，`sub2api`、`sub2api-ap1`、`sub2api-test` 均为 `healthy`。
- 公网验证：`https://api.upit.top/health`、`https://ai.upit.top/health`、`https://a1.upit.top/health`、`https://test.upit.top/health`、`https://a2t.upit.top/health` 均返回 ok。

后续重部署或重建 compose 时，必须保留上述分档。除非升级 VPS 或明确评估并发压力，不要把三套环境恢复成 `DATABASE_MAX_OPEN_CONNS=50`、`DATABASE_MAX_IDLE_CONNS=10`、`REDIS_POOL_SIZE=512`、`REDIS_MIN_IDLE_CONNS=10`。

#### 3.4.2 snapd 移除与 sysstat 历史监控

当前 VPS 不使用 snapd / LXD。为减少后台 watchdog 和无用服务，已移除：

- `lxd` snap
- `core20` snap
- `snapd`

确认方式：

```bash
ssh 51tokens '
  if command -v snap >/dev/null 2>&1; then snap list; else echo "snap removed"; fi
  systemctl list-units "snap*" --no-pager --all || true
'
```

历史监控使用 `sysstat`，并把默认 10 分钟采样改为 1 分钟采样。关键配置：

- `/etc/default/sysstat`：`ENABLED="true"`
- `/etc/systemd/system/sysstat-collect.timer.d/override.conf`

override 内容：

```ini
[Timer]
OnCalendar=
OnCalendar=*:0/1
AccuracySec=10s
```

常用查询：

```bash
ssh 51tokens '
  sar -u
  sar -u -s 16:00:00 -e 17:30:00
  sar -r
  sar -q
  systemctl list-timers "*sysstat*" --no-pager
'
```

## 七、Cloudflare Pages 前台与 VPS 旧入口关闭

2026-06-14 起，三套正式前台域名都迁到 Cloudflare Pages：

| 前台域名 | Pages 项目 | API 回源 | 后端容器 |
| --- | --- | --- | --- |
| `ai.upit.top` | `sub2api-frontend-main` | `https://api.upit.top` | `sub2api` |
| `a1.upit.top` | `sub2api-frontend-a1` | `https://ap1.upit.top` | `sub2api-ap1` |
| `test.upit.top` | `sub2api-frontend-test` | `https://a2t.upit.top` | `sub2api-test` |

VPS 继续保留 `api.upit.top`、`ap1.upit.top`、`a2t.upit.top` 作为 API 回源。`sub2api`、`sub2api-ap1`、`sub2api-test` 后端容器仍承载登录、后台、支付回调、Turnstile secret 校验、模型网关、`/api/*`、`/health` 和 `/51Token/*` 等服务，不能因为前台迁 Pages 而停止。

当前边界：

| 域名 / 服务 | 当前职责 | 是否可关闭 |
| --- | --- | --- |
| `ai.upit.top` | Cloudflare Pages 前台域名，提供 HTML / JS / CSS | VPS 上的旧 origin 入口应关闭 |
| `a1.upit.top` | Cloudflare Pages 前台域名，提供 HTML / JS / CSS | VPS 上的旧 origin 入口应关闭 |
| `test.upit.top` | Cloudflare Pages 前台域名，提供 HTML / JS / CSS | VPS 上的旧 origin 入口应关闭 |
| `api.upit.top` | Pages Worker API 回源域名 | 不可关闭 |
| `ap1.upit.top` | Pages Worker API 回源域名 | 不可关闭 |
| `a2t.upit.top` | Pages Worker API 回源域名 | 不可关闭 |
| `sub2api` | primary 后端/API 容器，监听 `127.0.0.1:8081` | 不可关闭 |
| `sub2api-ap1` | a1 后端/API 容器，监听 `127.0.0.1:8082` | 不可关闭 |
| `sub2api-test` | test 后端/API 容器，监听 `127.0.0.1:8083` | 不可关闭 |
| `cf-origin-ssl` | VPS origin TLS / Caddy 入口 | 不可关闭 |

本次关闭的不是后端容器，而是 `/opt/cf-origin-ssl/Caddyfile` 里 `ai.upit.top`、`a1.upit.top`、`test.upit.top` 的旧前台 origin server label。Caddy 应只保留 API 回源域名及内部测试别名。

变更前：

```caddyfile
api.upit.top:443, ai.upit.top:443 {
    ...
}

ap1.upit.top:443, a1.upit.top:443 {
    ...
}

a2t.upit.top:443, test.upit.top:443 {
    ...
}
```

变更后：

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

历史 a2 备份：

```text
/opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656
```

验证命令：

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
  grep -n "ai.upit.top\|a1.upit.top\|a2t.upit.top\|api.upit.top\|ap1.upit.top\|test.upit.top" /opt/cf-origin-ssl/Caddyfile
'
```

2026-06-13 实测结果：

- `test.upit.top/login` 继续由 Cloudflare Pages 返回，并加载 `assets/index-D26Z13h8.js`。
- `test.upit.top/health`、`a2t.upit.top/health` 均返回 `{"status":"ok"}`。
- `test.upit.top/api/v1/settings/public` 返回 `code=0`。
- `sub2api-test` 为 `healthy`。
- `/opt/cf-origin-ssl/Caddyfile` 中只保留 `a2t.upit.top:443`，不再包含 `test.upit.top:443`。

回滚时，只把确实需要从 Pages 回退到 VPS 的前台域名加回 Caddy；不要移除 API 回源域名。a2 的历史回滚示例：

```bash
ssh 51tokens '
  cp /opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656 /opt/cf-origin-ssl/Caddyfile
  docker exec cf-origin-ssl caddy validate --config /etc/caddy/Caddyfile
  docker exec cf-origin-ssl caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
'
```

注意：Cloudflare Pages 迁移只适合前台静态资源。不要把 `api.upit.top`、`ap1.upit.top`、`a2t.upit.top` 等 API 回源域名切到 Pages；不要停止 `sub2api`、`sub2api-ap1`、`sub2api-test` 来“关闭前台”，这些容器仍提供 API 和网关能力。

### Pages Worker 公开接口短缓存

为了进一步降低 VPS 的公开配置请求压力，`frontend/public/_worker.js` 对以下公开只读接口启用 Cloudflare Cache API 短缓存：

| 路径 | 方法 | TTL | 回源 |
| --- | --- | ---: | --- |
| `/api/v1/settings/public` | `GET` | `60s` | `ai.upit.top` 回 `https://api.upit.top`，`a1.upit.top` 回 `https://ap1.upit.top`，`test.upit.top` 回 `https://a2t.upit.top` |
| `/api/status` | `GET` | `60s` | 同上 |
| `/api/home_page_content` | `GET` | `60s` | 同上 |

缓存状态响应头：

```text
X-Sub2API-Edge-Cache: HIT | MISS | BYPASS | UNAVAILABLE
```

安全边界：

- 仅精确匹配上述公开只读路径；不缓存 `/api/v1/auth/*`、`/api/v1/admin/*`、支付/OAuth 回调、用户资料等接口。
- 不缓存 `/51Token/*`、`/v1/*`、`/responses/*`、`/images/*` 等模型网关和长流式接口。
- 缓存回源请求会移除 `Authorization` 和 `Cookie`，防止公开配置缓存混入用户态请求头。
- 请求头包含 `Cache-Control: no-cache/no-store` 或 `Pragma: no-cache` 时会绕过边缘缓存并回源。

部署验证：

```bash
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

pnpm dlx wrangler pages deploy /tmp/sub2api-pages-main --project-name sub2api-frontend-main --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-a1 --project-name sub2api-frontend-a1 --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-test --project-name sub2api-frontend-test --branch subapi

curl -fsSL --max-time 15 https://ai.upit.top/login \
  | rg "window\\.__APP_CONFIG__|https://api.upit.top/51Token/v1|<title>"
curl -sSi --max-time 15 https://ai.upit.top/api/v1/settings/public \
  | grep -Ei 'HTTP/|x-sub2api-edge-cache|cache-control|cf-cache-status'
curl -fsS --max-time 15 https://ai.upit.top/health
curl -fsS --max-time 15 https://api.upit.top/health

curl -fsSL --max-time 15 https://a1.upit.top/login \
  | rg "window\\.__APP_CONFIG__|https://ap1.upit.top/51Token/v1|<title>"
curl -sSi --max-time 15 https://a1.upit.top/api/v1/settings/public \
  | grep -Ei 'HTTP/|x-sub2api-edge-cache|cache-control|cf-cache-status'
curl -fsS --max-time 15 https://a1.upit.top/health
curl -fsS --max-time 15 https://ap1.upit.top/health

curl -fsSL --max-time 15 https://test.upit.top/login \
  | rg "window\\.__APP_CONFIG__|https://a2t.upit.top/51Token/v1|<title>"
curl -sSi --max-time 15 https://test.upit.top/api/v1/settings/public \
  | grep -Ei 'HTTP/|x-sub2api-edge-cache|cache-control|cf-cache-status'
curl -fsS --max-time 15 https://test.upit.top/health
curl -fsS --max-time 15 https://a2t.upit.top/health
```

***

## 八、多站点 Caddy 静态资源路由

### 现象

`a1.upit.top`、`ai.upit.top` 等站点同时承载 51token 首页和 sub2api 前台时，可能出现前台静态资源 404，例如：

```text
GET https://a1.upit.top/fonts/space-grotesk-700.ttf 404
```

这种问题不一定是 sub2api 前端构建缺少资源。曾遇到的实际原因是 Caddy 先用首页静态目录处理 `/fonts/*`，但首页目录里没有这些字体文件，请求没有继续回落到 sub2api 容器。

### 判断方法

先分别验证公网、Caddy 容器静态目录和 sub2api 上游：

```bash
# 公网入口
curl -sSIL --max-time 15 https://a1.upit.top/fonts/space-grotesk-700.ttf

# 首页静态目录是否真的有该文件
find /opt/cf-origin-ssl/a1-home -maxdepth 3 -path '*/fonts/*' -type f -ls

# sub2api 上游是否能直接提供该文件
curl -sSIL --max-time 5 http://127.0.0.1:8082/fonts/space-grotesk-700.ttf
```

如果公网返回 404，但 `127.0.0.1:8082` 返回 `200`，通常是 Caddy 静态 matcher 抢先命中了错误目录。

### 推荐 Caddy 写法

首页静态资源 matcher 不要只按 path 匹配 `/fonts/*`，还应要求文件在首页静态目录中真实存在。这样首页资源存在时直接由 Caddy 提供；不存在时继续回落到后面的 sub2api `reverse_proxy`。

`ai.upit.top` 示例：

```caddyfile
@homeStatic {
    path /static/* /textures/* /fonts/* /favicon.ico /logo.png /logo.svg /robots.txt /pay-google.png /pay-apple.png /pay-card.png
    file {
        root /srv/51token-home
        try_files {path}
    }
}
handle @homeStatic {
    root * /srv/51token-home
    file_server
}
```

`a1.upit.top` 示例：

```caddyfile
@homeStatic {
    path /static/* /textures/* /fonts/* /favicon.ico /logo.png /logo.svg /robots.txt /pay-google.png /pay-apple.png /pay-card.png
    file {
        root /etc/caddy/a1-home
        try_files {path}
    }
}
handle @homeStatic {
    root * /etc/caddy/a1-home
    file_server
}
```

注意：`@homeAssetFiles` 如果已经使用 `file` matcher，可保持同样思路。多个副本新增域名时，应复制“path + file exists”组合，而不是只复制 path matcher。

### 重载与验证

修改 `/opt/cf-origin-ssl/Caddyfile` 前先备份：

```bash
cp /opt/cf-origin-ssl/Caddyfile /opt/cf-origin-ssl/Caddyfile.bak-$(date +%Y%m%d%H%M%S)
docker exec cf-origin-ssl caddy validate --config /etc/caddy/Caddyfile
docker exec cf-origin-ssl caddy reload --config /etc/caddy/Caddyfile
```

验证字体资源：

```bash
for weight in 400 500 600 700; do
  curl -sSIL --max-time 15 "https://a1.upit.top/fonts/space-grotesk-${weight}.ttf" | sed -n '1,12p'
done
```

预期返回 `200` 和 `Content-Type: font/ttf`。

### 本次线上处理记录

- 备份：`/opt/cf-origin-ssl/Caddyfile.bak-font-fallback-20260529093717`
- 修复：`ai.upit.top` 和 `a1.upit.top` 的 `@homeStatic` 增加 `file { root ... try_files {path} }` 条件。
- 重载：`docker exec cf-origin-ssl caddy validate --config /etc/caddy/Caddyfile` 后执行 `caddy reload`。
- 验证：`a1.upit.top/fonts/space-grotesk-{400,500,600,700}.ttf` 返回 `200`。

***

## 九、Turnstile 前端脚本与 CSP

### 现象

浏览器控制台可能出现：

```text
Failed to initialize Turnstile: Error: Failed to load Turnstile script
```

也可能在 Network 或 Console 里看到 Cloudflare challenge 相关的 `401`、`403`、`600010` 等信息。

### 不要本地化 Turnstile `api.js`

不要把下面的官方脚本下载到本地静态目录，也不要通过自己的反向代理缓存后再给浏览器加载：

```text
https://challenges.cloudflare.com/turnstile/v0/api.js
```

Cloudflare 官方要求 Turnstile `api.js` 必须从 `https://challenges.cloudflare.com/turnstile/v0/api.js` 精确加载。代理或缓存该文件可能导致后续动态挑战逻辑、版本更新或安全校验失败。

正确方向是检查：

- 前端是否使用官方 URL 加载脚本。
- CSP 是否允许 `https://challenges.cloudflare.com`。
- Turnstile site key 是否允许当前部署域名，例如 `a1.upit.top`。
- 浏览器环境是否拦截 Cloudflare challenge，例如扩展、VPN、代理、自动化环境或公司网络策略。

### CSP 要求

CSP 至少需要允许：

```text
script-src ... https://challenges.cloudflare.com;
frame-src ... https://challenges.cloudflare.com;
connect-src ... https:;
```

线上 sub2api 曾使用的 CSP 示例中已经包含：

```text
script-src 'self' __CSP_NONCE__ https://challenges.cloudflare.com ...
frame-src https://challenges.cloudflare.com ...
```

如果脚本加载失败，先检查响应头：

```bash
curl -sSIL --max-time 15 https://a1.upit.top/login | grep -i content-security-policy
```

### 验证方法

从本地或 VPS 验证官方脚本是否可达：

```bash
curl -sSIL --max-time 15 \
  'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'

curl -sSIL --max-time 15 \
  -e 'https://a1.upit.top/' \
  'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'
```

正常情况下会先返回 `302`，再返回 `200 application/javascript`。

如果主脚本是 `200` 且浏览器里 `window.turnstile` 已存在，但 challenge 子请求出现 `401` 或 Turnstile 控制台报 `600010`，通常不是“脚本需要本地化”的问题。Cloudflare 文档说明 Turnstile 控制台里的 `401` 可能是底层 Challenge Platform 的预期流程；`600*` 是通用 challenge failure，应重点排查浏览器环境、site key 域名、网络拦截和 Cloudflare 后台配置。

### 本次线上处理记录

- `https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad` 本地验证为 `302 -> 200`。
- `a1.upit.top/login` 响应头 CSP 已允许 `https://challenges.cloudflare.com`。
- 使用 Chrome 打开 `https://a1.upit.top/login` 时，Turnstile 主脚本返回 `200`，`window.turnstile` 存在。
- 没有将 Turnstile 脚本下载到本地，因为这不是官方支持的部署方式。

***

## 九、Cloudflare 源站防火墙误拦 Docker 容器出站

### 现象

后台保存 Turnstile 配置时，接口可能返回 500，日志里出现：

```text
validate secret key: send request: Post "https://challenges.cloudflare.com/turnstile/v0/siteverify": dial tcp 104.18.xx.xx:443: i/o timeout
```

同一时间也可能看到其他容器内出站 HTTPS 请求超时，例如：

```text
Failed to fetch remote hash: Get "https://raw.githubusercontent.com/...": dial tcp 185.199.xxx.xxx:443: i/o timeout
```

### 判断方法

先区分宿主机网络和容器网络：

```bash
# 宿主机测试 Cloudflare Turnstile
curl -4 -sS --max-time 10 -o /tmp/turnstile_host.json \
  -w "http=%{http_code} connect=%{time_connect} total=%{time_total} remote=%{remote_ip} err=%{errormsg}\n" \
  -X POST https://challenges.cloudflare.com/turnstile/v0/siteverify \
  -d secret=1 -d response=test

# 在 sub2api 容器网络命名空间内测试
docker run --rm --network container:sub2api curlimages/curl:8.16.0 \
  -4 -sS --max-time 10 -o /tmp/turnstile.json \
  -w "http=%{http_code} connect=%{time_connect} total=%{time_total} remote=%{remote_ip} err=%{errormsg}\n" \
  -X POST https://challenges.cloudflare.com/turnstile/v0/siteverify \
  -d secret=1 -d response=test
```

如果宿主机正常返回 `http=400`，但容器内 `Connection timed out`，说明不是 Turnstile key 本身问题，而是 Docker 容器出站网络被拦。

继续检查 Docker 防火墙链：

```bash
iptables -S DOCKER-USER
iptables -S CF_DOCKER_LOCK
iptables -L CF_DOCKER_LOCK -n -v --line-numbers
```

曾遇到的错误规则：

```text
-A DOCKER-USER -j CF_DOCKER_LOCK
-A CF_DOCKER_LOCK -m conntrack --ctstate RELATED,ESTABLISHED -j RETURN
-A CF_DOCKER_LOCK -s <cloudflare-ip-range> -p tcp -m multiport --dports 80,443 -j RETURN
-A CF_DOCKER_LOCK -p tcp -m multiport --dports 80,443,3000,8080 -j DROP
```

这类规则本意是保护源站端口只允许 Cloudflare 回源访问，但如果挂在 `DOCKER-USER` 后没有区分流量方向，会把容器主动访问外部 80/443 的请求也丢掉。

### 修复方式

编辑 `/usr/local/sbin/cf-origin-firewall.sh`，在 `CF_DOCKER_LOCK` flush 后、DROP 规则前增加 Docker 网桥入方向放行：

```bash
iptables -w -A CF_DOCKER_LOCK -i docker0 -j RETURN
iptables -w -A CF_DOCKER_LOCK -i br+ -j RETURN
```

当前修复位置示例：

```bash
if iptables -w -L DOCKER-USER >/dev/null 2>&1; then
  iptables -w -N CF_DOCKER_LOCK 2>/dev/null || true
  iptables -w -F CF_DOCKER_LOCK
  iptables -w -A CF_DOCKER_LOCK -i docker0 -j RETURN
  iptables -w -A CF_DOCKER_LOCK -i br+ -j RETURN
  iptables -w -A CF_DOCKER_LOCK -m conntrack --ctstate ESTABLISHED,RELATED -j RETURN
  # Cloudflare IP allowlist...
  iptables -w -A CF_DOCKER_LOCK -p tcp -m multiport --dports 80,443,3000,8080 -j DROP
  iptables -w -A CF_DOCKER_LOCK -j RETURN
  iptables -w -C DOCKER-USER -j CF_DOCKER_LOCK 2>/dev/null || iptables -w -I DOCKER-USER 1 -j CF_DOCKER_LOCK
fi
```

重载规则：

```bash
systemctl restart cf-origin-firewall.service
systemctl status cf-origin-firewall.service --no-pager -l
```

### 验证

修复后再次从容器网络命名空间内测试：

```bash
docker run --rm --network container:sub2api curlimages/curl:8.16.0 \
  -4 -sS --max-time 10 -o /tmp/turnstile.json \
  -w "http=%{http_code} connect=%{time_connect} total=%{time_total} remote=%{remote_ip} err=%{errormsg}\n" \
  -X POST https://challenges.cloudflare.com/turnstile/v0/siteverify \
  -d secret=1 -d response=test
```

预期结果是 `http=400` 且几十到几百毫秒返回。这里的 400 是 dummy secret 的预期响应，代表容器可以正常连到 Turnstile；如果仍是超时，继续检查 `DOCKER-USER`、`CF_DOCKER_LOCK` 和 VPS 出站网络。

同时验证其他容器内 HTTPS 出站：

```bash
docker run --rm --network container:sub2api curlimages/curl:8.16.0 \
  -4 -sS --max-time 10 -o /dev/null \
  -w "http=%{http_code} connect=%{time_connect} total=%{time_total} remote=%{remote_ip} err=%{errormsg}\n" \
  https://raw.githubusercontent.com
```

### 本次线上处理记录

- 备份：`/usr/local/sbin/cf-origin-firewall.sh.bak-20260528175239`
- 修复：在 `CF_DOCKER_LOCK` 顶部增加 `-i docker0` 和 `-i br+` 放行。
- 重载：`systemctl restart cf-origin-firewall.service`
- 验证：容器内 Turnstile `siteverify` 从 10 秒超时恢复为约 0.1 秒返回 `http=400`；容器内访问 `raw.githubusercontent.com` 恢复为约 0.08 秒返回。

***

## 十一、2026-06-13 线上相关改动记录

### 1. a2 前台迁 Cloudflare Pages 后关闭 VPS 旧前台入口

2026-06-13 已执行：

- Cloudflare Pages 承载 `test.upit.top` 前台静态资源。
- Pages Worker 将 `/api/*`、`/health`、`/51Token/*` 等回源到 `https://a2t.upit.top`。
- VPS Caddy 备份：`/opt/cf-origin-ssl/Caddyfile.bak-disable-a2-origin-20260613081656`。
- Caddy server label 从 `a2t.upit.top:443, test.upit.top:443` 改为 `a2t.upit.top:443`，只关闭旧前台 origin，不关闭 `a2t.upit.top` 和 `sub2api-test`。
- 验证：`test.upit.top/login` 加载 Cloudflare Pages 资产；`test.upit.top/health`、`a2t.upit.top/health` 均返回 ok；`sub2api-test` 为 `healthy`。

后续不要把 `test.upit.top` 加回 VPS origin，除非明确回滚 Pages。

### 2. Pages public settings 注入和公开只读接口短缓存

2026-06-13 已落地：

- 新增 `scripts/cloudflare-pages-config.mjs` 和 `scripts/inject-pages-public-settings.mjs`，发布前把对应环境 `/api/v1/settings/public` 的公开 `data` 注入到 Pages 静态 `index.html`。
- `frontend/public/_worker.js` 对公开只读 GET 接口启用 Cloudflare Cache API 短缓存：`/api/v1/settings/public`、`/api/status`、`/api/home_page_content`。
- 认证、后台、支付/OAuth、模型网关、`/51Token/*`、长流式接口不缓存，只代理回 VPS。
- 部署前需要运行 `pages-worker.spec.ts` 和 `pages-config-injection.spec.ts`，部署后验证 `X-Sub2API-Edge-Cache`。

发布时只能注入公开 settings `data` 字段，不得把 `.env`、数据库连接、后台配置、Token secret、Turnstile secret 写进静态文件。

### 3. 连接池按环境分档

2026-06-13 已记录并执行过连接池分档，减少共享数据层常驻连接：

| 环境 | `DATABASE_MAX_OPEN_CONNS` | `DATABASE_MAX_IDLE_CONNS` | `REDIS_POOL_SIZE` | `REDIS_MIN_IDLE_CONNS` |
| --- | ---: | ---: | ---: | ---: |
| primary | `12` | `2` | `128` | `2` |
| ap1 / a1 | `12` | `2` | `128` | `2` |
| test / a2t | `4` | `1` | `32` | `1` |

实测 PostgreSQL idle 连接降为：`sub2api=2`、`sub2api_ap1=2`、`sub2api_test=1`。后续重建 compose 或迁移时不要恢复默认大连接池。

## 十二、2026-06-14 线上相关改动记录

### 1. 三套正式前台全部迁 Cloudflare Pages

2026-06-14 起，三套正式前台都按 Cloudflare Pages 发布：

| 前台域名 | Pages 项目 | API 回源域名 | 后端容器 |
| --- | --- | --- | --- |
| `ai.upit.top` | `sub2api-frontend-main` | `https://api.upit.top` | `sub2api` |
| `a1.upit.top` | `sub2api-frontend-a1` | `https://ap1.upit.top` | `sub2api-ap1` |
| `test.upit.top` | `sub2api-frontend-test` | `https://a2t.upit.top` | `sub2api-test` |

边界：

- `ai/a1/test` 只承载前台 HTML / JS / CSS。
- `api/ap1/a2t` 继续承载登录、后台、支付、Turnstile secret 校验、模型网关、`/api/*`、`/health`、`/51Token/*`。
- 前台改动走 Pages 发布；后端/API 常规发布默认 `--skip-frontend-build`。
- VPS 内嵌前台继续作为回滚兜底，但常规发布不再为 VPS 重新打前台产物。

前台发布流程：

```bash
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

pnpm dlx wrangler pages deploy /tmp/sub2api-pages-main --project-name sub2api-frontend-main --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-a1 --project-name sub2api-frontend-a1 --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-test --project-name sub2api-frontend-test --branch subapi
```

### 2. 新 VPS 迁移和共享数据层状态

2026-06-14 新 VPS 当前承载三套 sub2api：

| 环境 | compose 目录 | 容器 | 本机端口 | PostgreSQL database | Redis DB |
| --- | --- | --- | --- | --- | --- |
| primary | `/opt/sub2api-deploy` | `sub2api` | `127.0.0.1:8081` | `sub2api` | `0` |
| a1 / ap1 | `/opt/sub2api-ap1-deploy` | `sub2api-ap1` | `127.0.0.1:8082` | `sub2api_ap1` | `1` |
| test / a2t | `/opt/sub2api-test-deploy` | `sub2api-test` | `127.0.0.1:8083` | `sub2api_test` | `2` |

共享基础设施：

- PostgreSQL：`sub2api-postgres`
- Redis：`sub2api-redis`
- Caddy：`cf-origin-ssl`
- Docker network：`sub2api-deploy_sub2api-network`

当前没有三套独立 Redis/PostgreSQL。三套环境已经共用一组数据库和缓存容器，以不同 PostgreSQL database 和 Redis DB 隔离。不要再把 `sub2api-ap1-postgres`、`sub2api-ap1-redis`、`sub2api-test-postgres`、`sub2api-test-redis` 加回 compose。

### 3. 旧 VPS 备份记录

2026-06-14 旧 VPS 最终备份已放在新 VPS：

```text
/opt/migration-backups/old-vps-final-20260614-210956/
```

包含：

```text
old-vps-final-20260614-210956.tar.zst
old-vps-final-20260614-210956.tar.zst.sha256
```

已在新 VPS 手工比对 sha256，压缩包大小约 `315M`。用户明确要求不需要下载到本地；本地临时 rsync 目录已移除。后续部署和排障统一使用 `51tokens`。

## 十四、2026-06-21 分组用量展示倍率部署说明

本 fork 增加分组级“用量展示倍率”：`groups.usage_multiplier_enabled` 控制开关，`groups.usage_multiplier` 保存倍率；每条新用量在写入 `usage_logs.presentation_multiplier` 时按真实输入、输出、cache creation、cache read 合计是否达到 1000 token 决定是否快照倍率。数据库原始 token/cost 不变，普通用户和普通管理员看到的是展示倍率后的 usage；`super_admin` 看到真实 usage 和真实倍率配置。普通 admin 的分组接口/前端会隐藏并忽略展示倍率配置，避免通过管理页或手工请求泄露/修改真实倍率。

部署注意：

- 必须部署包含 `backend/migrations/159_add_usage_presentation_multiplier.sql` 的后端镜像，让目标环境先完成迁移。
- 只部署 test 时，后端只切 `/opt/sub2api-test-deploy` 的 `sub2api-test`；不要运行会同时滚动 primary/ap1 的 `deploy/local-gzip-binary-deploy.sh --deploy`。
- 如果前台有分组管理页面改动，需要发布 `sub2api-frontend-test` Pages，并注入 `https://a2t.upit.top/api/v1/settings/public`。
- 发布后用 `super_admin` 和普通 `admin` 各查一次 `/api/v1/admin/usage/stats`、`/api/v1/admin/dashboard/stats` 与分组编辑页：普通 admin 应看到展示后的 token/cost/RPM/TPM，分组页不显示展示倍率配置；`super_admin` 应看到 raw token/cost/RPM/TPM 和真实倍率配置；用户侧 group DTO 的 `usage_multiplier` 始终是 `1`。
- 用大于等于 1000 token 的非流式和流式请求各验证一次客户端响应 usage；低于 1000 token 的请求应保持 `presentation_multiplier=1`。

## 十五、2026-06-27 订阅过期管理员邮件提醒部署说明

本 fork 增加“订阅过期管理员提醒”系统设置：后台任务把订阅从 `active` 标记为 `expired` 后，只对本轮刚过期的订阅向配置的管理员邮箱发送一次通知。功能默认关闭，避免升级后对历史过期订阅批量补发邮件。提醒与“订阅到期前 7/3/1 天用户提醒”是独立开关。

部署注意：

- 必须部署包含 `backend/migrations/160_subscription_expired_admin_notify.sql` 的后端镜像，让目标环境先写入默认关闭的系统设置键。
- 后端部署完成后，在“系统设置 -> 邮件设置/通知”中开启“订阅过期管理员提醒”，并添加 `macseek@upit.top` 或其它管理员通知邮箱。后台录入的邮箱默认视为已确认；关闭开关或留空邮箱都不会发送。
- 需要先确认 SMTP 设置可用，否则提醒会被邮件服务拒绝。可用系统设置里的测试邮件功能验证。
- 该功能只通知新一轮状态变更的订阅，不会扫描所有历史 `expired` 订阅；如需补发历史提醒，需要单独写脚本并明确幂等规则。
- 邮件模板事件为 `subscription.expired_admin`，可在邮件模板编辑器中查看或调整。幂等键使用订阅 ID 和 `expired` reminder key，同一订阅对同一收件人不会重复发送。

本地复查命令：

```bash
cd backend
go test ./internal/service ./internal/repository ./internal/handler/dto ./internal/handler/admin ./internal/server/middleware -count=1
go test ./internal/server ./internal/handler -count=1
cd ..
/opt/homebrew/bin/pnpm --dir frontend run typecheck
/opt/homebrew/bin/pnpm --dir frontend run build
git diff --check
```

发布后验证：

```bash
curl -fsS https://test.upit.top/health
curl -fsS https://a2t.upit.top/api/v1/settings/public
# 登录后台后检查 /api/v1/admin/settings 返回 subscription_expired_admin_notify_enabled 与 subscription_expired_admin_notify_emails
```

后续同步官方或重构订阅任务时，继续搜索 `subscription_expired_admin_notify_enabled`、`subscription.expired_admin`、`BatchUpdateExpiredStatus`、`user_subscription_expired`。必须保持默认关闭、只通知本轮刚过期订阅、管理员邮箱不进入公开设置接口这三个约束。

## 十六、2026-06-27 公告邮件推送部署说明

本 fork 增加公告邮件推送：管理员创建或更新公告时，可选择“不推送邮件”“推送给全部用户”或“推送给指定用户”。邮件推送是保存公告时的一次性动作，不改变站内公告的展示条件、弹窗/静默模式和已读逻辑。

部署注意：

- 不需要新增迁移；功能复用现有用户表、公告表、SMTP 配置和通知邮件模板系统。
- 邮件模板事件为 `announcement.publish`，可在邮件模板编辑器中调整标题和正文。
- “全部用户”只推送 `active` 用户；“指定用户”会过滤重复 ID、非 active 用户、无邮箱用户和不存在用户。
- 编辑公告时表单默认仍为“不推送邮件”，避免普通编辑误触发二次群发。
- 幂等键使用公告 ID、用户 ID 和收件邮箱；同一公告对同一用户邮箱不会重复发送。
- 发送失败只记录日志，不回滚公告保存。上线前应先用系统设置里的 SMTP 测试邮件确认邮件链路可用。

本地复查命令：

```bash
cd backend
go test ./internal/service ./internal/handler/admin -run 'TestAnnouncement|TestNotificationEmail' -count=1
cd ..
/opt/homebrew/bin/pnpm --dir frontend run typecheck
/opt/homebrew/bin/pnpm --dir frontend run build
git diff --check
```

后续同步官方或重构公告时，继续搜索 `announcement.publish`、`AnnouncementEmailPushMode`、`email_push_mode`、`email_push_user_ids`。必须保持邮件推送为显式一次性动作，不能变成用户打开公告或公告到达 `starts_at` 后自动补发。

### 7. 2026-06-14 快速复查命令

线上状态总览：

```bash
ssh 51tokens '
  docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"
  docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" \
    sub2api sub2api-ap1 sub2api-test sub2api-postgres sub2api-redis cf-origin-ssl
  free -h
'
```

共享数据层：

```bash
ssh 51tokens '
  docker exec sub2api-postgres psql -U sub2api -d postgres -Atc "select datname from pg_database where datname like '''sub2api%''' order by datname;"
  for n in 0 1 2; do
    echo -n "redis-db$n "
    docker exec sub2api-redis sh -lc "env -u REDISCLI_AUTH redis-cli -n $n DBSIZE"
  done
'
```

Pages / API 健康：

```bash
curl -fsS https://api.upit.top/health
curl -fsS https://ap1.upit.top/health
curl -fsS https://test.upit.top/health
curl -fsS https://ai.upit.top/health
curl -fsS https://a1.upit.top/health
curl -fsS https://a2t.upit.top/health
```

## 十三、2026-06-15 宿主机运维监控与 a1/a2 灰度部署

### 1. 宿主机 CPU 原因采集边界

宿主机 CPU 过高原因不要放进 sub2api 业务容器里实时执行，也不要把 Docker socket 挂给业务容器。当前做法是：

- VPS 宿主机运行 `sub2api-host-health.timer`，每 15 秒执行一次轻量 collector。
- collector 读取 `/proc/loadavg`、`/proc/meminfo`、短采样 `/proc/stat`，并用 `docker stats --no-stream`、`ps` 生成 top 容器和 top 进程。
- collector 原子写入 `/run/sub2api-ops/host-health.json`。
- `sub2api-ap1` 和 `sub2api-test` 只读挂载 `/run/sub2api-ops:/run/sub2api-ops:ro`，后端接口只读 JSON，不在请求路径执行系统命令。
- 前台运维页调用 `/api/v1/admin/ops/host-health` 展示宿主机 CPU、load、可用内存、swap、top containers、top processes 和诊断文本。

相关仓库文件：

```text
deploy/sub2api-host-health-collector.py
deploy/sub2api-host-health.service
deploy/sub2api-host-health.timer
```

VPS 安装或更新 collector：

```bash
ssh 51tokens 'install -d -m 0755 /opt/sub2api-ops'
scp deploy/sub2api-host-health-collector.py 51tokens:/opt/sub2api-ops/sub2api-host-health-collector.py
scp deploy/sub2api-host-health.service deploy/sub2api-host-health.timer 51tokens:/etc/systemd/system/
ssh 51tokens '
  chmod 0755 /opt/sub2api-ops/sub2api-host-health-collector.py
  systemctl daemon-reload
  systemctl enable --now sub2api-host-health.timer
  systemctl start sub2api-host-health.service
  systemctl status sub2api-host-health.timer --no-pager
  cat /run/sub2api-ops/host-health.json
'
```

a1/a2 应用 compose 需要保留只读挂载，不要加到主环境：

```yaml
services:
  sub2api:
    volumes:
      - /run/sub2api-ops:/run/sub2api-ops:ro
    environment:
      SUB2API_HOST_HEALTH_PATH: /run/sub2api-ops/host-health.json
      OPS_HOST_HEALTH_VISIBLE: "true"
```

`ap2` 服务名是 `sub2api-test` 时，也同样只加到对应应用服务下。验证：

```bash
ssh 51tokens '
  docker exec sub2api-ap1 test -r /run/sub2api-ops/host-health.json
  docker exec sub2api-test test -r /run/sub2api-ops/host-health.json
'
```

### 2. 运维监控开关回显修复

2026-06-15 修复后台“系统设置 -> 运维监控”开关刷新后变回关闭的问题。根因是 `/api/v1/admin/settings` 的 GET 回包把数据库持久化值 `ops_monitoring_enabled` 又和 `opsService.IsMonitoringEnabled()` 做了 AND；当运行态 hard switch 或 handler 注入状态与数据库值不一致时，设置页会显示 false，造成“保存成功但刷新变关闭”。

当前边界：

- 设置页 GET 返回数据库持久化值，保证开关回显和保存值一致。
- 运维监控实际接口仍由 `RequireMonitoringEnabled()` 判断是否可访问。
- 如果部署 hard switch 关闭，运维接口会拒绝访问，但设置页不会把数据库保存值误覆盖为关闭。

### 3. 宿主机 CPU 面板按环境显示

宿主机 CPU 面板不是后台数据库开关，而是前台构建环境能力开关。主环境默认隐藏，a1/a2 的 Pages 构建明确打开：

```yaml
VITE_OPS_HOST_HEALTH_VISIBLE=true
```

前端运维页只在 `import.meta.env.VITE_OPS_HOST_HEALTH_VISIBLE === "true"` 时挂载 `OpsHostHealthCard`。主环境不带该变量构建，就不会显示 CPU 面板，也不会请求 `/api/v1/admin/ops/host-health`。

本次同步检查的设置开关联动：

- `ops_monitoring_enabled`：后台设置 GET 返回数据库持久化值；运行时接口仍由 `RequireMonitoringEnabled()` 控制。
- `ops_realtime_monitoring_enabled`：后台设置保存到 DB；实时接口和 WebSocket 通过 `IsRealtimeMonitoringEnabled()` 单独读取。
- `channel_monitor_enabled`、`available_channels_enabled`、`allow_user_view_error_requests`：public settings、HTML 注入和运行时读取均有对应链路。
- `VITE_OPS_HOST_HEALTH_VISIBLE`：只读前台构建环境变量，不进入后台保存请求，避免被设置页误覆盖。

### 4. 迁移文件不可变注意事项

不要修改已经在线上执行过的迁移文件。2026-06-15 首次尝试部署宿主机监控镜像时，a1/a2 因 `145_add_ops_system_disk_gpu_metrics.sql` checksum mismatch 启动失败；原因是该迁移在旧环境已按“磁盘 + GPU 字段”版本执行过，之后本地把 145 改成“只加磁盘字段”导致 checksum 变化。

当前正确状态：

- `backend/migrations/145_add_ops_system_disk_gpu_metrics.sql` 保持旧内容，继续包含 `gpu_usage_percent` 字段和 comment，匹配线上已记录 checksum。
- `backend/migrations/157_remove_ops_gpu_metrics.sql` 负责删除遗留 GPU 字段。
- 移除或调整已执行 schema 时，只追加新迁移，不修改旧迁移。

复查 checksum 可用迁移 runner 同样的 trim 后 SHA-256 算法：

```bash
perl -0pe 's/^\s+|\s+$//g' backend/migrations/145_add_ops_system_disk_gpu_metrics.sql | shasum -a 256
```

期望输出：

```text
3c137690c2146d2b3c9332cf87ee63ac6615452cf42c26339fd4551869aad59b
```

### 5. 只部署 a1/a2 的注意事项

本次宿主机监控先只部署到 a1/a2：

- 后端镜像可以本地构建并上传到新 VPS，但重启时只切换 `/opt/sub2api-ap1-deploy` 和 `/opt/sub2api-test-deploy`。
- 不要运行会滚动 primary 的 `deploy/local-gzip-binary-deploy.sh --deploy`。
- 前台改动只发布 `sub2api-frontend-a1` 和 `sub2api-frontend-test` Pages 项目，不发布 `sub2api-frontend-main`。
- 主环境不挂载 `/run/sub2api-ops`，不暴露宿主机监控卡片数据；后续需要主环境时再单独评估。

只部署 a1/a2 Pages：

```bash
VITE_OPS_HOST_HEALTH_VISIBLE=true pnpm --dir frontend run build
rm -rf /tmp/sub2api-pages-a1 /tmp/sub2api-pages-test
cp -R backend/internal/web/dist /tmp/sub2api-pages-a1
cp -R backend/internal/web/dist /tmp/sub2api-pages-test
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://ap1.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-a1/index.html
node scripts/inject-pages-public-settings.mjs \
  --settings-url https://a2t.upit.top/api/v1/settings/public \
  --html /tmp/sub2api-pages-test/index.html
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-a1 --project-name sub2api-frontend-a1 --branch subapi
pnpm dlx wrangler pages deploy /tmp/sub2api-pages-test --project-name sub2api-frontend-test --branch subapi
```

2026-06-15 已按该边界发布：

- 后端新镜像：`sub2api:subapi-7a7baea8-a1a2-host-health-env-20260615090353`。
- 已切换容器：`sub2api-ap1`、`sub2api-test`；两者均配置 `SUB2API_HOST_HEALTH_PATH=/run/sub2api-ops/host-health.json` 和 `OPS_HOST_HEALTH_VISIBLE=true`。
- 未切换主环境：`sub2api` 仍保持原镜像，主环境 public settings 返回 `ops_host_health_visible=false` 或不注入该字段。
- 已发布 Pages：`sub2api-frontend-a1`、`sub2api-frontend-test`；未发布 `sub2api-frontend-main`。
- 已验证：`https://a1.upit.top/login` 注入 `api_base_url=https://ap1.upit.top/51Token/v1`、`ops_host_health_visible=true`；`https://test.upit.top/login` 注入 `api_base_url=https://a2t.upit.top/51Token/v1`、`ops_host_health_visible=true`；`https://ai.upit.top/login` 未注入宿主机 CPU 面板显示字段。

2026-06-15 后续调整：CPU 面板是否显示改为只看前台构建变量 `VITE_OPS_HOST_HEALTH_VISIBLE=true`，不再让前台通过 public settings 字段决定显示；a1/a2 Pages 发布时带该变量构建，主环境 Pages 构建不带该变量即可隐藏。

### 6. Cloudflare 524 与 OpenAI 上游响应头超时

三套 API 域名均经过 Cloudflare。Cloudflare 与源站连接成功、但源站在 120 秒内没有返回完整响应时，会由边缘生成 524；这类响应不是 sub2api 生成的，后端无法在 524 正文后补充本地化说明。

线上统一配置：

```env
GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT=100
```

该值只限制 OpenAI/Codex 上游返回响应头前的等待时间。超过 100 秒时，sub2api 返回结构化 504 `upstream_timeout`，客户端能够收到中英文说明；上游一旦返回响应头并建立 SSE 流，仍由 `GATEWAY_STREAM_KEEPALIVE_INTERVAL=10` 保活，不限制正常长流的总时长。

2026-07-10 A1 的 524 复盘：对应时段 VPS CPU 正常，OpenAI 账号 53 在 20 分钟内处理 312 个请求，平均首 Token 约 50 秒，最长超过 1500 秒，并出现 `unexpected EOF` 和集中上游 502。故障属于上游账号响应挂起，不是 Cloudflare 无法连接源站，也不是 VPS CPU 打满。

部署后检查：

```bash
ssh 51tokens 'docker exec sub2api env | grep GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT'
ssh 51tokens 'docker exec sub2api-ap1 env | grep GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT'
ssh 51tokens 'docker exec sub2api-test env | grep GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT'
```

三套都应输出 `GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT=100`。不要重新设为 `0` 或高于 120，否则慢上游会再次由 Cloudflare 先返回不可控的 524。
