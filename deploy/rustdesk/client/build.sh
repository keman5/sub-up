#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
bash -n build.sh
ssh 51tokens 'bash -se' <<'REMOTE'
export DEBIAN_FRONTEND=noninteractive
package=/opt/rustdesk/client-1.5.3.deb
digest=81096d2f65fd43352540c351bca15bb0712a2a6f9efa4c95e9fff25cb97b3b81
if ! printf '%s  %s\n' "$digest" "$package" | sha256sum -c --status; then
  curl -fLsS --retry 2 --max-time 120 \
    https://github.com/rustdesk/rustdesk/releases/download/1.5.3/rustdesk-1.5.3-x86_64.deb \
    -o "$package"
fi
printf '%s  %s\n' "$digest" "$package" | sha256sum -c -
apt-get install -y /opt/rustdesk/client-1.5.3.deb
systemctl enable --now rustdesk
for attempt in $(seq 1 30); do
  client_id=$(rustdesk --get-id)
  test -z "$client_id" || break
  sleep 1
done
test -n "$client_id"
rustdesk --option custom-rendezvous-server 89.116.220.139:21116
rustdesk --option relay-server 89.116.220.139:21117
rustdesk --option key soyV+wsS5NVQUtmUNoNijrTuyc8sUJuX1lBia4x4iN0=
umask 077
if test -s /opt/rustdesk/client-access.txt; then
  client_password=$(sed -n 's/^Password: //p' /opt/rustdesk/client-access.txt)
else
  client_password=$(openssl rand -base64 24)
fi
test ${#client_password} -ge 20
password_result=$(rustdesk --password "$client_password")
test "$password_result" = 'Done!'
rustdesk --option verification-method use-permanent-password
rustdesk --option approve-mode password
printf 'Device ID: %s\nPassword: %s\n' "$client_id" "$client_password" > /opt/rustdesk/client-access.txt
chmod 600 /opt/rustdesk/client-access.txt
systemctl is-active rustdesk
printf 'Device ID: %s\n' "$client_id"
for option in custom-rendezvous-server relay-server key verification-method approve-mode; do
  printf '%s: ' "$option"
  rustdesk --option "$option"
done
test "$(rustdesk --option custom-rendezvous-server)" = '89.116.220.139:21116'
test "$(rustdesk --option relay-server)" = '89.116.220.139:21117'
test "$(rustdesk --option key)" = 'soyV+wsS5NVQUtmUNoNijrTuyc8sUJuX1lBia4x4iN0='
test "$(rustdesk --option verification-method)" = 'use-permanent-password'
test "$(rustdesk --option approve-mode)" = 'password'
REMOTE
install -d -m 700 "$HOME/.config/rustdesk-vps"
scp 51tokens:/opt/rustdesk/client-access.txt "$HOME/.config/rustdesk-vps/access.txt"
chmod 600 "$HOME/.config/rustdesk-vps/access.txt"
python3 - <<'PY'
import pathlib
import subprocess
from urllib.parse import quote

credentials = dict(line.split(': ', 1) for line in
                   (pathlib.Path.home() / '.config/rustdesk-vps/access.txt').read_text().splitlines())
device_id = credentials['Device ID']
password = credentials['Password']
assert device_id.isdigit() and len(password) >= 20
subprocess.Popen(['/Applications/RustDesk.app/Contents/MacOS/RustDesk',
                  '--connect', device_id, '--password', quote(password, safe='')],
                 stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                 stderr=subprocess.DEVNULL, start_new_session=True)
print('RustDesk connection requested for device', device_id)
PY
