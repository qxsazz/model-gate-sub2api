#!/usr/bin/env python3
"""Isolated recovery fault/load exercise. Synthetic local SQL fixtures only.

Requires aiohttp/psutil, a locally built gateway, and dedicated local Postgres
and Redis containers. Never accepts a remote gateway URL or real API keys.
"""
import argparse
import asyncio
import collections
import json
import math
import os
from pathlib import Path
import re
import subprocess
import time

import aiohttp
from aiohttp import web
import psutil

ROOT = Path('/workspace/.onboarding/recovery-load')
GATEWAY = 'http://127.0.0.1:18086'
UPSTREAM = 'http://127.0.0.1:18087'
MODEL = 'gpt-4o-mini'
LANES = ['healthy', 'mixed503', 'mixed401', 'outage', 'busy', 'slow']


def sql(query):
    result = subprocess.run(['docker','--host=unix:///var/run/docker.sock','exec','-i','mg-recovery-load-pg','psql','-X','-v','ON_ERROR_STOP=1','-U','loadtest','-d','loadtest','-At'], input=query, text=True, capture_output=True, check=True)
    return result.stdout.strip()


def quote(value):
    return "'" + str(value).replace("'", "''") + "'"


def seed():
    assert sql('SELECT current_database()') == 'loadtest'
    assert sql('SELECT count(*) FROM accounts') == '0', 'seed requires a fresh isolated database'
    assert sql("SELECT count(*) FROM users WHERE email <> 'loadtest@example.invalid'") == '0', 'unexpected users in fixture database'
    groups, accounts = {}, {}
    for lane in LANES:
        gid = int(sql(f"INSERT INTO groups(name,platform,rate_multiplier,status,subscription_type,profit_control_enabled,model_allowlist) VALUES('load-{lane}','openai',1,'active','standard',false,'{{\"enabled\":true,\"models\":[\"{MODEL}\"]}}') RETURNING id;" ).splitlines()[0])
        groups[lane] = gid
        accounts[lane] = []
        for slot in range(2 if lane in ['busy','slow'] else 4):
            creds = {'api_key':'loadtest-upstream-fixture','base_url':f'{UPSTREAM}/{lane}/{slot}','model_mapping':{MODEL:MODEL},'pool_mode':True,'pool_mode_retry_count':3}
            extra = {'openai_responses_mode':'force_chat_completions','openai_responses_supported':False}
            concurrency = 1 if lane in ['busy','slow'] else 32
            aid = int(sql(f"INSERT INTO accounts(name,platform,type,credentials,extra,concurrency,priority,status,schedulable,rate_multiplier) VALUES('load-{lane}-{slot}','openai','apikey',{quote(json.dumps(creds))}::jsonb,{quote(json.dumps(extra))}::jsonb,{concurrency},{slot+1},'active',true,1) RETURNING id;").splitlines()[0])
            sql(f'INSERT INTO account_groups(account_id,group_id,priority) VALUES({aid},{gid},{slot+1});')
            accounts[lane].append(aid)
    sql("INSERT INTO users(email,password_hash,role,balance,concurrency,status) SELECT 'load-user-'||i||'@example.invalid','test-fixture-login-disabled','user',10000,4,'active' FROM generate_series(1,160) AS i;")
    users = [int(x) for x in sql("SELECT id FROM users WHERE email LIKE 'load-user-%' ORDER BY id").splitlines()]
    for gid in groups.values():
        sql(f"INSERT INTO api_keys(user_id,key,name,group_id,status) SELECT id,'load-fixture-user-'||id||'-group-{gid}','load-test',{gid},'active' FROM users WHERE email LIKE 'load-user-%';")
    sql("INSERT INTO settings(key,value) VALUES('openai_advanced_scheduler_enabled','true'),('openai_advanced_scheduler_subscription_priority_enabled','false') ON CONFLICT(key) DO UPDATE SET value=excluded.value;")
    fixture = {'groups':groups,'accounts':accounts,'users':users}
    path = ROOT/'fixtures.json'
    path.write_text(json.dumps(fixture,indent=2)); path.chmod(0o600)
    print(json.dumps({'seeded_groups':groups,'synthetic_users':len(users),'accounts':sum(map(len,accounts.values()))}))


class Mock:
    def __init__(self):
        self.controls = {}
        self.reset()

    def reset(self):
        self.stats = {'hits':collections.Counter(),'statuses':collections.Counter(),'attempts':collections.Counter(),'active':0,'peak_active':0}

    async def control(self, request):
        config = await request.json()
        if config.pop('reset',False): self.reset()
        for lane,value in config.items():
            self.controls[lane] = dict(value, changed=time.monotonic())
        return web.json_response({'ok':True})

    async def status(self, request):
        return web.json_response({k:dict(v) if isinstance(v,collections.Counter) else v for k,v in self.stats.items()})

    async def handle(self, request):
        lane,slot = request.match_info['lane'],int(request.match_info['slot'])
        body = await request.json()
        marker = re.search(r'LOADTEST:([a-zA-Z0-9_-]+)',json.dumps(body))
        marker = marker.group(1) if marker else 'unmarked'
        self.stats['hits'][f'{lane}/{slot}'] += 1
        self.stats['attempts'][marker] += 1
        self.stats['active'] += 1
        self.stats['peak_active'] = max(self.stats['active'],self.stats['peak_active'])
        try:
            config = self.controls.get(lane,{'mode':'healthy'})
            mode = config.get('mode','healthy')
            status = 200
            if mode == 'all503' or mode == 'mixed503' and slot < 2: status = 503
            if mode == 'mixed401' and slot == 0: status = 401
            if mode == 'recover_after' and time.monotonic() < config['changed']+config.get('seconds',3): status = 503
            await asyncio.sleep(config.get('delay',0.04))
            self.stats['statuses'][str(status)] += 1
            if status != 200:
                return web.json_response({'error':{'message':f'mock upstream {status}','type':'server_error' if status==503 else 'authentication_error'}},status=status,headers={'Retry-After':'1'} if status==503 else None)
            text = f'ok:LOADTEST:{marker}'
            rid = f'chatcmpl-{marker}'
            if body.get('stream'):
                response = web.StreamResponse(headers={'Content-Type':'text/event-stream'})
                await response.prepare(request)
                chunk = {'id':rid,'object':'chat.completion.chunk','created':int(time.time()),'model':MODEL,'choices':[{'index':0,'delta':{'role':'assistant','content':text},'finish_reason':None}]}
                await response.write(('data: '+json.dumps(chunk)+'\n\n').encode())
                if mode == 'partial_close':
                    request.transport.close()
                    return response
                await response.write(('data: '+json.dumps({'id':rid,'object':'chat.completion.chunk','model':MODEL,'choices':[{'index':0,'delta':{},'finish_reason':'stop'}],'usage':{'prompt_tokens':1,'completion_tokens':1,'total_tokens':2}})+'\n\ndata: [DONE]\n\n').encode())
                return response
            return web.json_response({'id':rid,'object':'chat.completion','created':int(time.time()),'model':MODEL,'choices':[{'index':0,'message':{'role':'assistant','content':text},'finish_reason':'stop'}],'usage':{'prompt_tokens':1,'completion_tokens':1,'total_tokens':2}})
        except (ConnectionResetError,asyncio.CancelledError):
            raise
        finally:
            self.stats['active'] -= 1


def serve():
    mock = Mock(); app = web.Application()
    app.router.add_post('/control',mock.control);app.router.add_get('/stats',mock.status)
    app.router.add_post('/{lane}/{slot}/{path:.*}',mock.handle)
    web.run_app(app,host='127.0.0.1',port=18087,access_log=None,print=None)


def percentile(values,p):
    return round(sorted(values)[max(0,math.ceil(len(values)*p)-1)]*1000,2) if values else None


class Exercise:
    def __init__(self,session):
        self.session = session
        self.fixture = json.loads((ROOT/'fixtures.json').read_text())
        self.results = []

    async def control(self,config):
        async with self.session.post(UPSTREAM+'/control',json=config) as response: assert response.status==200

    async def stats(self):
        async with self.session.get(UPSTREAM+'/stats') as response: return await response.json()

    async def request(self,lane,marker,index,stream=False,timeout=145,user_offset=0,model=MODEL,fixed_user=False):
        uid = self.fixture['users'][(user_offset if fixed_user else index+user_offset)%len(self.fixture['users'])]
        gid = self.fixture['groups'][lane]
        key = f'load-fixture-user-{uid}-group-{gid}'
        start=time.monotonic(); first=None
        try:
            async with self.session.post(GATEWAY+'/v1/chat/completions',json={'model':model,'messages':[{'role':'user','content':'LOADTEST:'+marker}],'stream':stream},headers={'Authorization':'Bearer '+key},timeout=aiohttp.ClientTimeout(total=timeout)) as response:
                body=b''
                async for chunk in response.content.iter_any():
                    if first is None:first=time.monotonic()-start
                    body+=chunk
                decoded=body.decode(errors='replace')
                success=response.status==200 and ('ok:LOADTEST:'+marker) in decoded and (not stream or '[DONE]' in decoded)
                return {'status':response.status,'success':success,'seconds':time.monotonic()-start,'first_byte_seconds':first,'stream_terminal_error':stream and response.status==200 and not success,'body_sample':decoded[:400] if not success else '', 'marker':marker}
        except (asyncio.TimeoutError,aiohttp.ClientError) as error:
            return {'status':type(error).__name__,'success':False,'seconds':time.monotonic()-start,'first_byte_seconds':first,'marker':marker}

    async def batch(self,name,lane,n,concurrency,**options):
        sem=asyncio.Semaphore(concurrency)
        tag=f'{name}-{time.monotonic_ns()}'
        start=time.monotonic()
        async def one(i):
            async with sem:return await self.request(lane,f'{tag}-{i}',i,**options)
        rows=await asyncio.gather(*(one(i) for i in range(n)))
        elapsed=time.monotonic()-start;after=await self.stats()
        attempts={k:v for k,v in after['attempts'].items() if k.startswith(tag+'-')}
        durations=[r['seconds'] for r in rows]
        result={'scenario':name,'lane':lane,'requests':n,'concurrency':concurrency,'seconds':round(elapsed,3),'rps':round(n/elapsed,2),'success':sum(r['success'] for r in rows),'statuses':dict(collections.Counter(str(r['status']) for r in rows)),'p50_ms':percentile(durations,.5),'p95_ms':percentile(durations,.95),'p99_ms':percentile(durations,.99),'max_ms':percentile(durations,1),'upstream_attempts':sum(attempts.values()),'max_attempts_per_request':max(attempts.values(),default=0),'first_bytes_p95_ms':percentile([r['first_byte_seconds'] for r in rows if r['first_byte_seconds'] is not None],.95),'errors':[r for r in rows if not r['success']][:3]}
        self.results.append(result)
        (ROOT/'results.json').write_text(json.dumps(self.results,indent=2))
        print(json.dumps({k:v for k,v in result.items() if k!='errors'}),flush=True)
        return result

    async def run(self):
        await self.control({'reset':True,**{lane:{'mode':'healthy'} for lane in LANES}})
        warm=await self.batch('warmup','healthy',20,4)
        if warm['success']!=20:raise RuntimeError('Warmup failed; inspect results.json before applying load')
        for concurrency in [8,32,64,128]:
            await self.batch('healthy'+str(concurrency),'healthy',512,concurrency)
        await self.control({'mixed503':{'mode':'mixed503'}})
        await self.batch('partial503','mixed503',512,64)
        await self.control({'mixed401':{'mode':'mixed401'}})
        await self.batch('partial401','mixed401',256,32)
        await self.control({'busy':{'mode':'healthy','delay':3}})
        pressure=asyncio.create_task(self.batch('busy-pressure','busy',96,96))
        await asyncio.sleep(.5)
        await self.batch('healthy-under-pressure','healthy',512,32,user_offset=80)
        await pressure
        await self.control({'outage':{'mode':'all503'}})
        outage=asyncio.create_task(self.batch('all503','outage',96,96))
        await asyncio.sleep(1)
        await self.batch('healthy-during-outage','healthy',512,32,user_offset=80)
        await outage
        await self.control({'slow':{'mode':'healthy','delay':15}})
        canceled=await self.batch('cancel','slow',48,48,timeout=.5)
        await asyncio.sleep(1)
        await self.control({'slow':{'mode':'healthy','delay':.04}})
        await self.batch('post-cancel','slow',32,8)
        await self.batch('unsupported','healthy',32,16,model='not-supported-loadtest')
        await self.control({'healthy':{'mode':'healthy','delay':.04}})
        await self.batch('stream','healthy',128,32,stream=True)
        await self.control({'slow':{'mode':'partial_close','delay':.04}})
        await self.batch('partial-output','slow',8,4,stream=True)


async def run():
    process=psutil.Process(int((ROOT/'server.pid').read_text()))
    samples=[]
    async def monitor():
        process.cpu_percent()
        while True:
            samples.append({'at':time.time(),'rss_mb':round(process.memory_info().rss/1024**2,2),'cpu_percent':process.cpu_percent(),'threads':process.num_threads(),'fds':process.num_fds()})
            await asyncio.sleep(.5)
    task=asyncio.create_task(monitor())
    async with aiohttp.ClientSession(connector=aiohttp.TCPConnector(limit=300)) as session:
        try:await Exercise(session).run()
        finally:
            task.cancel()
            (ROOT/'resources.json').write_text(json.dumps({'samples':samples,'peak_rss_mb':max((s['rss_mb'] for s in samples),default=0),'peak_cpu_percent':max((s['cpu_percent'] for s in samples),default=0)},indent=2))


async def probes():
    async with aiohttp.ClientSession(connector=aiohttp.TCPConnector(limit=64)) as session:
        exercise=Exercise(session)
        exercise.results=json.loads((ROOT/'results.json').read_text())
        waiting=asyncio.create_task(exercise.batch('same-user-wait-cancel','outage',12,12,timeout=.5,fixed_user=True))
        await asyncio.sleep(.1)
        await exercise.batch('same-user-healthy','healthy',1,1,timeout=3,fixed_user=True)
        await waiting
        await asyncio.sleep(1)
        fixture=exercise.fixture
        # Closed/open comparison uses an otherwise identical fresh local process.
        for enabled,name,budget,blocked,delay in [('false','first-busy-disabled',300,5,.04),('true','first-busy-enabled',300,5,.04),('true','budget-overrun-scaled',3,.8,5)]:
            pid=int((ROOT/'server.pid').read_text());os.kill(pid,15)
            for _ in range(100):
                try:
                    async with session.get(GATEWAY+'/health',timeout=aiohttp.ClientTimeout(total=.2)):pass
                except (aiohttp.ClientError,asyncio.TimeoutError):break
                await asyncio.sleep(.1)
            subprocess.run(['docker','--host=unix:///var/run/docker.sock','exec','mg-recovery-load-redis','redis-cli','FLUSHDB'],check=True,capture_output=True)
            ids=','.join(map(str,fixture['accounts']['healthy']))
            sql(f"UPDATE accounts SET status='active',schedulable=true,overload_until=NULL,rate_limit_reset_at=NULL,temp_unschedulable_until=now()+interval '{blocked} seconds' WHERE id IN ({ids});")
            await exercise.control({'healthy':{'mode':'healthy','delay':delay}})
            env=os.environ.copy();env.update({'LOADTEST_RECOVERY_ENABLED':enabled,'LOADTEST_RECOVERY_BUDGET':str(budget),'LOADTEST_SELECTION_WAIT':str(min(budget,120))})
            with (ROOT/f'{name}-launch.log').open('w') as output:
                subprocess.run(['python',str(Path(__file__).with_name('launch_gateway.py'))],env=env,stdout=output,check=True)
            for _ in range(100):
                try:
                    async with session.get(GATEWAY+'/health',timeout=aiohttp.ClientTimeout(total=.2)) as response:
                        if response.status==200:break
                except (aiohttp.ClientError,asyncio.TimeoutError):pass
                await asyncio.sleep(.1)
            await exercise.batch(name,'healthy',1,1,timeout=20)
            (ROOT/f'{name}-gateway.log').write_bytes((ROOT/'gateway.log').read_bytes())


async def cancellation_probe():
    def slots():
        code='local out={account_slots=0,user_slots=0,waiting=0} local keys=redis.call("KEYS","concurrency:*") for _,k in ipairs(redis.call("KEYS","wait:account:*")) do table.insert(keys,k) end for _,k in ipairs(keys) do local t=redis.call("TYPE",k).ok if string.match(k,"^concurrency:account:%d+$") then out.account_slots=out.account_slots+redis.call("ZCARD",k) elseif string.match(k,"^concurrency:user:%d+$") then out.user_slots=out.user_slots+redis.call("ZCARD",k) elseif (string.match(k,"^concurrency:wait:%d+$") or string.match(k,"^wait:account:%d+$")) and t=="string" then out.waiting=out.waiting+tonumber(redis.call("GET",k) or "0") end end return cjson.encode(out)'
        return json.loads(subprocess.check_output(['docker','--host=unix:///var/run/docker.sock','exec','mg-recovery-load-redis','redis-cli','EVAL',code,'0'],text=True))
    async with aiohttp.ClientSession() as session:
        exercise=Exercise(session);exercise.results=json.loads((ROOT/'results.json').read_text())
        await exercise.control({'slow':{'mode':'healthy','delay':4}})
        await exercise.batch('cancel-retention','slow',8,8,timeout=.3)
        await asyncio.sleep(.5);early=slots();active=(await exercise.stats())['active']
        await exercise.control({'slow':{'mode':'healthy','delay':.04}})
        following=asyncio.create_task(exercise.batch('post-cancel-retention','slow',4,2,timeout=10))
        await asyncio.sleep(.25);during=slots()
        result=await following
        await asyncio.sleep(.2);late=slots()
        evidence={'client_cancel_seconds':.3,'mock_upstream_delay_seconds':4,'after_cancel_500ms':early,'mock_active_after_cancel':active,'during_following_requests':during,'following_requests_p95_ms':result['p95_ms'],'after_following_requests':late}
        (ROOT/'cancellation-slots.json').write_text(json.dumps(evidence,indent=2));print(json.dumps(evidence),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('command',choices=['seed','serve','run','probes','cancel-probe'])
    command=parser.parse_args().command
    if command=='seed':seed()
    elif command=='serve':serve()
    elif command=='run':asyncio.run(run())
    elif command=='probes':asyncio.run(probes())
    else:asyncio.run(cancellation_probe())
