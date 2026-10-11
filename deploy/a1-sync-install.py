#!/usr/bin/env python3
"""Install locally built binaries; retain existing images, configuration and rollback files."""
import datetime
import gzip
import pathlib
import re
import shutil
import subprocess
import time
import yaml

root = pathlib.Path('/opt/a1-sync-release')
stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%d%H%M%S')
release = root / stamp
release.mkdir()
backup = release / 'backups'
backup.mkdir()
shutil.copy2(root / 'incoming/a1-user-sync', release / 'a1-user-sync')
with gzip.open(root / 'incoming/sub2api.gz', 'rb') as source, (release / 'sub2api').open('wb') as target:
    shutil.copyfileobj(source, target)
for name in ('sub2api', 'a1-user-sync'):
    (release / name).chmod(0o755)

compose = pathlib.Path('/opt/sub2api-ap1-deploy/docker-compose.yml')
unit = pathlib.Path('/etc/systemd/system/a1-user-sync.service')
shutil.copy2(compose, backup / 'docker-compose.yml')
shutil.copy2(unit, backup / 'a1-user-sync.service')
timer_active = subprocess.run(['systemctl','is-active','--quiet','a1-subscription-sync.timer']).returncode == 0
(backup / 'subscription-timer-active').write_text(str(timer_active))
cfg = yaml.safe_load(compose.read_text())
service = cfg['services']['sub2api']
volumes = service.setdefault('volumes', [])
volumes[:] = [v for v in volumes if not (isinstance(v, str) and ':/app/sub2api' in v)]
volumes.append(str(release / 'sub2api') + ':/app/sub2api:ro')
lines = unit.read_text().splitlines()
for i, line in enumerate(lines):
    if line.startswith('ExecStart='):
        if ' --env-file ' not in line:
            raise RuntimeError('Unexpected a1-user-sync service definition')
        line = re.sub(r' -v /opt/a1-sync-release/[^ ]+:/usr/local/bin/a1-user-sync:ro', '', line)
        lines[i] = line.replace(' --env-file ', f' -v {release}/a1-user-sync:/usr/local/bin/a1-user-sync:ro --env-file ', 1)

def run(*args):
    subprocess.run(args, check=True)

try:
    compose.write_text(yaml.safe_dump(cfg, sort_keys=False))
    unit.write_text('\n'.join(lines) + '\n')
    run('docker', 'compose', '-f', str(compose), 'config', '--quiet')
    run('docker', 'compose', '-f', str(compose), 'up', '-d', 'sub2api')
    for _ in range(45):
        state = subprocess.check_output(['docker','inspect','-f','{{.State.Health.Status}}','sub2api-ap1'], text=True).strip()
        if state == 'healthy':
            break
        time.sleep(2)
    else:
        raise RuntimeError('a1 application failed health gate')
    run('curl', '-fsS', '--max-time', '15', 'http://127.0.0.1:8082/health')
    # The old script replaces subscriptions independently and would bypass cancellation.
    run('systemctl', 'stop', 'a1-subscription-sync.timer')
    run('systemctl', 'stop', 'a1-subscription-sync.service')
    run('systemctl', 'disable', 'a1-subscription-sync.timer')
    run('systemctl', 'daemon-reload')
    run('systemctl', 'restart', 'a1-user-sync')
    time.sleep(8)
    run('systemctl', 'is-active', '--quiet', 'a1-user-sync')
    running = subprocess.check_output(['docker','inspect','-f','{{.State.Running}}','a1-user-sync'], text=True).strip()
    if running != 'true':
        raise RuntimeError('a1-user-sync container failed')
except Exception:
    shutil.copy2(backup / 'docker-compose.yml', compose)
    shutil.copy2(backup / 'a1-user-sync.service', unit)
    run('systemctl', 'daemon-reload')
    run('docker','compose','-f',str(compose),'up','-d','sub2api')
    run('systemctl','restart','a1-user-sync')
    if timer_active:
        run('systemctl','enable','--now','a1-subscription-sync.timer')
    raise
print('\nrelease=' + str(release))
print('rollback_files=' + str(backup))
