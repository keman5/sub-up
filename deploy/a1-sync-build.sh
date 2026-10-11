#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT=${A1_SYNC_OUTPUT:-/tmp/a1-sync-release}
mkdir -p "$OUT"
cd "$ROOT/services/a1-user-sync-service"
go test ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$OUT/a1-user-sync" ./cmd/a1-user-sync
if [[ ${1:-} == --service-only ]]; then exit 0; fi
cd "$ROOT/frontend"
./node_modules/.bin/vitest run src/views/admin/orders/__tests__/AdminOrdersView.sync.spec.ts
./node_modules/.bin/vue-tsc --noEmit
./node_modules/.bin/vite build > "$OUT/frontend-build.log" 2>&1
cd "$ROOT/backend"
go test ./internal/service ./internal/handler/admin -run TestSubscriptionSync -count=1
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags embed -ldflags="-s -w -X main.Version=$(cat cmd/server/VERSION) -X main.Commit=$(git rev-parse --short HEAD)-a1sync -X main.BuildType=release" -o "$OUT/sub2api" ./cmd/server
gzip -c "$OUT/sub2api" > "$OUT/sub2api.gz"
shasum -a 256 "$OUT/a1-user-sync" "$OUT/sub2api"
