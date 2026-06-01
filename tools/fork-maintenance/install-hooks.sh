#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PRE_COMMIT="$ROOT/.git/hooks/pre-commit"
POST_MERGE="$ROOT/.git/hooks/post-merge"
POST_REWRITE="$ROOT/.git/hooks/post-rewrite"

cat >"$PRE_COMMIT" <<'HOOK'
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
"$ROOT/tools/fork-maintenance/fork-maintenance.sh" check-doc
HOOK

cat >"$POST_MERGE" <<'HOOK'
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
"$ROOT/tools/fork-maintenance/fork-maintenance.sh" verify-after-upstream --skip-build || {
  echo "fork-maintenance: post-merge verification failed; inspect fork-only changes before deploying." >&2
  exit 0
}
HOOK

cat >"$POST_REWRITE" <<'HOOK'
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
"$ROOT/tools/fork-maintenance/fork-maintenance.sh" verify-after-upstream --skip-build || {
  echo "fork-maintenance: post-rewrite verification failed; inspect fork-only changes before deploying." >&2
  exit 0
}
HOOK

chmod +x "$PRE_COMMIT" "$POST_MERGE" "$POST_REWRITE"
echo "Installed fork-maintenance hooks:"
echo "  $PRE_COMMIT"
echo "  $POST_MERGE"
echo "  $POST_REWRITE"
