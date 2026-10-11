#!/usr/bin/env bash
set -euo pipefail
ssh -o BatchMode=yes -o ConnectTimeout=15 "${HOST:-51tokens}" bash -s <<'REMOTE'
set -euo pipefail
books=/opt/sub2api-retired/sub2api-3dbooks-deploy-20260918151421
test_dir=/opt/sub2api-test-deploy
backup=/opt/sub2api-retirement-backup-$(date +%Y%m%d%H%M%S)
mkdir -m 700 "$backup"
cp "$books/docker-compose.yml" "$backup/3dbooks-compose.yml"
cp /opt/cf-origin-ssl/Caddyfile "$backup/Caddyfile"
tar -czf "$backup/test-deploy.tar.gz" -C /opt sub2api-test-deploy
docker exec sub2api-postgres pg_dump -U sub2api -Fc sub2api_test > "$backup/test.dump"
test -s "$backup/test.dump"
# Validate identity before deleting the old environment's dedicated data.
python3 - <<'PY'
import json,subprocess
for name,db,redis in [('sub2api-test','sub2api_test','2'),('sub2api-3dbooks','sub2api_3dbooks','3')]:
    d=json.loads(subprocess.check_output(['docker','inspect',name]))[0]
    env=dict(e.split('=',1) for e in d['Config']['Env'])
    assert env['DATABASE_DBNAME']==db and env['REDIS_DB']==redis, name
PY
docker stop sub2api-test
python3 - <<'PY'
from pathlib import Path
p=Path('/opt/cf-origin-ssl/Caddyfile')
s=p.read_text()
start=s.index('a2t.upit.top:443, test.51tokens.upit.top:443 {')
depth=0
for i in range(s.index('{',start),len(s)):
    if s[i]=='{': depth+=1
    elif s[i]=='}': depth-=1
    if depth==0:
        s=s[:start]+s[i+1:]
        break
assert 'a2t.upit.top' not in s and 'test.51tokens.upit.top' not in s
assert s.count('127.0.0.1:8084')==3
p.write_text(s.replace('127.0.0.1:8084','127.0.0.1:8083'))
p=Path('/opt/sub2api-retired/sub2api-3dbooks-deploy-20260918151421/docker-compose.yml')
s=p.read_text()
assert s.count('127.0.0.1:8084:8080')==1
p.write_text(s.replace('127.0.0.1:8084:8080','127.0.0.1:8083:8080'))
PY
docker exec cf-origin-ssl caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
cd "$books"
docker compose up -d --no-deps --pull never sub2api-3dbooks
for attempt in $(seq 1 40); do
  if curl -fsS --max-time 5 http://127.0.0.1:8083/health | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"]=="ok"'; then break; fi
  [[ "$attempt" -lt 40 ]] || exit 1
  sleep 2
done
docker exec cf-origin-ssl caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
curl -fsS --retry 3 --max-time 20 https://api.3dbooks.top/health
docker rm sub2api-test
docker exec sub2api-postgres psql -U sub2api -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE sub2api_test WITH (FORCE);'
docker exec sub2api-redis sh -c 'env -u REDISCLI_AUTH redis-cli -n 2 FLUSHDB'
rm -rf "$test_dir" /opt/sub2api-test-offline-backup-20261009 /opt/sub2api-ap2-deploy.archived-20260706083857
echo "Retired test; 3dbooks now uses 8083. Recovery backup: $backup"
REMOTE
