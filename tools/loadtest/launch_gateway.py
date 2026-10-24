#!/usr/bin/env python3
"""Launch only the synthetic local load-test gateway, never a remote deployment."""
import json
import os
from pathlib import Path
import subprocess

root=Path('/workspace/.onboarding/recovery-load')
assert (root/'fixtures.json').exists(), 'seed the dedicated local database first'
(root/'prices.json').write_text(json.dumps({'gpt-4o-mini':{'input_cost_per_token':0.00000015,'output_cost_per_token':0.0000006,'litellm_provider':'openai','mode':'chat','max_input_tokens':128000,'max_output_tokens':16384}}))
config=root/'data/config.yaml'
content=config.read_text()
if '\npricing:\n' not in content:
    with config.open('a') as output:output.write(f'\npricing:\n  remote_url: ""\n  hash_url: ""\n  data_dir: "{root}/pricing"\n  fallback_file: "{root}/prices.json"\n')
env=os.environ.copy()
env.update({'DATA_DIR':str(root/'data'),'SERVER_HOST':'127.0.0.1','SERVER_PORT':'18086','SERVER_MODE':'release','GATEWAY_RECOVERY_ENABLED':os.environ.get('LOADTEST_RECOVERY_ENABLED','true'),'GATEWAY_RECOVERY_BUDGET_SECONDS':os.environ.get('LOADTEST_RECOVERY_BUDGET','300'),'GATEWAY_RECOVERY_SELECTION_WAIT_SECONDS':os.environ.get('LOADTEST_SELECTION_WAIT','120'),'GATEWAY_RECOVERY_MAX_ATTEMPTS':'12','GATEWAY_RECOVERY_MAX_WAITERS':'256','GATEWAY_RECOVERY_MAX_WAITERS_PER_GROUP':'64','GATEWAY_RECOVERY_MAX_WAITERS_PER_USER':'2','SECURITY_URL_ALLOWLIST_ENABLED':'false','SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS':'true','SECURITY_URL_ALLOWLIST_ALLOW_HTTP':'true','GATEWAY_OPENAI_HTTP2_ENABLED':'false','DATABASE_MAX_OPEN_CONNS':'20','DATABASE_MAX_IDLE_CONNS':'10','GOMAXPROCS':'2','GOMEMLIMIT':'512MiB'})
with (root/'gateway.log').open('w') as log:
    process=subprocess.Popen([str(root/'server')],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
(root/'server.pid').write_text(str(process.pid))
print(json.dumps({'gateway_pid':process.pid,'recovery':env['GATEWAY_RECOVERY_ENABLED'],'budget':env['GATEWAY_RECOVERY_BUDGET_SECONDS'],'selection_wait':env['GATEWAY_RECOVERY_SELECTION_WAIT_SECONDS'],'database_pool':20}))
