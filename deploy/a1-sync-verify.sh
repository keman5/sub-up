#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
HOST=${HOST:-51tokens}
scp "$ROOT/deploy/a1-sync-verify.py" "$HOST:/opt/a1-sync-release/incoming/"
ssh "$HOST" 'python3 /opt/a1-sync-release/incoming/a1-sync-verify.py'
