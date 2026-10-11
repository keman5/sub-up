#!/usr/bin/env python3
"""End-to-end VPS check using a disposable subscription user; never print credentials or keys."""
import base64
import hashlib
import hmac
import json
import pathlib
import subprocess
import time
import urllib.request
import uuid
import yaml

cfg = yaml.safe_load(pathlib.Path('/opt/a1-user-sync/config.yaml').read_text())
source_db = cfg['source_database']['dbname']
target_db = cfg['target_database']['dbname']

def sql(db, statement):
    result = subprocess.run(['docker','exec','-i','-e','SYNC_DB='+db,'sub2api-postgres','sh','-c',
        'exec psql -U "$POSTGRES_USER" -d "$SYNC_DB" -X -qAt -v ON_ERROR_STOP=1'],
        input=statement, text=True, capture_output=True)
    if result.returncode:
        # SQL can contain an API key; report only PostgreSQL's error line.
        raise RuntimeError('\n'.join(line for line in result.stderr.splitlines() if line.startswith(('ERROR:', 'DETAIL:'))))
    return result.stdout.strip()

def quote(value):
    return "'" + str(value).replace("'", "''") + "'"

def token(email):
    user = json.loads(sql(source_db, 'SELECT json_build_object(\'id\',id,\'email\',email,\'password_hash\',password_hash,\'role\',role) FROM users WHERE deleted_at IS NULL AND lower(email)='+quote(email)))
    env = json.loads(subprocess.check_output(['docker','inspect','-f','{{json .Config.Env}}','sub2api-ap1'], text=True))
    env = dict(item.split('=',1) for item in env)
    secret = env.get('JWT_SECRET')
    if not secret:
        raise RuntimeError('JWT_SECRET missing; use an authenticated session to verify the API')
    version = int.from_bytes(hashlib.sha256((user['email'].strip().lower()+'\n'+user['password_hash']).encode()).digest()[:8], 'big') & 0x7fffffffffffffff
    def encode(value):
        return base64.urlsafe_b64encode(json.dumps(value,separators=(',',':')).encode()).rstrip(b'=')
    now = int(time.time())
    body = encode({'alg':'HS256','typ':'JWT'}) + b'.' + encode({'user_id':user['id'],'email':user['email'],'role':user['role'],'token_version':version,'iat':now,'nbf':now,'exp':now+300})
    signature = base64.urlsafe_b64encode(hmac.new(secret.encode(),body,hashlib.sha256).digest()).rstrip(b'=')
    return (body+b'.'+signature).decode()

bearer = token('macseek@upit.top')
suffix = uuid.uuid4().hex[:16]
email = 'codex-sync-'+suffix+'@example.invalid'
key = 'sk-codex-verification-'+uuid.uuid4().hex
user_id = None

def api(order, enabled):
    request = urllib.request.Request('http://127.0.0.1:8082/api/v1/admin/payment/orders/'+str(order)+'/subscription-sync',
        data=json.dumps({'enabled':enabled}).encode(),headers={'Authorization':'Bearer '+bearer,'Content-Type':'application/json'})
    with urllib.request.urlopen(request,timeout=20) as response:
        data = json.load(response)
    assert data['code'] == 0, data
    return data['data']

def wait_result(enabled):
    for _ in range(20):
        value = sql(source_db, "SELECT value FROM settings WHERE key="+quote('a1_user_sync.result.'+str(user_id)))
        request = sql(source_db, "SELECT value FROM settings WHERE key="+quote('a1_user_sync.request.'+str(user_id)))
        if value and request and json.loads(value)['enabled'] == enabled and json.loads(value)['updated_at'] == json.loads(request)['updated_at']:
            return
        time.sleep(2)
    raise RuntimeError('notification was not consumed')

def snapshot():
    return sql(target_db, "SELECT json_build_object('notes',u.notes,'subscriptions',(SELECT json_agg(json_build_object('group',group_id,'expires',expires_at,'daily',daily_usage_usd)) FROM user_subscriptions WHERE user_id=u.id AND deleted_at IS NULL),'keys',(SELECT json_agg(json_build_object('name',name,'status',status,'quota',quota)) FROM api_keys WHERE user_id=u.id)) FROM users u WHERE email="+quote(email))

try:
    source_groups = json.loads(sql(source_db,"SELECT coalesce(json_agg(json_build_object('id',id,'name',name,'platform',platform)),'[]'::json) FROM groups WHERE deleted_at IS NULL"))
    target_groups = json.loads(sql(target_db,"SELECT coalesce(json_agg(json_build_object('name',name,'platform',platform)),'[]'::json) FROM groups WHERE deleted_at IS NULL"))
    group = next(g for g in source_groups if sum(t['name']==g['name'] and t['platform']==g['platform'] for t in target_groups)==1)
    user_id = int(sql(source_db, "INSERT INTO users(email,password_hash,username,notes,role,status,balance,concurrency,signup_source) VALUES("+quote(email)+",'disabled-test-hash','codex-sync-verification','before-sync','user','inactive',0,1,'email') RETURNING id"))
    sql(source_db, f"INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,status) VALUES({user_id},{group['id']},now()-interval '1 hour',now()+interval '1 day','active'); INSERT INTO api_keys(user_id,group_id,key,name,status,quota) VALUES({user_id},{group['id']},{quote(key)},'verification-key','inactive',1);")
    order = int(sql(source_db, f"INSERT INTO payment_orders(user_id,user_email,user_name,amount,pay_amount,fee_rate,recharge_code,out_trade_no,payment_type,payment_trade_no,order_type,status,expires_at,client_ip,src_host) VALUES({user_id},{quote(email)},'codex-sync-verification',1,1,0,{quote(suffix)},{quote('codex-sync-'+suffix)},'alipay','','subscription','COMPLETED',now()+interval '1 hour','127.0.0.1','verification') RETURNING id"))
    api(order,True)
    wait_result(True)
    before = snapshot()
    state = json.loads(before)
    assert state['notes']=='before-sync' and len(state['subscriptions'])==1 and state['keys'][0]['quota']==1, 'user/subscription/key sync mismatch'
    print('PASS: authenticated sync request -> consumer -> primary user/subscription/API token')
    api(order,False)
    wait_result(False)
    sql(source_db, f"UPDATE users SET notes='after-cancel' WHERE id={user_id}; UPDATE user_subscriptions SET daily_usage_usd=0.123 WHERE user_id={user_id}; UPDATE api_keys SET quota=2 WHERE user_id={user_id};")
    # A forced periodic round with the user still in the legacy allowlist must also honor cancellation.
    verify_cfg = dict(cfg, user_ids=[user_id], sync_subscription_user_ids=[user_id], remove_user_ids=[])
    private = pathlib.Path('/opt/a1-sync-release/incoming/verify-config.yaml')
    private.write_text(yaml.safe_dump(verify_cfg)); private.chmod(0o600)
    subprocess.run(['docker','cp',str(private),'a1-user-sync:/tmp/verify-config.yaml'],check=True,capture_output=True)
    private.unlink()
    subprocess.run(['docker','exec','a1-user-sync','/usr/local/bin/a1-user-sync','-once','-config','/tmp/verify-config.yaml'],check=True,capture_output=True)
    subprocess.run(['docker','exec','a1-user-sync','rm','/tmp/verify-config.yaml'],check=True)
    time.sleep(7)
    assert snapshot()==before, 'cancelled user was copied again'
    print('PASS: cancellation blocks user/subscription/token updates, including legacy allowlist')
    api(order,True)
    # Wait for the latest request timestamp, rather than accepting the previous enabled acknowledgement.
    for _ in range(20):
        if snapshot() and json.loads(snapshot())['notes']=='after-cancel':
            break
        time.sleep(2)
    else:
        raise RuntimeError('re-enable did not resume synchronization')
    assert json.loads(snapshot())['keys'][0]['quota']==2
    print('PASS: re-enable resumes synchronization')
    sql(source_db, f"UPDATE api_keys SET deleted_at=now() WHERE user_id={user_id};")
    api(order,True)
    wait_result(True)
    revoked = sql(target_db, "SELECT count(*) FROM api_keys WHERE user_id IN(SELECT id FROM users WHERE email="+quote(email)+") AND status='inactive' AND deleted_at IS NOT NULL")
    assert revoked=='1', 'deleted source token was not revoked in primary'
    print('PASS: deleted source API token is revoked in primary')
    api(order,False)
    wait_result(False)
finally:
    if user_id is not None:
        # Cancellation is persisted before cleanup so any in-flight worker releases its row lock first.
        sql(source_db, "UPDATE settings SET value=jsonb_set(value::jsonb,'{enabled}','false')::text WHERE key="+quote('a1_user_sync.request.'+str(user_id)))
        for db in (target_db,source_db):
            sql(db, "BEGIN; DELETE FROM api_keys WHERE user_id IN(SELECT id FROM users WHERE email="+quote(email)+"); DELETE FROM user_subscriptions WHERE user_id IN(SELECT id FROM users WHERE email="+quote(email)+"); DELETE FROM payment_orders WHERE user_id IN(SELECT id FROM users WHERE email="+quote(email)+"); DELETE FROM auth_identities WHERE user_id IN(SELECT id FROM users WHERE email="+quote(email)+"); DELETE FROM users WHERE email="+quote(email)+"; COMMIT;")
        sql(source_db, 'DELETE FROM settings WHERE key IN('+quote('a1_user_sync.request.'+str(user_id))+','+quote('a1_user_sync.result.'+str(user_id))+')')
        print('PASS: disposable verification data removed')
