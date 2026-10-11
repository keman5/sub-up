#!/usr/bin/env bash
set -euo pipefail
ssh -o BatchMode=yes -o ConnectTimeout=15 "${HOST:-51tokens}" python3 - <<'PY'
import json,subprocess
sql="""SELECT coalesce(json_agg(json_build_object('identity',md5(lower(btrim(u.email))),'id',u.id,'result',r.value::json,'request',q.value::json)),'[]'::json) FROM users u LEFT JOIN settings r ON r.key='a1_user_sync.result.'||u.id LEFT JOIN settings q ON q.key='a1_user_sync.request.'||u.id WHERE u.deleted_at IS NULL"""
def read(db):
    out=subprocess.check_output(['docker','exec','sub2api-postgres','psql','-U','sub2api','-d',db,'-At','-v','ON_ERROR_STOP=1','-c',sql],text=True)
    return json.loads(out)
source=read('sub2api_ap1')
by_id={u['id']:u for u in source}
for uid in (24,41,71):
    u=by_id[uid]
    assert u['result'] and not u['result']['enabled'], 'Expected cancelled user still synced'
    assert u['result']['updated_at']==u['request']['updated_at'], 'Cancellation not acknowledged'
print('a1: three reported cancellations have matching executed receipts')
for db in ('sub2api','sub2api_3dbooks'):
    target={u['identity']:u for u in read(db)}
    checked=0
    for u in source:
        if not u['result'] or u['identity'] not in target: continue
        result=target[u['identity']]['result']
        assert result, 'Missing mapped execution receipt in '+db
        assert result['user_id']==target[u['identity']]['id'], 'Wrong environment user ID'
        assert (result['enabled'],result['updated_at'])==(u['result']['enabled'],u['result']['updated_at']), 'Stale execution receipt in '+db
        checked+=1
    print(db+': '+str(checked)+' mapped execution receipts match a1')
subprocess.run(['systemctl','is-active','--quiet','a1-user-sync.service'],check=True)
print('sync consumer active')
PY
