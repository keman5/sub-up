#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bash -n "$ROOT/deploy/deploy-all.sh"
bash -n "$ROOT/deploy/build.sh"
bash -n "$ROOT/services/a1-user-sync-service/build.sh"
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
python3 - "$ROOT/deploy/deploy-all.sh" <<'PY'
import os, pathlib, subprocess, sys, tempfile

source = pathlib.Path(sys.argv[1]).read_text()
# Include the parent legacy-backup guard, not just the private backup block.
legacy = next(line for line in source.splitlines() if 'cp "$override" "$override.bak-$stamp"' in line)
# Execute the production backup block; redirect only its absolute backup root.
block = source.split('umask 077\n', 1)[1].split('# Preserve existing override settings', 1)[0]
assert block.count('/opt/sub2api-release-backups') == 1
block = legacy + '\numask 077\n' + block.replace('/opt/sub2api-release-backups', '${BACKUP_ROOT}')
stub = r'''
set -euo pipefail
stamp=fixture
name=a1
override=docker-compose.override.yml
docker() {
  case "$*" in
    *pg_dump*)
      printf 'dump\n' >> "$CALL_LOG"
      printf 'valid-dump'
      [[ "$STUB_MODE" != dump-fail ]]
      ;;
    *'pg_restore --list'*)
      printf 'restore\n' >> "$CALL_LOG"
      archive="$(cat)"
      [[ "$STUB_MODE" != archive-fail && "$archive" == valid-dump ]]
      ;;
    *) echo 'Unexpected docker call in backup test' >&2; return 99 ;;
  esac
}
'''
with tempfile.TemporaryDirectory(prefix='sub2api-backup-test-') as directory:
    root = pathlib.Path(directory)

    def run(case, mode):
        case.mkdir(exist_ok=True)
        for filename in ('docker-compose.yml', 'docker-compose.override.yml'):
            if not (case / filename).exists():
                (case / filename).write_text('original-' + filename)
        log = case / 'calls'
        log.write_text('')
        marker = case / 'switch-reached'
        marker.unlink(missing_ok=True)
        env = dict(os.environ, BACKUP_ROOT=str(case / 'backups'), CALL_LOG=str(log), STUB_MODE=mode)
        result = subprocess.run(['bash', '-s', '--', '', '', '', '', '', '', 'sub2api_ap1'],
                                input=stub + block + '\nprintf switched > switch-reached\n',
                                cwd=case, env=env, text=True, capture_output=True)
        return result, case / 'backups/fixture/a1', log.read_text().splitlines(), marker

    for mode in ('dump-fail', 'archive-fail'):
        result, backup, calls, marker = run(root / mode, mode)
        assert result.returncode != 0, (mode, result.stdout, result.stderr)
        assert not (backup / 'database.dump').exists(), mode
        assert not marker.exists(), mode
        assert calls == (['dump'] if mode == 'dump-fail' else ['dump', 'restore']), calls

    case = root / 'success'
    result, backup, calls, marker = run(case, 'ok')
    assert result.returncode == 0, (result.stdout, result.stderr)
    assert calls == ['dump', 'restore', 'restore'], calls
    assert marker.exists() and (backup / 'database.dump').read_text() == 'valid-dump'
    assert not (backup / 'database.dump.part').exists()
    originals = {p.name: (p.read_bytes(), p.stat().st_mtime_ns) for p in backup.iterdir()}
    legacy_backup = case / 'docker-compose.override.yml.bak-fixture'
    assert legacy_backup.read_text() == 'original-docker-compose.override.yml'
    legacy_original = (legacy_backup.read_bytes(), legacy_backup.stat().st_mtime_ns)
    assert (backup.stat().st_mode & 0o777) == 0o700
    assert (backup / 'database.dump').stat().st_mode & 0o777 == 0o600
    for filename in ('docker-compose.yml', 'docker-compose.override.yml'):
        (case / filename).write_text('changed-live-config')
    result, backup, calls, marker = run(case, 'ok')
    assert result.returncode == 0 and marker.exists(), (result.stdout, result.stderr)
    assert calls == ['restore'], calls
    assert {p.name: (p.read_bytes(), p.stat().st_mtime_ns) for p in backup.iterdir()} == originals
    assert (legacy_backup.read_bytes(), legacy_backup.stat().st_mtime_ns) == legacy_original
    # A finalized but unreadable archive must also block a retry without a new dump.
    (backup / 'database.dump').write_text('corrupt-archive')
    result, backup, calls, marker = run(case, 'ok')
    assert result.returncode != 0 and not marker.exists()
    assert calls == ['restore'], calls
    assert (backup / 'database.dump').read_text() == 'corrupt-archive'
    # The same actual guard must allow absent overrides and stop on copy errors.
    for exists in (False, True):
        guard_case = root / ('legacy-exists' if exists else 'legacy-absent')
        guard_case.mkdir()
        if exists:
            (guard_case / 'docker-compose.override.yml').write_text('original')
        script = ('set -euo pipefail\noverride=docker-compose.override.yml\nstamp=fixture\n'
                  'cp() { return 23; }\n' + legacy + '\nprintf guard-passed\n')
        result = subprocess.run(['bash', '-c', script], cwd=guard_case, text=True, capture_output=True)
        assert (result.returncode == 0) == (not exists), result.stderr
        assert ('guard-passed' in result.stdout) == (not exists)
        assert not (guard_case / 'docker-compose.override.yml.bak-fixture').exists()
print('Real shell backup stubs passed: legacy first-save/retry/copy failure, dump/archive failure, final archive gate.')
PY
python3 - "$ROOT/deploy/build.sh" <<'PY'
import os, pathlib, subprocess, sys

source = pathlib.Path(sys.argv[1]).read_text()
guard = 'endpoint=' + source.split('endpoint=', 1)[1].split('go clean -cache', 1)[0]
build = next(line.strip() for line in source.splitlines() if line.strip().startswith('docker buildx build '))
stub = r'''
set -euo pipefail
image=fixture-image
work=fixture-work
docker() {
  case "$1 $2" in
    'context inspect') printf '%s\n' "$STUB_ENDPOINT" ;;
    'buildx build')
      [[ "$3 $4" == '--builder default' ]]
      [[ "$DOCKER_HOST" == "$EXPECTED_SOCKET" ]]
      [[ ! ${DOCKER_CONTEXT+x} && ! ${BUILDX_BUILDER+x} && ! ${BUILDKIT_HOST+x} ]]
      [[ ! ${BUILDX_CONFIG+x} && ! ${DOCKER_TLS_VERIFY+x} && ! ${DOCKER_CERT_PATH+x} ]]
      printf 'local-default-build\n'
      ;;
    *) return 99 ;;
  esac
}
'''
# An invalid remote builder is deliberately present in every scenario. Accepted
# builds must clear it and explicitly select default, without calling any daemon.
cases = [
    ('ssh://remote-context', 'unix:///tmp/local.sock', False),
    ('unix:///tmp/context.sock', 'tcp://remote-builder:2375', False),
    ('ssh://remote-context', '', False),
    ('unix:///tmp/context.sock', '', True),
    ('unix:///tmp/context.sock', 'unix:///tmp/explicit.sock', True),
]
for endpoint, host, allowed in cases:
    env = dict(os.environ, STUB_ENDPOINT=endpoint, DOCKER_CONTEXT='fixture-context',
               BUILDX_BUILDER='invalid-remote-builder', BUILDKIT_HOST='tcp://remote:1234',
               BUILDX_CONFIG='/nonexistent/remote-buildx', DOCKER_TLS_VERIFY='1',
               DOCKER_CERT_PATH='/nonexistent/remote-certs', EXPECTED_SOCKET=host or endpoint)
    env.pop('DOCKER_HOST', None)
    if host:
        env['DOCKER_HOST'] = host
    result = subprocess.run(['bash', '-s'], input=stub + guard + build + '\n',
                            env=env, text=True, capture_output=True)
    assert (result.returncode == 0) == allowed, (endpoint, host, result.stdout, result.stderr)
    assert ('local-default-build' in result.stdout) == allowed, (endpoint, host, result.stdout)
print('Docker locality shell stubs passed: context/host conflicts rejected; remote builder overrides cleared.')
PY
echo 'Deployment syntax, target plan, remote local-build boundary and health gates passed.'
