#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
PAGES_OUT=$(mktemp -d /tmp/sub2api-a1-pages.XXXXXX)
cp -R "$ROOT/backend/internal/web/dist/." "$PAGES_OUT/"
node "$ROOT/scripts/inject-pages-public-settings.mjs" --settings-url https://ap1.upit.top/api/v1/settings/public --html "$PAGES_OUT/index.html"
cd "$ROOT"
pnpm dlx wrangler pages deploy "$PAGES_OUT" --project-name sub2api-frontend-a1 --branch subapi --commit-dirty=true
