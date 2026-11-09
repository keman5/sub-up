#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
out="${1:?output directory required}"
[[ -d "$out" && "$out" == /* && "$out" != / ]]
# The 9fddec9-tagged release included the later committed sync receipt fix.
# d247bad03 records that frontend state without the unreleased expiry feature.
base=d247bad03
go clean -cache
find "$ROOT" -path "$ROOT/.git" -prune -o -type d -name .artifacts -prune -exec rm -rf {} +
mkdir "$out/source"
git -C "$ROOT" archive "$base" frontend scripts docs/legal | tar -x -C "$out/source"
mkdir -p "$out/source/backend/internal/web"
node --input-type=module - "$out/source/frontend/src/views/admin/SubscriptionsView.vue" <<'JS'
import fs from 'node:fs';
import assert from 'node:assert/strict';
const file = process.argv[2];
const source = fs.readFileSync(file, 'utf8');
const before = `v-if="row.status === 'active'"\n                @click="handleResetQuota(row)"`;
const after = `v-if="row.status === 'active' || row.status === 'expired'"\n                @click="handleResetQuota(row)"`;
assert.equal(source.split(before).length, 2);
fs.writeFileSync(file, source.replace(before, after));
JS
cp "$ROOT/frontend/src/views/admin/__tests__/SubscriptionsView.sync.spec.ts" "$out/source/frontend/src/views/admin/__tests__/SubscriptionsView.sync.spec.ts"
PNPM=(npx --yes pnpm@9.15.9)
CI=true "${PNPM[@]}" --dir "$out/source/frontend" install --frozen-lockfile
"${PNPM[@]}" --dir "$out/source/frontend" exec vitest run src/views/admin/__tests__/SubscriptionsView.sync.spec.ts src/cloudflare/__tests__/pages-worker.spec.ts src/cloudflare/__tests__/pages-config-injection.spec.ts
for variant in main a1; do
  visible=false
  [[ "$variant" != a1 ]] || visible=true
  VITE_OPS_HOST_HEALTH_VISIBLE="$visible" "${PNPM[@]}" --dir "$out/source/frontend" run build
  cp -R "$out/source/backend/internal/web/dist" "$out/$variant"
done
cp -R "$out/main" "$out/3dbooks"
git -C "$ROOT" rev-parse "$base" > "$out/source-commit.txt"
