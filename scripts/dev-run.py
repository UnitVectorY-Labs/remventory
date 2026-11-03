#!/usr/bin/env python3
"""Run against explicitly isolated local resources, never production credentials."""
import json, os, pathlib, subprocess, sys, urllib.parse
root = pathlib.Path(__file__).resolve().parents[1]
config = pathlib.Path(os.environ.get('REMVENTORY_DEV_CONFIG', str(pathlib.Path.home()/'.local/share/remventory-dev/local.json')))
env = os.environ.copy()
env.update(json.loads(config.read_text()))
u = urllib.parse.urlparse(env['DATABASE_URL'])
if u.hostname not in ('localhost', '127.0.0.1', '::1') or not u.path.endswith('/remventory_dev'):
    sys.exit('Development runner requires a loopback remventory_dev database.')
if urllib.parse.urlparse(env['S3_ENDPOINT']).hostname not in ('localhost', '127.0.0.1', '::1'):
    sys.exit('Development runner requires isolated local object storage.')
os.chdir(root)
os.execvpe('go', ['go', 'run', '.'], env)
