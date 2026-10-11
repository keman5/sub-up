#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
HOST=${HOST:-51tokens}
OUT=${A1_SYNC_OUTPUT:-/tmp/a1-sync-release}
[[ ${1:-} == --apply ]] || { echo 'Usage: bash deploy/a1-sync-release.sh --apply [--skip-build]'; exit 1; }
if [[ ${2:-} != --skip-build ]]; then bash "$ROOT/deploy/a1-sync-build.sh"; fi
ssh "$HOST" 'mkdir -p /opt/a1-sync-release/incoming'
scp "$OUT/a1-user-sync" "$OUT/sub2api.gz" "$ROOT/deploy/a1-sync-install.py" "$HOST:/opt/a1-sync-release/incoming/"
ssh "$HOST" 'python3 /opt/a1-sync-release/incoming/a1-sync-install.py'
