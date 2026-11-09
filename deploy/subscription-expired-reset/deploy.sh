#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
work="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-expired-reset.XXXXXX")"
trap 'rm -rf "$work"' EXIT
HOST="${HOST:-51tokens}"
PNPM=(npx --yes pnpm@9.15.9)
stamp="$(date +%Y%m%d%H%M%S)"
evidence="$ROOT/tmp/subscription-expired-reset-$stamp"
mkdir -p "$evidence"
for name in a1 3dbooks main; do
  "${PNPM[@]}" dlx wrangler pages deployment list --project-name "sub2api-frontend-$name" --json > "$evidence/$name-before.json"
  node --input-type=module - "$evidence/$name-before.json" <<'JS'
import fs from 'node:fs';
import assert from 'node:assert/strict';
const previous = JSON.parse(fs.readFileSync(process.argv[2], 'utf8')).find(d => d.Environment === 'Production' && d.Branch === 'subapi');
assert.equal(previous?.Source, '9fddec9', 'Pages baseline changed; inspect before publishing');
console.log(`Previous deployment: ${previous.Id}`);
JS
done
images="$(ssh -o BatchMode=yes -o ConnectTimeout=15 "$HOST" 'docker inspect -f "{{.Config.Image}}" sub2api-ap1 sub2api-3dbooks sub2api')"
[[ "$(printf '%s\n' "$images" | sort -u)" == sub2api:subapi-9fddec9ff-all-20261011205921 ]]
bash deploy/subscription-expired-reset/build.sh "$work"
for name in a1 3dbooks main; do
  case "$name" in
    a1) origin=https://ap1.upit.top; site=https://a1.upit.top ;;
    3dbooks) origin=https://api.3dbooks.top; site=https://ai.3dbooks.top ;;
    main) origin=https://api.upit.top; site=https://ai.upit.top ;;
  esac
  node scripts/inject-pages-public-settings.mjs --settings-url "$origin/api/v1/settings/public" --html "$work/$name/index.html"
  bundle="$(rg -l 'reset-quota-scope' "$work/$name/assets/"*.js)"
  [[ -f "$bundle" ]]
  expected="$(shasum -a 256 "$bundle" | cut -d ' ' -f 1)"
  "${PNPM[@]}" dlx wrangler pages deploy "$work/$name" --project-name "sub2api-frontend-$name" --branch subapi --commit-hash "$(cat "$work/source-commit.txt")" --commit-message 'Show reset quota for expired subscriptions (isolated frontend fix)' --commit-dirty=true
  curl -fsS --retry 5 --retry-delay 3 --max-time 20 "$site/health" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"] == "ok"'
  curl -fsS --retry 5 --retry-delay 3 --max-time 30 "$site/assets/$(basename "$bundle")" -o "$evidence/$name-subscriptions.js"
  [[ "$(shasum -a 256 "$evidence/$name-subscriptions.js" | cut -d ' ' -f 1)" == "$expected" ]]
  entry="$(node -e 'const s=require("fs").readFileSync(process.argv[1],"utf8"); const m=s.match(/<script[^>]+src="([^"]+)"/); if(!m)process.exit(1); console.log(m[1])' "$work/$name/index.html")"
  curl -fsS --retry 5 --retry-delay 3 --max-time 30 "$site/" -o "$evidence/$name-index.html"
  rg -Fq "$entry" "$evidence/$name-index.html"
  printf '%s %s\n' "$name" "$expected" >> "$evidence/verified-bundles.txt"
  "${PNPM[@]}" dlx wrangler pages deployment list --project-name "sub2api-frontend-$name" --json > "$evidence/$name-after.json"
done
echo "Verified Pages deployments; evidence: $evidence"
