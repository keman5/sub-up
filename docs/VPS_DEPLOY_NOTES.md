# VPS 部署与更新排障记录

本文记录将 `https://github.com/keman5/51token.git` 部署到 VPS 时遇到的问题、判断方法和可复用处理方案。适用于已有 `new-api` 容器、需要更新到当前仓库代码的场景。

> 注意：不要把 `SQL_DSN`、`SESSION_SECRET`、`CRYPTO_SECRET`、Redis 密码等环境变量写入文档、聊天记录或公开日志。排障时只确认变量是否存在，避免展开具体值。

***

## 一、部署前检查

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

如果 VPS 上已有旧容器，不要直接覆盖。先记录旧容器的端口、挂载、网络和重启策略：

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
docker inspect new-api --format '{{json .Mounts}}'
docker inspect new-api --format '{{json .HostConfig.PortBindings}}'
docker inspect new-api --format '{{json .NetworkSettings.Networks}}'
docker inspect new-api --format '{{.HostConfig.RestartPolicy.Name}}'
```

本次线上容器的关键配置是：

- 容器名：`new-api`
- 网络：`new-api-net`
- 端口：`80:3000` 和 `3000:3000`
- 数据挂载：`/opt/new-api/data:/data`
- 日志挂载：`/opt/new-api/logs:/app/logs`
- Redis 容器：`new-api-redis`

***

## 二、GitHub 仓库 Docker 构建问题

### 1. `web/classic/.npmrc` 缺失

当前 `Dockerfile` 里有：

```dockerfile
COPY web/classic/.npmrc .
```

如果仓库里没有 `web/classic/.npmrc`，干净构建会在该步骤失败。应提交一个最小配置：

```text
registry=https://registry.npmjs.org/
```

### 2. VPS 无法访问 npm registry

曾遇到：

```text
Error when performing the request to https://registry.npmjs.org/pnpm/-/pnpm-10.33.2.tgz
ETIMEDOUT
```

这说明 VPS 到 npm registry 的出口网络不可用或不稳定。可以先测试：

```bash
curl -I --max-time 15 https://registry.npmjs.org/pnpm/-/pnpm-10.33.2.tgz
curl -I --max-time 15 https://registry.npmmirror.com/pnpm/-/pnpm-10.33.2.tgz
```

如果镜像源可达，可以临时给构建阶段加：

```dockerfile
ENV COREPACK_NPM_REGISTRY=https://registry.npmmirror.com
ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
```

如果 Corepack 仍超时，可以绕过 Corepack：

```dockerfile
RUN npm install -g pnpm@10.33.2 --registry=https://registry.npmmirror.com \
    && pnpm install --frozen-lockfile
```

### 3. VPS 无法访问 Debian apt 源

曾遇到：

```text
Could not connect to deb.debian.org:80
Package 'ca-certificates' has no installation candidate
Unable to locate package libasan8
```

这说明运行镜像构建阶段无法执行 `apt-get update`。如果只是更新应用二进制，且 VPS 已有可用的 `calciumion/new-api:latest` 镜像，可以复用旧运行镜像作为基础镜像，仅替换 `/new-api`：

```dockerfile
FROM calciumion/new-api:latest
COPY new-api /new-api
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
```

### 4. VPS 上 Go 依赖下载过慢或卡住

曾遇到 `go mod download` 长时间无输出。可尝试：

```dockerfile
ENV GOPROXY=https://goproxy.cn,direct
```

如果 VPS 出口仍不稳定，建议改为本机交叉编译，再把二进制传到 VPS。

***

## 三、网络不稳定时的可用替代部署方案

当 VPS 无法稳定访问 npm、apt、Go proxy 时，可以采用“本机构建前端和后端，VPS 只打运行镜像”的方式。

### 1. 本机构建前端

```bash
cd web/default
pnpm run build

cd ../classic
pnpm run build
```

### 2. 本机交叉编译 Linux amd64 后端

在项目根目录执行：

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOEXPERIMENT=greenteagc \
go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" \
  -o /tmp/51token-new-api
```

确认二进制：

```bash
file /tmp/51token-new-api
```

应看到类似：

```text
ELF 64-bit LSB executable, x86-64, statically linked
```

### 3. 上传到 VPS

链路慢时，先压缩再上传：

```bash
gzip -c /tmp/51token-new-api > /tmp/51token-new-api.gz
scp -C /tmp/51token-new-api.gz new-api-vps:/opt/51token-build/new-api.gz
```

在 VPS 解压：

```bash
cd /opt/51token-build
gzip -df new-api.gz
chmod +x new-api
```

### 3.1 sub2api 本地构建产物的 gzip 分块上传与远端解压方案

当前 sub2api 嵌入前端后的 Linux amd64 二进制约 86 MB。VPS 出口和本地到 VPS 链路不稳定时，不要直接 `scp` 原始二进制，也不要在 VPS 上重新拉取 npm / Go / apt 依赖。标准流程是：

1. 本机构建前端。
2. 本机交叉编译 Linux amd64 后端。
3. 本地使用 `gzip -c` 生成 `.gz` 产物。
4. 将 `.gz` 切成 1 MB 小块，通过 SSH 标准输入逐块写入远端 `/tmp/sub2api-upload-<timestamp>`。
5. 远端按文件名顺序拼接为临时 `.gz`，执行 `gzip -t` 校验。
6. 远端解压到临时可执行文件。
7. `chmod +x` 后原子 `mv` 为 `/opt/sub2api-runtime-build/sub2api`。
8. 基于上一版运行镜像只替换 `/app/sub2api`，再按 standby、primary 顺序滚动。

说明：2026-06-01 线上重部署时，单条 gzip 管道和单文件 scp 在当前 VPS SSH 链路上都出现过中途停住；脚本已改为小块短连接上传，并带 5 次退避重试。

仓库已提供脚本：

```bash
# 只打印计划，不改远端
deploy/local-gzip-binary-deploy.sh

# 使用已有本地产物做 dry-run
deploy/local-gzip-binary-deploy.sh --skip-frontend-build --skip-backend-build

# 构建、gzip 上传、远端解压、打镜像，但不重启服务
deploy/local-gzip-binary-deploy.sh --apply

# 构建、gzip 上传、远端解压、打镜像，并滚动部署 standby + primary
deploy/local-gzip-binary-deploy.sh --apply --deploy
```

可用环境变量或参数覆盖目标：

```bash
HOST=new-api-vps \
BASE_IMAGE=sub2api:subapi-6b800b77-logo-dbterms-20260530222347 \
IMAGE_TAG=sub2api:subapi-$(git rev-parse --short HEAD)-gzip-$(date +%Y%m%d%H%M%S) \
deploy/local-gzip-binary-deploy.sh --apply --deploy
```

脚本关键安全点：

- 默认 dry-run；必须显式 `--apply` 才会改远端。
- gzip 上传后远端先拼接并 `gzip -t` 校验，再解压。
- 默认分块大小为 `UPLOAD_CHUNK_SIZE=1m`，可按链路情况覆盖。
- 解压到 `sub2api.<timestamp>.tmp`，校验权限后再 `mv` 覆盖正式二进制。
- compose 更新前会备份为 `docker-compose.yml.bak-gzip-<timestamp>`。
- `--deploy` 会先更新 `sub2api-standby`，确认 healthy 后再更新 `sub2api`。
- 最后验证 standby、primary 和公开 `/health`。

### 3.2 动态模型路由上线核对

动态模型路由的代码默认配置是关闭的，线上必须通过 compose 环境变量显式开启，否则即使账号 `extra` 中已有 Codex 5h / 7d 用量快照，请求也仍会按调用方原始模型转发。

生产 compose 需要保留以下环境变量：

```yaml
- GATEWAY_MODEL_ROUTER_ENABLED=${GATEWAY_MODEL_ROUTER_ENABLED:-true}
- GATEWAY_MODEL_ROUTER_OAUTH_MODE=${GATEWAY_MODEL_ROUTER_OAUTH_MODE:-passthrough}
- GATEWAY_MODEL_ROUTER_DEFAULT_MODEL=${GATEWAY_MODEL_ROUTER_DEFAULT_MODEL:-gpt-5.3-codex-spark}
- GATEWAY_MODEL_ROUTER_BALANCED_MODEL=${GATEWAY_MODEL_ROUTER_BALANCED_MODEL:-gpt-5.4}
- GATEWAY_MODEL_ROUTER_PREMIUM_MODEL=${GATEWAY_MODEL_ROUTER_PREMIUM_MODEL:-gpt-5.5}
- GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_CHARS=${GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_CHARS:-12000}
- GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_ITEMS=${GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_MIN_ITEMS:-20}
- GATEWAY_MODEL_ROUTER_PRESSURE_LOW_REMAINING_PERCENT=${GATEWAY_MODEL_ROUTER_PRESSURE_LOW_REMAINING_PERCENT:-40}
- GATEWAY_MODEL_ROUTER_PRESSURE_MEDIUM_REMAINING_PERCENT=${GATEWAY_MODEL_ROUTER_PRESSURE_MEDIUM_REMAINING_PERCENT:-70}
- GATEWAY_MODEL_ROUTER_IMAGE_OR_VISION_FORCE_PREMIUM=${GATEWAY_MODEL_ROUTER_IMAGE_OR_VISION_FORCE_PREMIUM:-true}
```

`GATEWAY_MODEL_ROUTER_OAUTH_MODE` 默认 `passthrough`，OpenAI OAuth / Codex Pro 账号保持调用方原始模型。若要让 OAuth 账号参与 5.3 Spark / 5.4 / 5.5 自适应路由，先仅在 ap2/a2 灰度设置为 `adaptive_codex`；ap1 和主环境保持 `passthrough` 或关闭动态路由，等 a2 验证稳定后再推进。

路由分层规则：普通文本优先 `GATEWAY_MODEL_ROUTER_DEFAULT_MODEL`，中等复杂文本走 `GATEWAY_MODEL_ROUTER_BALANCED_MODEL`，超过 `GATEWAY_MODEL_ROUTER_PREMIUM_INPUT_*` 的大请求、图片/视觉或连续能力失败升级到 `GATEWAY_MODEL_ROUTER_PREMIUM_MODEL`。当账号 5h/7d 剩余额度进入压力区间时，会优先压回 economy/balanced 以节省额度。

对外隐藏规则：动态路由只改变内部上游 `upstream_model`；客户端响应体、流式 SSE `model` 字段和普通用户用量记录继续显示调用方请求模型。真实上游模型仅在管理员用量/运维日志中保留，用于排障和成本核对。

上线后检查：

```bash
ssh new-api-vps 'docker exec sub2api env | grep GATEWAY_MODEL_ROUTER'
ssh new-api-vps 'docker exec sub2api-standby env | grep GATEWAY_MODEL_ROUTER'
curl -fsS https://ai.upit.top/health
curl -fsS https://a1.upit.top/health
```

若用户反馈高压账号仍未切到 `gpt-5.3-codex-spark`，先看容器环境变量，再查对应 OpenAI 账号 `extra` 中的 `codex_5h_used_percent`、`codex_7d_used_percent` 和最近 `usage_logs.model/upstream_model`。

### 3.3 ap2 / a2 灰度环境单独重部署

`ap2` 不是 `sub2api-standby`。当前线上单独存在一套灰度环境：

- compose 目录：`/opt/sub2api-ap2-deploy`
- compose project：`sub2api-ap2-deploy`
- 服务名 / 容器名：`sub2api-ap2`
- 本地健康检查：`http://127.0.0.1:8083/health`
- 镜像来源：`/opt/sub2api-ap2-deploy/.env` 中的 `IMAGE_TAG=...`

因此，如果只是重新部署 `ap2/a2`，不要使用 `deploy/local-gzip-binary-deploy.sh --apply --deploy`，因为那个流程会更新 `sub2api-standby` 和 `sub2api`。正确做法是：

1. 先用脚本仅完成“本地构建 + gzip 上传 + 远端替换 `/opt/sub2api-runtime-build/sub2api` + 打新镜像”，不要滚动主环境：

```bash
IMAGE_TAG="sub2api:subapi-<git-sha>-ap2-redeploy-$(date +%Y%m%d%H%M%S)" \
BASE_IMAGE="$(ssh new-api-vps \"grep '^IMAGE_TAG=' /opt/sub2api-ap2-deploy/.env | cut -d= -f2-\")" \
deploy/local-gzip-binary-deploy.sh --apply
```

2. 然后仅修改 `ap2` 目录下的 `.env` 并重启 `sub2api-ap2`：

```bash
ssh new-api-vps '
  set -eu
  cd /opt/sub2api-ap2-deploy
  TS=$(date +%Y%m%d%H%M%S)
  cp .env .env.bak.$TS.redeploy-ap2
  sed -i "s#^IMAGE_TAG=.*#IMAGE_TAG=sub2api:subapi-<git-sha>-ap2-redeploy-<timestamp>#\" .env
  docker compose up -d sub2api-ap2
  curl -fsS http://127.0.0.1:8083/health
'
```

3. 再确认容器与镜像：

```bash
ssh new-api-vps '
  docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" sub2api-ap2
  docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep sub2api-ap2
'
```

2026-06-02 实测：

- 旧镜像：`sub2api:subapi-a5e4b0c6-ap2-oauth-adaptive-v2-202606021754`
- 新镜像：`sub2api:subapi-9d75fb6b-ap2-redeploy-20260602232707`
- `sub2api-ap2` 更新后状态为 `healthy`
- `http://127.0.0.1:8083/health` 返回 `{"status":"ok"}`

如果只想验证灰度 OAuth/Codex 自适应路由，不要动 `sub2api-standby` 或 `sub2api`，保持 `ap2` 独立切换即可。

底层等价命令示例：

```bash
gzip -c /tmp/sub2api-build-output/sub2api | ssh new-api-vps '
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

### 4. 构建只替换二进制的运行镜像

```bash
cat > /opt/51token-build/Dockerfile.replace-binary <<'EOF'
FROM calciumion/new-api:latest
COPY new-api /new-api
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
EOF

docker build \
  -f /opt/51token-build/Dockerfile.replace-binary \
  -t 51token:$(date +%Y%m%d%H%M%S) \
  /opt/51token-build
```

***

## 四、更新容器时保留旧容器以便回滚

### 1. 导出旧容器环境变量

```bash
TS=$(date +%Y%m%d%H%M%S)
ENV_FILE=/opt/new-api/new-api.env.$TS
docker inspect new-api --format '{{range .Config.Env}}{{println .}}{{end}}' > "$ENV_FILE"
```

此文件包含敏感信息，部署完成后应删除：

```bash
rm -f /opt/new-api/new-api.env.*
```

### 2. 启动新容器

```bash
BACKUP_NAME=new-api-prev-$TS

docker stop new-api
docker rename new-api "$BACKUP_NAME"

docker run -d \
  --name new-api \
  --restart unless-stopped \
  --network new-api-net \
  --env-file "$ENV_FILE" \
  -p 80:3000 \
  -p 3000:3000 \
  -v /opt/new-api/data:/data \
  -v /opt/new-api/logs:/app/logs \
  51token:YOUR_TAG
```

### 3. 健康检查

```bash
curl -I --max-time 10 http://127.0.0.1:3000/
docker logs --tail 80 new-api
```

期望结果：

```text
HTTP/1.1 200 OK
New API started
ready in ... ms
```

***

## 五、启动迁移很慢导致误判失败

本次更新时，新容器会对 PostgreSQL 做 GORM 自动迁移和表结构检查。日志里会出现大量类似内容：

```text
SLOW SQL >= 200ms
SELECT ... FROM information_schema.columns ...
ALTER TABLE ...
```

启动期间 `curl http://127.0.0.1:3000/` 可能反复出现：

```text
Empty reply from server
Recv failure: Connection reset by peer
```

这不一定代表容器崩溃，可能只是迁移尚未结束。短健康检查窗口（如 5 秒、90 秒、180 秒）都可能误判并触发回滚。建议：

- 给首次启动至少 10-15 分钟健康检查窗口
- 观察 `docker logs -f new-api`，确认迁移是否仍在推进
- 不要在迁移仍推进时反复杀容器，否则每次都会重新走一部分启动检查

### 可选：预热迁移

可以先开一个不占用公网端口的临时容器，让迁移先跑完：

```bash
docker rm -f new-api-warmup 2>/dev/null || true

docker run -d \
  --name new-api-warmup \
  --network new-api-net \
  --env-file "$ENV_FILE" \
  -v /opt/new-api/data:/data \
  -v /opt/new-api/logs:/app/logs \
  51token:YOUR_TAG

for i in $(seq 1 360); do
  if docker exec new-api-warmup wget -qO- --timeout=3 http://127.0.0.1:3000/ >/dev/null 2>&1; then
    echo "warmup-ready"
    break
  fi
  sleep 2
done

docker rm -f new-api-warmup
```

注意：预热容器和正式容器同时连接同一个数据库时，只建议用于一次性迁移预热，不要长期并行运行两个应用实例。

***

## 六、回滚方式

如果新容器无法启动：

```bash
docker logs --tail 200 new-api
docker rm -f new-api
docker rename new-api-prev-YYYYMMDDHHMMSS new-api
docker start new-api
```

回滚后再确认：

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
curl -I --max-time 10 http://127.0.0.1:3000/
```

***

## 七、多站点 Caddy 静态资源路由

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

## 八、Turnstile 前端脚本与 CSP

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

## 十、本次实际结论

本次最终可行路径是：

1. 本机完成前端构建和 Linux amd64 后端编译
2. 压缩上传后端二进制到 VPS
3. 基于 VPS 已有的 `calciumion/new-api:latest` 镜像替换 `/new-api`
4. 使用旧容器的端口、挂载、网络和环境变量启动新容器
5. 把健康检查窗口拉长，等待 PostgreSQL 迁移完成

最终验证：

```text
new-api   51token:<tag>   Up
HTTP/1.1 200 OK
New API started
```
