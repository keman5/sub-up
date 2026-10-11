#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
(cd frontend && ./node_modules/.bin/vitest run src/views/admin/__tests__/SubscriptionsView.sync.spec.ts src/views/admin/orders/__tests__/AdminOrdersView.sync.spec.ts)
(cd services/a1-user-sync-service && GOTOOLCHAIN=local go test ./...)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp backend/go.mod "$work/check.mod"
cp backend/go.sum "$work/check.sum"
if [[ "$(GOTOOLCHAIN=local go env GOVERSION)" == go1.26.* ]]; then
  export GO_COMPAT_VERSION="${GO_COMPAT_VERSION:-$(GOTOOLCHAIN=local go env GOVERSION | sed 's/^go//')}"
  export GOEXPERIMENT="${GOEXPERIMENT:-jsonv2}"
  export GOTOOLCHAIN=local
fi
if [[ -n "${GO_COMPAT_VERSION:-}" ]]; then
  (cd backend && go mod edit -modfile="$work/check.mod" -go="$GO_COMPAT_VERSION")
fi
(cd backend && go test -modfile="$work/check.mod" ./internal/service -run 'TestSubscriptionSync' -count=1)
git diff --check
bash -n deploy/a1-sync-release.sh
