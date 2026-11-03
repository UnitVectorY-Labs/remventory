#!/usr/bin/env python3
"""Start isolated Postgres and Garage. Named volumes survive container restarts."""
import pathlib, subprocess
root=pathlib.Path(__file__).resolve().parents[1]
def container(name,args):
    exists=subprocess.run(['docker','inspect',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0
    subprocess.run(['docker','start',name] if exists else ['docker','run','-d','--name',name]+args,check=True)
container('remventory-dev-db',['-p','127.0.0.1:55432:5432','-e','POSTGRES_PASSWORD=remventory-local','-e','POSTGRES_DB=remventory_dev','-v','remventory-dev-pg:/var/lib/postgresql','postgres:18'])
container('remventory-dev-s3',['-p','127.0.0.1:59000:3900','-e','GARAGE_DEFAULT_ACCESS_KEY=remventorylocalkey','-e','GARAGE_DEFAULT_SECRET_KEY=remventory-local-storage','-e','GARAGE_DEFAULT_BUCKET=default','-v',str(root/'dev/garage.toml')+':/etc/garage.toml:ro','-v','remventory-dev-garage:/var/lib/garage','dxflrs/garage:v2.3.0','/garage','server','--single-node','--default-bucket'])
print('Local Postgres: 127.0.0.1:55432; local Garage: 127.0.0.1:59000')
