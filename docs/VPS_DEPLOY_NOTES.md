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

## 七、Cloudflare 源站防火墙误拦 Docker 容器出站

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

## 八、本次实际结论

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
