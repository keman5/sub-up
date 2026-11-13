# RustDesk Server OSS

Official documentation: https://rustdesk.com/docs/en/self-host/rustdesk-server-oss/docker/

Run `bash deploy/rustdesk/build.sh check` or `bash deploy/rustdesk/build.sh deploy` locally.
Deployment uses the official prebuilt 1.1.16 image pinned by digest, with no VPS build.
The SSH target is `51tokens`; persistent server data and keys live in `/opt/rustdesk/data`.
Keep the private key on the VPS and back up this directory securely before migrating.
The deploy command backs up an existing installation before replacing its configuration.

Set both RustDesk clients under Settings → Network → ID/Relay server:

- ID server: `89.116.220.139:21116`
- Relay server: `89.116.220.139:21117`
- Key: the content of `/opt/rustdesk/data/id_ed25519.pub` (also printed by deployment).
- API server: leave empty (OSS has no management console).

TCP 21115–21117 and UDP 21116 are published. WebSocket ports 21118/21119
are not published. All clients must use the server key. Direct connections are
preferred; the relay handles connections that cannot connect directly.

Check the server with `ssh 51tokens 'cd /opt/rustdesk && docker compose ps && docker compose logs --tail=30'`.
Run `python3 deploy/rustdesk/verify.py <server-public-key>` locally to verify TCP NAT
responses, UDP registration, rejection of an invalid relay key, and authenticated
bidirectional relay traffic. It does not register a device key or open a desktop session.
Stop it with `ssh 51tokens 'cd /opt/rustdesk && docker compose down'`; data is retained.

## VPS desktop client

Run `bash deploy/rustdesk/client/build.sh` on the Mac to install the official,
SHA-256-verified RustDesk 1.5.3 Debian package, enable its system service, and
configure the same server and key. No compilation is performed. The script
reuses its generated unattended password on later runs, checks the live options,
and requests a connection in the installed Mac client.

VPS device ID: `17230704`.
Credentials: `/opt/rustdesk/client-access.txt` on the VPS and
`~/.config/rustdesk-vps/access.txt` on the Mac (both mode 600).
These files contain the unattended password; do not add them to Git.
RustDesk reaches the existing LightDM login screen; an operating-system login
is still required to enter a user desktop.

Installation, service startup, live settings, and encrypted TCP connectivity
were verified. Desktop authentication and screen reception were not verified;
native UI automation timed out, and the connection did not complete authentication.
