#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

HOST="${HOST:-51tokens}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sub2api-runtime-build}"
PRIMARY_COMPOSE_DIR="${PRIMARY_COMPOSE_DIR:-/opt/sub2api-deploy}"
AP1_COMPOSE_DIR="${AP1_COMPOSE_DIR:-${STANDBY_COMPOSE_DIR:-/opt/sub2api-ap1-deploy}}"
TEST_COMPOSE_DIR="${TEST_COMPOSE_DIR:-/opt/sub2api-test-deploy}"
SERVICE_NAME="${SERVICE_NAME:-sub2api}"
TEST_SERVICE_NAME="${TEST_SERVICE_NAME:-sub2api-test}"
PRIMARY_CONTAINER="${PRIMARY_CONTAINER:-sub2api}"
AP1_CONTAINER="${AP1_CONTAINER:-${STANDBY_CONTAINER:-sub2api-ap1}}"
TEST_CONTAINER="${TEST_CONTAINER:-sub2api-test}"
PRIMARY_HEALTH_URL="${PRIMARY_HEALTH_URL:-http://127.0.0.1:8081/health}"
AP1_HEALTH_URL="${AP1_HEALTH_URL:-${STANDBY_HEALTH_URL:-http://127.0.0.1:8082/health}}"
TEST_HEALTH_URL="${TEST_HEALTH_URL:-http://127.0.0.1:8083/health}"
PUBLIC_HEALTH_URL="${PUBLIC_HEALTH_URL:-https://ai.upit.top/health}"
AP1_PUBLIC_HEALTH_URL="${AP1_PUBLIC_HEALTH_URL:-https://a1.upit.top/health}"
TEST_PUBLIC_HEALTH_URL="${TEST_PUBLIC_HEALTH_URL:-https://test.upit.top/health}"
PRIMARY_API_PUBLIC_HEALTH_URL="${PRIMARY_API_PUBLIC_HEALTH_URL:-https://api.upit.top/health}"
AP1_API_PUBLIC_HEALTH_URL="${AP1_API_PUBLIC_HEALTH_URL:-https://ap1.upit.top/health}"
TEST_API_PUBLIC_HEALTH_URL="${TEST_API_PUBLIC_HEALTH_URL:-https://a2t.upit.top/health}"
OUTPUT="${OUTPUT:-/tmp/sub2api-build-output/sub2api}"
BASE_IMAGE="${BASE_IMAGE:-}"
IMAGE_TAG="${IMAGE_TAG:-}"
VERSION="${VERSION:-}"
TAG_SUFFIX="${TAG_SUFFIX:-local-gzip}"
UPLOAD_CHUNK_SIZE="${UPLOAD_CHUNK_SIZE:-1m}"
APPLY=0
DEPLOY=0
SKIP_FRONTEND=0
SKIP_BACKEND=0
SKIP_IMAGE=0

usage() {
  cat <<'EOF'
Usage:
  deploy/local-gzip-binary-deploy.sh [options]

Build a Linux amd64 embedded sub2api binary locally, gzip it, upload it to the VPS,
atomically decompress it on the remote host, build a Docker image, and optionally
roll it out to the test, ap1, and primary compose deployments.

Options:
  --apply                 Execute commands. Default is dry-run.
  --deploy                After building the image, update compose/env files and restart test, ap1, then primary.
  --host HOST             SSH host alias. Default: 51tokens.
  --base-image IMAGE      Docker base image on the remote host. Default: current primary compose image.
  --image-tag TAG         New Docker image tag. Default: sub2api:subapi-<git-sha>-<suffix>-<timestamp>.
  --tag-suffix SUFFIX     Image tag suffix. Default: local-gzip.
  --output PATH           Local binary output path. Default: /tmp/sub2api-build-output/sub2api.
  --skip-frontend-build   Do not run pnpm frontend build.
  --skip-backend-build    Do not run go build; upload the existing --output file.
  --skip-image-build      Upload/decompress only; do not build a Docker image.
  -h, --help              Show this help.

Environment overrides:
  HOST, REMOTE_DIR, PRIMARY_COMPOSE_DIR, AP1_COMPOSE_DIR, TEST_COMPOSE_DIR,
  SERVICE_NAME, TEST_SERVICE_NAME, PRIMARY_CONTAINER, AP1_CONTAINER,
  TEST_CONTAINER, PRIMARY_HEALTH_URL, AP1_HEALTH_URL, TEST_HEALTH_URL,
  PUBLIC_HEALTH_URL, AP1_PUBLIC_HEALTH_URL, TEST_PUBLIC_HEALTH_URL,
  PRIMARY_API_PUBLIC_HEALTH_URL, AP1_API_PUBLIC_HEALTH_URL,
  TEST_API_PUBLIC_HEALTH_URL,
  BASE_IMAGE, IMAGE_TAG, VERSION, TAG_SUFFIX, OUTPUT, UPLOAD_CHUNK_SIZE

Compatibility:
  STANDBY_COMPOSE_DIR, STANDBY_CONTAINER, and STANDBY_HEALTH_URL are still
  accepted as aliases for the ap1 deployment.
EOF
}

log() {
  printf '[local-gzip-deploy] %s\n' "$*"
}

die() {
  printf '[local-gzip-deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

retry_cmd() {
  local attempt status
  for attempt in 1 2 3 4 5; do
    "$@" && return 0
    status=$?
    printf '[local-gzip-deploy] retry %s/5 after exit %s: %s\n' "$attempt" "$status" "$*" >&2
    sleep $((attempt * 2))
  done
  return "$status"
}

upload_file_via_ssh() {
  local local_path="$1"
  local remote_path="$2"
  local attempt status
  for attempt in 1 2 3 4 5; do
    ssh -o BatchMode=yes -o ConnectTimeout=10 -o ServerAliveInterval=10 -o ServerAliveCountMax=3 -o IPQoS=none "$HOST" "cat > '$remote_path'" < "$local_path" && return 0
    status=$?
    printf '[local-gzip-deploy] retry %s/5 after exit %s: upload %s\n' "$attempt" "$status" "$(basename "$local_path")" >&2
    sleep $((attempt * 2))
  done
  return "$status"
}

run() {
  if [[ "$APPLY" -eq 1 ]]; then
    log "run: $*"
    "$@"
  else
    log "dry-run: $*"
  fi
}

ssh_run() {
  local command="$1"
  if [[ "$APPLY" -eq 1 ]]; then
    log "ssh $HOST: ${command//$'\n'/; }"
    retry_cmd ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" "$command"
  else
    log "dry-run ssh $HOST:"
    printf '%s\n' "$command"
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --apply) APPLY=1 ;;
      --deploy) DEPLOY=1 ;;
      --host) HOST="$2"; shift ;;
      --base-image) BASE_IMAGE="$2"; shift ;;
      --image-tag) IMAGE_TAG="$2"; shift ;;
      --tag-suffix) TAG_SUFFIX="$2"; shift ;;
      --output) OUTPUT="$2"; shift ;;
      --skip-frontend-build) SKIP_FRONTEND=1 ;;
      --skip-backend-build) SKIP_BACKEND=1 ;;
      --skip-image-build) SKIP_IMAGE=1 ;;
      -h|--help) usage; exit 0 ;;
      *) die "unknown option: $1" ;;
    esac
    shift
  done
}

current_primary_image() {
  retry_cmd ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" \
    "awk '/image: sub2api:subapi-/ {print \$2; exit}' '$PRIMARY_COMPOSE_DIR/docker-compose.yml'"
}

build_frontend() {
  if [[ "$SKIP_FRONTEND" -eq 1 ]]; then
    log "skip frontend build"
    return
  fi
  run pnpm --dir "$ROOT/frontend" run build
}

build_backend() {
  if [[ "$SKIP_BACKEND" -eq 1 ]]; then
    if [[ -x "$OUTPUT" ]]; then
      log "skip backend build; using $OUTPUT"
    elif [[ "$APPLY" -eq 1 ]]; then
      die "existing output is not executable: $OUTPUT"
    else
      log "skip backend build; dry-run will assume output path $OUTPUT"
    fi
    return
  fi
  mkdir -p "$(dirname "$OUTPUT")"
  local commit
  commit="$(git -C "$ROOT" rev-parse --short HEAD)"
  local ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$commit -X main.BuildType=release"
  (
    cd "$ROOT/backend"
    run env \
      GOOS=linux \
      GOARCH=amd64 \
      CGO_ENABLED=0 \
      GOPROXY=https://goproxy.cn,direct \
      GOSUMDB=sum.golang.google.cn \
      go build -tags embed -ldflags="$ldflags" -o "$OUTPUT" ./cmd/server
  )
  [[ "$APPLY" -eq 0 || -x "$OUTPUT" ]] || die "build did not create executable: $OUTPUT"
}

show_size_plan() {
  if [[ ! -f "$OUTPUT" ]]; then
    log "output does not exist yet: $OUTPUT"
    return
  fi
  local raw_size gz_size
  raw_size=$(wc -c <"$OUTPUT" | tr -d ' ')
  gz_size=$(gzip -c "$OUTPUT" | wc -c | tr -d ' ')
  log "binary size: $raw_size bytes"
  log "gzip transfer size: $gz_size bytes"
}

upload_gzip() {
  if [[ ! -f "$OUTPUT" ]]; then
    if [[ "$APPLY" -eq 1 ]]; then
      die "missing binary output: $OUTPUT"
    fi
    log "dry-run gzip artifact chunked upload:"
    printf 'gzip -c %q > %q.<timestamp>.gz && split -b %q ... && ssh-cat chunks ... && ssh %q %q\n' "$OUTPUT" "$OUTPUT" "$UPLOAD_CHUNK_SIZE" "$HOST" "cat chunks > '$REMOTE_DIR/sub2api.<timestamp>.gz' && gzip -t ... && gzip -dc ... > '$REMOTE_DIR/sub2api.<timestamp>.tmp' && mv ... '$REMOTE_DIR/sub2api'"
    return
  fi
  if [[ "$APPLY" -eq 0 ]]; then
    log "dry-run gzip artifact chunked upload:"
    printf 'gzip -c %q > %q.<timestamp>.gz && split -b %q ... && ssh-cat chunks ... && ssh %q %q\n' "$OUTPUT" "$OUTPUT" "$UPLOAD_CHUNK_SIZE" "$HOST" "cat chunks > '$REMOTE_DIR/sub2api.<timestamp>.gz' && gzip -t ... && gzip -dc ... > '$REMOTE_DIR/sub2api.<timestamp>.tmp' && mv ... '$REMOTE_DIR/sub2api'"
    return
  fi
  local ts
  ts="$(date +%Y%m%d%H%M%S)"
  local local_gz
  local_gz="$OUTPUT.$ts.gz"
  log "create local gzip artifact: $local_gz"
  gzip -c "$OUTPUT" > "$local_gz"
  ls -lh "$local_gz"
  local chunk_dir chunk_count remote_chunk_dir
  chunk_dir="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-gzip-chunks.XXXXXX")"
  remote_chunk_dir="/tmp/sub2api-upload-$ts"
  split -b "$UPLOAD_CHUNK_SIZE" "$local_gz" "$chunk_dir/chunk-"
  chunk_count="$(find "$chunk_dir" -type f -name 'chunk-*' | wc -l | tr -d ' ')"
  log "upload gzip artifact in $chunk_count chunks of $UPLOAD_CHUNK_SIZE and atomic remote decompress to $HOST:$REMOTE_DIR/sub2api"
  retry_cmd ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" "rm -rf '$remote_chunk_dir' && mkdir -p '$remote_chunk_dir'"
  local n=0
  for chunk in "$chunk_dir"/chunk-*; do
    n=$((n + 1))
    log "upload chunk $n/$chunk_count: $(basename "$chunk")"
    upload_file_via_ssh "$chunk" "$remote_chunk_dir/$(basename "$chunk")"
  done
  rm -rf "$chunk_dir"
  rm -f "$local_gz"
  retry_cmd ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" "set -eu
gz='$REMOTE_DIR/sub2api.$ts.gz'
tmp='$REMOTE_DIR/sub2api.$ts.tmp'
cat '$remote_chunk_dir'/chunk-* > \"\$gz\"
rm -rf '$remote_chunk_dir'
gzip -t \"\$gz\"
gzip -dc \"\$gz\" > \"\$tmp\"
chmod +x \"\$tmp\"
mv \"\$tmp\" '$REMOTE_DIR/sub2api'
rm -f \"\$gz\"
ls -lh '$REMOTE_DIR/sub2api'
file '$REMOTE_DIR/sub2api'"
}

ensure_tags() {
  local sha ts
  sha="$(git -C "$ROOT" rev-parse --short HEAD)"
  ts="$(date +%Y%m%d%H%M%S)"
  if [[ -z "$VERSION" ]]; then
    VERSION="$(cd "$ROOT/backend" && ./scripts/resolve-version.sh)"
  fi
  if [[ -z "$IMAGE_TAG" ]]; then
    IMAGE_TAG="sub2api:subapi-${sha}-${TAG_SUFFIX}-${ts}"
  fi
  if [[ -z "$BASE_IMAGE" ]]; then
    if [[ "$APPLY" -eq 1 ]]; then
      BASE_IMAGE="$(current_primary_image)"
    else
      BASE_IMAGE="<current-primary-compose-image>"
    fi
  fi
  log "version: $VERSION"
  log "base image: $BASE_IMAGE"
  log "new image: $IMAGE_TAG"
}

build_remote_image() {
  if [[ "$SKIP_IMAGE" -eq 1 ]]; then
    log "skip remote image build"
    return
  fi
  ssh_run "set -eu
cat > '$REMOTE_DIR/Dockerfile.replace-binary' <<'EOF'
FROM $BASE_IMAGE
USER root
COPY sub2api /app/sub2api
RUN chmod +x /app/sub2api && chown sub2api:sub2api /app/sub2api
EOF
docker build -f '$REMOTE_DIR/Dockerfile.replace-binary' -t '$IMAGE_TAG' '$REMOTE_DIR'"
}

rollout() {
  [[ "$DEPLOY" -eq 1 ]] || return 0
  [[ "$SKIP_IMAGE" -eq 0 ]] || die "--deploy cannot be used with --skip-image-build"
  ssh_run "set -eu
wait_healthy() {
  container=\"\$1\"
  for i in \$(seq 1 40); do
    s=\$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \"\$container\")
    echo \"\$container=\$s\"
    [ \"\$s\" = healthy ] && return 0
    sleep 2
  done
  return 1
}

TS=\$(date +%Y%m%d%H%M%S)

cp '$TEST_COMPOSE_DIR/.env' '$TEST_COMPOSE_DIR/.env.bak-gzip-'\$TS
cp '$TEST_COMPOSE_DIR/docker-compose.yml' '$TEST_COMPOSE_DIR/docker-compose.yml.bak-gzip-'\$TS
if grep -q '^IMAGE_TAG=' '$TEST_COMPOSE_DIR/.env'; then
  sed -i 's#^IMAGE_TAG=.*#IMAGE_TAG=$IMAGE_TAG#' '$TEST_COMPOSE_DIR/.env'
else
  printf '\\nIMAGE_TAG=%s\\n' '$IMAGE_TAG' >> '$TEST_COMPOSE_DIR/.env'
fi
grep '^IMAGE_TAG=' '$TEST_COMPOSE_DIR/.env'
cd '$TEST_COMPOSE_DIR'
docker compose up -d '$TEST_SERVICE_NAME'
wait_healthy '$TEST_CONTAINER'
curl -fsS '$TEST_HEALTH_URL'
curl -fsS '$TEST_PUBLIC_HEALTH_URL'
curl -fsS '$TEST_API_PUBLIC_HEALTH_URL'

cp '$AP1_COMPOSE_DIR/docker-compose.yml' '$AP1_COMPOSE_DIR/docker-compose.yml.bak-gzip-'\$TS
sed -i -E '0,/image: sub2api:subapi-/s#image: sub2api:subapi-[^[:space:]]+#image: $IMAGE_TAG#' '$AP1_COMPOSE_DIR/docker-compose.yml'
grep '^ *image:' '$AP1_COMPOSE_DIR/docker-compose.yml'
cd '$AP1_COMPOSE_DIR'
docker compose up -d '$SERVICE_NAME'
wait_healthy '$AP1_CONTAINER'
curl -fsS '$AP1_HEALTH_URL'
curl -fsS '$AP1_PUBLIC_HEALTH_URL'
curl -fsS '$AP1_API_PUBLIC_HEALTH_URL'

cp '$PRIMARY_COMPOSE_DIR/docker-compose.yml' '$PRIMARY_COMPOSE_DIR/docker-compose.yml.bak-gzip-'\$TS
sed -i -E '0,/image: sub2api:subapi-/s#image: sub2api:subapi-[^[:space:]]+#image: $IMAGE_TAG#' '$PRIMARY_COMPOSE_DIR/docker-compose.yml'
grep '^ *image:' '$PRIMARY_COMPOSE_DIR/docker-compose.yml'
cd '$PRIMARY_COMPOSE_DIR'
docker compose up -d '$SERVICE_NAME'
wait_healthy '$PRIMARY_CONTAINER'
curl -fsS '$PRIMARY_HEALTH_URL'
curl -fsS '$PUBLIC_HEALTH_URL'
curl -fsS '$PRIMARY_API_PUBLIC_HEALTH_URL'"
}

main() {
  parse_args "$@"
  ensure_tags
  log "mode: $([[ "$APPLY" -eq 1 ]] && echo apply || echo dry-run)"
  log "deploy: $([[ "$DEPLOY" -eq 1 ]] && echo yes || echo no)"
  build_frontend
  build_backend
  show_size_plan
  upload_gzip
  build_remote_image
  rollout
}

main "$@"
