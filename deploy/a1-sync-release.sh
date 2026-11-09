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
bash "$ROOT/services/a1-user-sync-service/build.sh" "$work/a1-user-sync"
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
python3 - "$1" "$3" /etc/systemd/system/a1-user-sync.service /opt/a1-user-sync/config.yaml <<'PY'
import os,pathlib,re,stat,sys,tempfile
try:
    import yaml
except ImportError:
    raise SystemExit('python3-yaml is required; unit/config unchanged') from None
remote,stamp,unit_path,config_path=sys.argv[1:]
unit=pathlib.Path(unit_path)
config=pathlib.Path(config_path)
old=unit.read_text()
new,n=re.subn(r'/opt/a1-sync-release/[^\s:]+/a1-user-sync:/usr/local/bin/a1-user-sync:ro',lambda _: remote+'/a1-user-sync:/usr/local/bin/a1-user-sync:ro',old)
if n!=1:
    raise SystemExit('Unexpected service mount; unit/config unchanged')
try:
    data=yaml.compose(config.read_text(),Loader=yaml.SafeLoader)
except yaml.YAMLError:
    raise SystemExit('Invalid YAML configuration; unit/config unchanged') from None
if not isinstance(data,yaml.MappingNode):
    raise SystemExit('YAML configuration must be a mapping; unit/config unchanged')
# Preserve YAML scalar spelling and tags: constructing Python values would turn
# e.g. password: on or 012 into true or 10 before Go reads its string fields.
field='status_database_names'
matches=[i for i,(key,_) in enumerate(data.value)
         if isinstance(key,yaml.ScalarNode) and key.value==field]
if len(matches)>1:
    raise SystemExit('Duplicate status_database_names; unit/config unchanged')
replacement=yaml.SequenceNode('tag:yaml.org,2002:seq',
    [yaml.ScalarNode('tag:yaml.org,2002:str','sub2api_3dbooks')],flow_style=True)
if matches:
    key,_=data.value[matches[0]]
    if key.tag!='tag:yaml.org,2002:str':
        raise SystemExit('Invalid status_database_names key; unit/config unchanged')
    data.value[matches[0]]=(key,replacement)
else:
    data.value.append((yaml.ScalarNode('tag:yaml.org,2002:str',field),replacement))

def signature(node,active=None):
    active=set() if active is None else active
    if id(node) in active:
        raise SystemExit('Recursive YAML configuration; unit/config unchanged')
    active=active|{id(node)}
    if isinstance(node,yaml.ScalarNode):
        return ('scalar',node.tag,node.value,node.style)
    if isinstance(node,yaml.SequenceNode):
        return ('sequence',node.tag,node.flow_style,tuple(signature(n,active) for n in node.value))
    if isinstance(node,yaml.MappingNode):
        return ('mapping',node.tag,node.flow_style,
                tuple((signature(k,active),signature(v,active)) for k,v in node.value))
    raise SystemExit('Unsupported YAML node; unit/config unchanged')

expected=signature(data)
class PreservingDumper(yaml.SafeDumper):
    def choose_scalar_style(self):
        # The nodes came from valid YAML. PyYAML otherwise quotes plain flow
        # timestamps containing colons, changing their original scalar style.
        if self.event.style is None:
            return ''
        return super().choose_scalar_style()

try:
    text=yaml.serialize(data,Dumper=PreservingDumper,allow_unicode=True)
    actual=signature(yaml.compose(text,Loader=yaml.SafeLoader))
except yaml.YAMLError:
    raise SystemExit('YAML validation failed; unit/config unchanged') from None
if actual!=expected:
    raise SystemExit('YAML validation failed; unit/config unchanged')

temps=[]
def stage(path,content,metadata):
    fd,name=tempfile.mkstemp(prefix='.'+path.name+'.',dir=path.parent)
    temps.append(name)
    with os.fdopen(fd,'wb') as out:
        out.write(content)
        os.fchown(out.fileno(),metadata.st_uid,metadata.st_gid)
        os.fchmod(out.fileno(),stat.S_IMODE(metadata.st_mode))
        out.flush()
        os.fsync(out.fileno())
    return name

replaced=[]
try:
    prepared=[]
    for path,content in ((config,text.encode()),(unit,new.encode())):
        metadata=path.lstat()
        if not stat.S_ISREG(metadata.st_mode):
            raise SystemExit('Expected regular unit/config files; no changes made')
        original=stage(path,path.read_bytes(),metadata)
        updated=stage(path,content,metadata)
        prepared.append((path,original,updated))
    for path,original,_ in prepared:
        # Refuse to overwrite an existing backup for this release.
        os.link(original,str(path)+'.bak-'+stamp)
    for path,original,updated in prepared:
        os.replace(updated,path)
        replaced.append((path,original))
except BaseException:
    for path,original in reversed(replaced):
        os.replace(original,path)
    raise
finally:
    for name in temps:
        if os.path.exists(name):
            os.unlink(name)
PY
systemctl daemon-reload
systemctl restart a1-user-sync.service
systemctl is-active --quiet a1-user-sync.service
rm "$1"/chunk-* "$1/service.gz"
REMOTE
echo "a1-user-sync deployed: $remote (execution receipts remain the verification source)"
