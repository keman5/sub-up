#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
work="${1:?output directory required}"
image="${2:?image tag required}"
[[ -d "$work" && "$work" != / ]]
export GOTOOLCHAIN=go1.27.3
unset GO_COMPAT_VERSION GOFLAGS
endpoint="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
[[ "$endpoint" == unix://* && "${DOCKER_HOST:-$endpoint}" == unix://* ]] || { echo 'Use a local Docker unix socket context and DOCKER_HOST.' >&2; exit 1; }
# Pin subsequent Docker calls to the checked socket, independent of context or
# Buildx/BuildKit environment overrides and the user's selected remote builder.
export DOCKER_HOST="${DOCKER_HOST:-$endpoint}"
unset DOCKER_CONTEXT BUILDX_BUILDER BUILDKIT_HOST BUILDX_CONFIG DOCKER_TLS_VERIFY DOCKER_CERT_PATH
go clean -cache
find "$ROOT" -path "$ROOT/.git" -prune -o -type d -name .artifacts -prune -exec rm -rf {} +
PNPM=(npx --yes pnpm@9.15.9)
commit="$(git rev-parse --short HEAD)"
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
  docker buildx build --builder default --platform linux/amd64 --load -t "$image" -f Dockerfile.goreleaser "$work"
fi
[[ "$(docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}')" == linux/amd64 ]]
