#!/usr/bin/env python3
"""Compare recovery off/on using only dedicated local synthetic fixtures.

Initialize/seed recovery-normal-load as described in README.md and build its
server first. The mock and each gateway are stopped in finally; dedicated Redis
is cleared between runs. No remote target or real credentials are accepted.
"""
import asyncio, importlib.util, json, os, subprocess, sys
from pathlib import Path
import aiohttp
ROOT=Path('/workspace/.onboarding/recovery-normal-load')
REPO=Path('/workspace/model-gate-sub2api')
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('exercise',REPO/'tools/loadtest/gateway_recovery.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);m.ROOT=ROOT
BINARY=ROOT/'server'
DOCKER=['docker','--host=unix:///var/run/docker.sock']
(ROOT/'prices.json').write_text(json.dumps({'gpt-4o-mini':{'input_cost_per_token':.00000015,'output_cost_per_token':.0000006,'litellm_provider':'openai','mode':'chat','max_input_tokens':128000,'max_output_tokens':16384}}))
config=ROOT/'data/config.yaml'
if '\npricing:\n' not in config.read_text():
 with config.open('a') as f:f.write(f'\npricing:\n  remote_url: ""\n  hash_url: ""\n  data_dir: "{ROOT}/pricing"\n  fallback_file: "{ROOT}/prices.json"\n')
def launch(enabled,tag):
 subprocess.run(DOCKER+['exec','mg-recovery-load-redis','redis-cli','FLUSHDB'],check=True,capture_output=True)
 env=os.environ.copy();env.update({'DATA_DIR':str(ROOT/'data'),'SERVER_HOST':'127.0.0.1','SERVER_PORT':'18086','GATEWAY_RECOVERY_ENABLED':str(enabled).lower(),'GATEWAY_RECOVERY_BUDGET_SECONDS':'300','GATEWAY_RECOVERY_SELECTION_WAIT_SECONDS':'120','GATEWAY_RECOVERY_MAX_ATTEMPTS':'12','GATEWAY_RECOVERY_MAX_WAITERS':'256','GATEWAY_RECOVERY_MAX_WAITERS_PER_GROUP':'64','GATEWAY_RECOVERY_MAX_WAITERS_PER_USER':'2','SECURITY_URL_ALLOWLIST_ENABLED':'false','SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS':'true','SECURITY_URL_ALLOWLIST_ALLOW_HTTP':'true','GATEWAY_OPENAI_HTTP2_ENABLED':'false','DATABASE_MAX_OPEN_CONNS':'20','DATABASE_MAX_IDLE_CONNS':'10','GOMAXPROCS':'2','GOMEMLIMIT':'512MiB'})
 with (ROOT/(tag+'.log')).open('w') as out:p=subprocess.Popen([str(BINARY)],env=env,cwd=ROOT,stdout=out,stderr=subprocess.STDOUT,start_new_session=True)
 (ROOT/'server.pid').write_text(str(p.pid));return p
async def ready(session,p):
 for _ in range(100):
  if p.poll() is not None:raise RuntimeError('gateway exited')
  try:
   async with session.get(m.GATEWAY+'/health') as r:
    if r.status==200:return
  except aiohttp.ClientError:pass
  await asyncio.sleep(.2)
 raise RuntimeError('health timeout')
def stop(p):
 p.terminate()
 try:p.wait(timeout=10)
 except subprocess.TimeoutExpired:p.kill();p.wait()
async def main():
 with (ROOT/'mock.log').open('w') as out:
  mock=subprocess.Popen([sys.executable,'-B','-c',f"import sys;sys.path.insert(0,{str(REPO/'tools/loadtest')!r});import gateway_recovery as m;m.serve()"],stdout=out,stderr=subprocess.STDOUT,start_new_session=True)
 (ROOT/'mock.pid').write_text(str(mock.pid))
 await asyncio.sleep(.5)
 p=None;allrows=[]
 try:
  async with aiohttp.ClientSession() as session:
   for run,enabled in enumerate([False,True,True,False]):
    tag=f'normal-{run}-'+('on' if enabled else 'off');p=launch(enabled,tag);await ready(session,p)
    x=m.Exercise(session);await x.control({'reset':True,**{lane:{'mode':'healthy','delay':.04} for lane in m.LANES}})
    await x.batch(tag+'-warm','healthy',20,4)
    await x.batch(tag+'-warm-busy-keys','busy',96,4)
    for n,c,lane,extra in [(512,32,'healthy',{}),(512,128,'healthy',{}),(128,32,'healthy',{'stream':True}),(96,96,'busy',{}),(32,16,'healthy',{'model':'load-unsupported-model'})]:
     row=await x.batch(tag+f'-{lane}-{n}',lane,n,c,**extra);allrows.append(dict(row,recovery=enabled,run=run))
     if extra.get('model'):
      assert row['statuses']=={'404':n} and row['upstream_attempts']==0,row
     else:assert row['success']==n and row['upstream_attempts']==n,row
    stop(p);p=None
    text=(ROOT/(tag+'.log')).read_text()
    assert 'gateway.recovery_wait' not in text and 'gateway.recovery_finished' not in text,'normal traffic entered recovery'
    (ROOT/'comparison.json').write_text(json.dumps(allrows,indent=2))
   p=launch(True,'fault-on');await ready(session,p);x=m.Exercise(session)
   await x.control({'reset':True,'mixed503':{'mode':'mixed503','delay':.04}})
   row=await x.batch('fault-partial503','mixed503',128,32);allrows.append(row);assert row['success']==128,row
   stop(p);p=None
   for enabled in [False,True]:
    p=launch(enabled,'first-selection-'+str(enabled));await ready(session,p)
    ids=','.join(map(str,x.fixture['accounts']['healthy']))
    m.sql(f"UPDATE accounts SET temp_unschedulable_until=now()+interval '2 seconds',temp_unschedulable_reason='load transient' WHERE id IN ({ids})")
    x=m.Exercise(session);await x.control({'reset':True,'healthy':{'mode':'healthy','delay':.04}})
    row=await x.batch('transient-pool-'+str(enabled),'healthy',1,1);allrows.append(row)
    assert row['success']==int(enabled),row
    stop(p);p=None
   (ROOT/'comparison.json').write_text(json.dumps(allrows,indent=2))
 finally:
  if p:stop(p)
  stop(mock)
if __name__ == '__main__':
 assert m.sql('SELECT current_database()') == 'loadtest'
 assert m.sql("SELECT count(*) FROM users WHERE email NOT LIKE 'load-user-%@example.invalid' AND email <> 'loadtest@example.invalid'") == '0'
 asyncio.run(main())
