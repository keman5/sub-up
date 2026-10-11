#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
HOST="${HOST:-51tokens}"
DRY_RUN=0
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  --help|-h) echo 'npm run deploy [-- --dry-run]'; exit 0 ;;
  '') ;;
  *) echo "Unknown option: $1" >&2; exit 2 ;;
esac
[[ $# -le 1 ]] || exit 2
TARGETS=(a1 3dbooks main)
DIRS=(/opt/sub2api-ap1-deploy /opt/sub2api-retired/sub2api-3dbooks-deploy-20260918151421 /opt/sub2api-deploy)
SERVICES=(sub2api sub2api-3dbooks sub2api)
PORTS=(8082 8083 8081)
ORIGINS=(https://ap1.upit.top https://api.3dbooks.top https://api.upit.top)
SITES=(https://a1.upit.top https://ai.3dbooks.top https://ai.upit.top)
log() { printf '[deploy] %s\n' "$*"; }
if [[ "$DRY_RUN" == 1 ]]; then
  log 'Local frontend builds (main/3dbooks CPU panel off; a1 on), Go linux/amd64, Docker image; upload and docker load.'
  for i in "${!TARGETS[@]}"; do
    log "${TARGETS[$i]}: ${DIRS[$i]} / ${SERVICES[$i]} / ${PORTS[$i]} / sub2api-frontend-${TARGETS[$i]} / ${SITES[$i]}"
  done
  log 'Health-check each backend and Pages before continuing. Retired test is excluded.'
  exit 0
fi
for cmd in node npx go docker ssh gzip split curl python3; do
  command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done
PNPM=(npx --yes pnpm@9.15.9)
# A remote Docker context would violate the local-build requirement.
endpoint="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
[[ "${DOCKER_HOST:-$endpoint}" == unix://* ]] || { echo 'Use a local Docker unix socket context.' >&2; exit 1; }
docker info >/dev/null
SSH=(ssh -o BatchMode=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=3 "$HOST")
"${SSH[@]}" 'command -v python3 >/dev/null; docker compose version >/dev/null; docker image ls >/dev/null'
"${PNPM[@]}" dlx wrangler whoami
work="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-deploy.XXXXXX")"
trap 'rm -rf "$work"' EXIT
# The installed 1.26 toolchain gates jsontext behind jsonv2; keep compatibility local to the release.
if [[ "$(GOTOOLCHAIN=local go env GOVERSION)" == go1.26.* ]]; then
  export GO_COMPAT_VERSION="${GO_COMPAT_VERSION:-$(GOTOOLCHAIN=local go env GOVERSION | sed 's/^go//')}"
  export GOEXPERIMENT="${GOEXPERIMENT:-jsonv2}"
  export GOTOOLCHAIN=local
  log "Using local Go $GO_COMPAT_VERSION with jsonv2 and a temporary modfile"
fi
stamp="$(date +%Y%m%d%H%M%S)"
if [[ -n "${UPLOADED_RELEASE:-}" ]]; then
  [[ "$UPLOADED_RELEASE" =~ ^[0-9]{14}$ && -n "${PREBUILT_IMAGE:-}" ]] || exit 2
  stamp="$UPLOADED_RELEASE"
fi
commit="$(git rev-parse --short HEAD)"
if [[ -n "${GO_COMPAT_VERSION:-}" ]]; then
  cp backend/go.mod "$work/build.mod"
  cp backend/go.sum "$work/build.sum"
  (cd backend && go mod edit -modfile="$work/build.mod" -go="$GO_COMPAT_VERSION")
  export GOFLAGS="${GOFLAGS:-} -modfile=$work/build.mod"
fi
image="${PREBUILT_IMAGE:-sub2api:subapi-$commit-all-$stamp}"
log "Build and verify release $image"
CI=true "${PNPM[@]}" --dir frontend install --frozen-lockfile
"${PNPM[@]}" --dir frontend exec vitest run src/cloudflare/__tests__/pages-worker.spec.ts src/cloudflare/__tests__/pages-config-injection.spec.ts
(cd backend && go test ./internal/handler/admin ./cmd/server)
VITE_OPS_HOST_HEALTH_VISIBLE=false "${PNPM[@]}" --dir frontend run build
cp -R backend/internal/web/dist "$work/main"
cp -R backend/internal/web/dist "$work/3dbooks"
version="$(bash backend/scripts/resolve-version.sh)"
if [[ -z "${PREBUILT_IMAGE:-}" ]]; then
  (cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed -trimpath -ldflags="-s -w -X main.Version=$version -X main.Commit=$commit -X main.BuildType=release" -o "$work/sub2api" ./cmd/server)
fi
VITE_OPS_HOST_HEALTH_VISIBLE=true "${PNPM[@]}" --dir frontend run build
cp -R backend/internal/web/dist "$work/a1"
mkdir -p "$work/backend" "$work/deploy"
cp -R backend/resources "$work/backend/resources"
cp deploy/docker-entrypoint.sh "$work/deploy/"
if [[ -z "${PREBUILT_IMAGE:-}" ]]; then
  docker buildx build --platform linux/amd64 --load -t "$image" -f Dockerfile.goreleaser "$work"
fi
[[ "$(docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}')" == linux/amd64 ]]
remote="/tmp/sub2api-deploy-$stamp"
if [[ -z "${UPLOADED_RELEASE:-}" ]]; then
docker save "$image" | gzip -c > "$work/image.tar.gz"
gzip -t "$work/image.tar.gz"
mkdir "$work/chunks"
split -b 1m "$work/image.tar.gz" "$work/chunks/chunk-"
"${SSH[@]}" "mkdir -p '$remote'"
for chunk in "$work"/chunks/chunk-*; do
  uploaded=0
  for attempt in 1 2 3 4 5; do
    if "${SSH[@]}" "cat > '$remote/$(basename "$chunk")'" < "$chunk"; then uploaded=1; break; fi
    sleep "$attempt"
  done
  [[ "$uploaded" == 1 ]] || { echo 'Upload failed; no services changed.' >&2; exit 1; }
done
fi
"${SSH[@]}" bash -s -- "$remote" "$image" <<'REMOTE'
set -euo pipefail
cat "$1"/chunk-* > "$1/image.tar.gz"
gzip -t "$1/image.tar.gz"
gzip -dc "$1/image.tar.gz" | docker load
docker image inspect "$2" >/dev/null
rm -rf "$1"
REMOTE
for i in "${!TARGETS[@]}"; do
  name="${TARGETS[$i]}"
  log "Deploy $name backend"
  "${SSH[@]}" bash -s -- "${DIRS[$i]}" "${SERVICES[$i]}" "$image" "${PORTS[$i]}" "$name" "$stamp" <<'REMOTE'
set -euo pipefail
cd "$1"
service="$2"; image="$3"; port="$4"; name="$5"; stamp="$6"
# Compose override updates only the application, preserving secrets and shared dependencies.
override=docker-compose.override.yml
[ ! -f "$override" ] || cp "$override" "$override.bak-$stamp"
docker compose config --format json | python3 -c 'import json,sys; c=json.load(sys.stdin); assert sys.argv[1] in c["services"]' "$service"
docker compose config --format json | python3 -c 'import json,sys; c=json.load(sys.stdin); assert not any(v.get("target") == "/app/sub2api" for v in c["services"][sys.argv[1]].get("volumes", [])), "Remove the obsolete /app/sub2api bind mount before deploying; it overrides the image binary"' "$service"
# Preserve existing override settings, including mounts and resource limits.
if [ -f "$override" ]; then
  docker compose -f "$override" config --no-consistency --format json > release-override.tmp.json
else
  printf '{}\n' > release-override.tmp.json
fi
python3 - "$service" "$image" "$override" <<'PY'
import json,sys
with open('release-override.tmp.json') as f:
    config=json.load(f)
config.pop('name', None)
config.setdefault('services', {}).setdefault(sys.argv[1], {}).update(image=sys.argv[2], restart='unless-stopped')
with open(sys.argv[3], 'w') as f:
    json.dump(config, f)
PY
rm release-override.tmp.json
docker compose -f docker-compose.yml -f "$override" up -d --no-deps --pull never "$service"
container="$(docker compose -f docker-compose.yml -f "$override" ps -q "$service")"
[[ "$(docker inspect -f '{{.Config.Image}}' "$container")" == "$image" ]]
healthy=0
for attempt in $(seq 1 60); do
  if [[ "$(docker inspect -f '{{.State.Health.Status}}' "$container")" == healthy ]]; then healthy=1; break; fi
  sleep 2
done
[[ "$healthy" == 1 ]]
curl -fsS --max-time 15 "http://127.0.0.1:$port/health" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"] == "ok"'
REMOTE
  curl -fsS --retry 3 --max-time 20 "${ORIGINS[$i]}/health" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"] == "ok"'
  log "Publish $name Pages"
  node scripts/inject-pages-public-settings.mjs --settings-url "${ORIGINS[$i]}/api/v1/settings/public" --html "$work/$name/index.html"
  "${PNPM[@]}" dlx wrangler pages deploy "$work/$name" --project-name "sub2api-frontend-$name" --branch subapi
  curl -fsS --retry 5 --retry-delay 3 --max-time 20 "${SITES[$i]}/health" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"] == "ok"'
done
bash deploy/a1-sync-release.sh --apply
log "All three environments and subscription sync consumer deployed: $image"
