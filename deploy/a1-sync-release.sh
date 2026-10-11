#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
HOST=${HOST:-51tokens}
if [[ "${1:-}" != --apply ]]; then
  echo 'Build and test locally, upload a1-user-sync, mirror statuses to main/3dbooks, restart existing service. Use --apply.'
  exit 0
fi
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
stamp=$(date +%Y%m%d%H%M%S)
remote="/opt/a1-sync-release/$stamp"
(cd "$ROOT/services/a1-user-sync-service" && GOTOOLCHAIN=local GOFLAGS= go test ./... && GOTOOLCHAIN=local GOFLAGS= CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "$work/a1-user-sync" ./cmd/a1-user-sync)
gzip -c "$work/a1-user-sync" > "$work/service.gz"
split -b 1m "$work/service.gz" "$work/chunk-"
SSH=(ssh -o BatchMode=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 "$HOST")
"${SSH[@]}" "mkdir -p '$remote'"
for chunk in "$work"/chunk-*; do
  uploaded=0
  for attempt in 1 2 3; do
    if "${SSH[@]}" "cat > '$remote/$(basename "$chunk")'" < "$chunk"; then uploaded=1; break; fi
  done
  [[ "$uploaded" == 1 ]]
done
hash=$(shasum -a 256 "$work/a1-user-sync" | cut -d ' ' -f 1)
"${SSH[@]}" bash -s -- "$remote" "$hash" "$stamp" <<'REMOTE'
set -euo pipefail
cat "$1"/chunk-* > "$1/service.gz"
gzip -t "$1/service.gz"
gzip -dc "$1/service.gz" > "$1/a1-user-sync"
echo "$2  $1/a1-user-sync" | sha256sum -c -
chmod 755 "$1/a1-user-sync"
cp /etc/systemd/system/a1-user-sync.service "/etc/systemd/system/a1-user-sync.service.bak-$3"
cp /opt/a1-user-sync/config.yaml "/opt/a1-user-sync/config.yaml.bak-$3"
python3 - "$1" <<'PY'
import pathlib,re,sys
unit=pathlib.Path('/etc/systemd/system/a1-user-sync.service')
old=unit.read_text()
new,n=re.subn(r'/opt/a1-sync-release/[^\s:]+/a1-user-sync:/usr/local/bin/a1-user-sync:ro',sys.argv[1]+'/a1-user-sync:/usr/local/bin/a1-user-sync:ro',old)
assert n==1, 'Unexpected service mount; no changes made'
config=pathlib.Path('/opt/a1-user-sync/config.yaml')
text=config.read_text()
line='status_database_names: [sub2api_3dbooks]\n'
text,n=re.subn(r'^status_database_names:.*\n',line,text,flags=re.M)
if not n: text+='\n'+line
config.write_text(text)
unit.write_text(new)
PY
systemctl daemon-reload
systemctl restart a1-user-sync.service
systemctl is-active --quiet a1-user-sync.service
rm "$1"/chunk-* "$1/service.gz"
REMOTE
echo "a1-user-sync deployed: $remote (execution receipts remain the verification source)"
