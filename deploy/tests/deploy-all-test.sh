#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bash -n "$ROOT/deploy/deploy-all.sh"
bash -n "$ROOT/deploy/a1-sync-release.sh"
bash -n "$ROOT/deploy/check-subscription-sync.sh"
bash -n "$ROOT/deploy/verify-subscription-sync.sh"
bash -n "$ROOT/deploy/fix-a1-binary-mount.sh"
bash -n "$ROOT/deploy/retire-test-move-3dbooks.sh"
node --check "$ROOT/deploy/remove-test-pages.mjs"
plan="$(bash "$ROOT/deploy/deploy-all.sh" --dry-run)"
for target in a1 3dbooks main; do
  [[ "$plan" == *"sub2api-frontend-$target"* ]]
done
[[ "$plan" == *'/sub2api-3dbooks-deploy-20260918151421 / sub2api-3dbooks / 8083'* ]]
[[ "$plan" != *'sub2api-frontend-test'* ]]
if bash "$ROOT/deploy/deploy-all.sh" --unknown >/dev/null 2>&1; then exit 1; fi
python3 - "$ROOT/deploy/deploy-all.sh" <<'PY'
import pathlib, subprocess, sys, tempfile
source=pathlib.Path(sys.argv[1]).read_text()
for block in source.split("<<'REMOTE'\n")[1:]:
    remote=block.split('\nREMOTE',1)[0]
    subprocess.run(['bash','-n'], input=remote, text=True, check=True)
    assert 'docker build' not in remote and 'go build' not in remote
assert 'docker compose -f docker-compose.yml -f "$override" up -d --no-deps --pull never' in source
assert 'sub2api-test' not in source and 'a2t.upit.top' not in source
assert 'Remove the obsolete /app/sub2api bind mount before deploying' in source
assert source.index('curl -fsS --retry 3') < source.index('wrangler pages deploy')
retirement=pathlib.Path(sys.argv[1]).with_name('retire-test-move-3dbooks.sh').read_text()
remote=retirement.split("<<'REMOTE'\n",1)[1].split('\nREMOTE',1)[0]
subprocess.run(['bash','-n'], input=remote, text=True, check=True)
assert retirement.index('pg_dump') < retirement.index('DROP DATABASE')
assert 'FLUSHALL' not in retirement
repair=pathlib.Path(sys.argv[1]).with_name('fix-a1-binary-mount.sh').read_text()
remote=repair.split("<<'REMOTE'\n",1)[1].split('\nREMOTE',1)[0]
subprocess.run(['bash','-n'], input=remote, text=True, check=True)
python=repair.split("<<'PY'\n",1)[1].split('\nPY',1)[0]
with tempfile.TemporaryDirectory() as directory:
    compose=pathlib.Path(directory)/'docker-compose.yml'
    fixture='services:\n  sub2api:\n    volumes:\n    - ./data:/app/data\n    - /opt/a1-sync-release/20261011015401/sub2api:/app/sub2api:ro\n    - /run/sub2api-ops:/run/sub2api-ops:ro\n'
    compose.write_text(fixture)
    subprocess.run([sys.executable,'-c',python],cwd=directory,check=True)
    assert compose.read_text() == fixture.replace('    - /opt/a1-sync-release/20261011015401/sub2api:/app/sub2api:ro\n','')
    before=compose.read_text()
    assert subprocess.run([sys.executable,'-c',python],cwd=directory,capture_output=True).returncode != 0
    assert compose.read_text() == before
PY
echo 'Deployment syntax, target plan, remote local-build boundary and health gates passed.'
