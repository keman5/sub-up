#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

check() {
  bash -n build.sh
  docker compose -f compose.yaml config --quiet
  docker compose -f compose.yaml config --format json | ruby -rjson -e '
    services = JSON.parse(STDIN.read).fetch("services")
    abort "expected ID and relay services" unless services.keys.sort == %w[hbbr hbbs]
    ports = services.values.flat_map { |s| s.fetch("ports").map { |p| [p["published"].to_i, p["target"], p["protocol"]] } }.sort
    abort "unexpected exposed ports" unless ports == [[21115,21115,"tcp"],[21116,21116,"tcp"],[21116,21116,"udp"],[21117,21117,"tcp"]]
    services.each_value do |s|
      abort "image must be pinned" unless s.fetch("image").include?("@sha256:")
      abort "key enforcement required" unless s.fetch("command").last(2) == ["-k", "_"]
      abort "data must persist" unless s.fetch("volumes").any? { |v| v["target"] == "/root" && v["type"] == "bind" }
    end
    puts "RustDesk configuration checks passed"
  '
}

case "${1:-check}" in
  check) check ;;
  deploy)
    check
    ssh 51tokens 'umask 077
      install -d -m 700 /opt/rustdesk /opt/rustdesk/data
      if test -f /opt/rustdesk/compose.yaml; then
        tar -czf /opt/rustdesk-backup-$(date +%Y%m%d%H%M%S).tgz -C /opt rustdesk
      fi'
    scp compose.yaml build.sh 51tokens:/opt/rustdesk/
    ssh 51tokens 'bash -se' <<'REMOTE'
cd /opt/rustdesk
chmod 700 build.sh data
chmod 600 compose.yaml
docker compose config --quiet
docker compose pull
docker compose up -d hbbs
for attempt in $(seq 1 30); do
  test ! -s data/id_ed25519.pub || break
  sleep 1
done
test -s data/id_ed25519.pub
chmod 600 data/id_ed25519 data/id_ed25519.pub
docker compose up -d hbbr
docker compose ps
printf 'Server public key: '
cat data/id_ed25519.pub
printf '\n'
REMOTE
    ;;
  *) echo "Usage: bash build.sh [check|deploy]" >&2; exit 2 ;;
esac
