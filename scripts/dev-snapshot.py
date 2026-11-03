#!/usr/bin/env python3
"""Read a production snapshot. Restore only into an empty local dev database."""
import argparse,base64,json,os,pathlib,subprocess,sys,urllib.parse
p=argparse.ArgumentParser();p.add_argument('--kubeconfig',required=True);p.add_argument('--namespace',default='uvy-prod');args=p.parse_args()
root=pathlib.Path(__file__).resolve().parents[1];private=pathlib.Path.home()/'.local/share/remventory-dev';private.mkdir(mode=0o700,parents=True,exist_ok=True);private.chmod(0o700)
local=['docker','exec','remventory-dev-db','psql','-U','postgres','-d','remventory_dev','-Atc']
count=subprocess.check_output(local+["select count(*) from information_schema.tables where table_schema='public'"],text=True).strip()
if count!='0':sys.exit('Local database is populated. Use a separate empty Docker volume for a new snapshot; this script never replaces an existing collection.')
env={}
for name in ['remventory-db','remventory-llm','remventory-s3']:
    secret=json.loads(subprocess.check_output(['kubectl','--kubeconfig',args.kubeconfig,'-n',args.namespace,'get','secret',name,'-o','json']))
    env.update({k:base64.b64decode(v).decode() for k,v in secret['data'].items()})
f=private/'production.json';f.write_text(json.dumps(env));f.chmod(0o600)
u=urllib.parse.urlparse(env['DATABASE_URL']);child=os.environ.copy();child.update(PGHOST=u.hostname,PGPORT=str(u.port or 5432),PGUSER=urllib.parse.unquote(u.username),PGPASSWORD=urllib.parse.unquote(u.password),PGDATABASE=u.path.lstrip('/'),PGSSLMODE=dict(urllib.parse.parse_qsl(u.query)).get('sslmode','prefer'),PGOPTIONS='-c default_transaction_read_only=on')
dump=private/'production.dump'
with dump.open('wb') as out:
    subprocess.run(['docker','run','--rm','--network','host']+sum((['-e',k] for k in ['PGHOST','PGPORT','PGUSER','PGPASSWORD','PGDATABASE','PGSSLMODE','PGOPTIONS']),[])+['postgres:18','pg_dump','--no-owner','--no-acl','-Fc'],env=child,stdout=out,check=True)
dump.chmod(0o600)
with dump.open('rb') as inp:subprocess.run(['docker','exec','-i','remventory-dev-db','pg_restore','-U','postgres','-d','remventory_dev','--no-owner','--no-acl','--exit-on-error'],stdin=inp,check=True)
subprocess.run(['go','run','./cmd/devcopy'],cwd=root,check=True)
print('Production snapshot copied locally. Source DB was read-only; source S3 was accessed only with GET. Credentials stay outside Git.')
