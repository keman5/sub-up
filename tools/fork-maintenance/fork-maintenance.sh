#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cleanup_python_cache() {
  rm -rf "$ROOT/tools/fork-maintenance/__pycache__"
}
trap cleanup_python_cache EXIT

exec python3 "$ROOT/tools/fork-maintenance/fork_maintenance.py" "$@"
