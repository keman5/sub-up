#!/usr/bin/env bash
set -euo pipefail
HOST="${HOST:-51tokens}"
ssh -o BatchMode=yes -o ConnectTimeout=15 "$HOST" bash -s <<'REMOTE'
set -euo pipefail
cd /opt/sub2api-ap1-deploy
backup="docker-compose.yml.bak-binary-mount-$(date +%Y%m%d%H%M%S)"
cp -p docker-compose.yml "$backup"
python3 - <<'PY'
from pathlib import Path
import re
path = Path('docker-compose.yml')
source = path.read_text()
pattern = r'^\s*- /opt/a1-sync-release/[^\s:]+/sub2api:/app/sub2api:ro\s*$'
lines = source.splitlines(keepends=True)
matches = [line for line in lines if re.fullmatch(pattern, line)]
if len(matches) != 1:
    raise SystemExit('Expected exactly one obsolete a1-sync-release binary mount; no changes made')
path.write_text(''.join(line for line in lines if line not in matches))
PY
docker compose config --format json | python3 -c 'import json,sys; c=json.load(sys.stdin); assert not any(v.get("target") == "/app/sub2api" for v in c["services"]["sub2api"].get("volumes", []))'
docker compose up -d --no-deps --pull never sub2api
container="$(docker compose ps -q sub2api)"
healthy=0
for attempt in $(seq 1 60); do
  if [[ "$(docker inspect -f '{{.State.Health.Status}}' "$container")" == healthy ]]; then healthy=1; break; fi
  sleep 2
done
[[ "$healthy" == 1 ]]
curl -fsS --max-time 15 http://127.0.0.1:8082/health
status="$(curl -sS --max-time 15 -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{"enabled":false}' http://127.0.0.1:8082/api/v1/admin/payment/subscriptions/595/subscription-sync)"
[[ "$status" == 401 ]] || { echo "Expected registered protected route (401), got $status" >&2; exit 1; }
echo "a1 binary mount removed; protected subscription route verified; backup=$backup"
REMOTE
