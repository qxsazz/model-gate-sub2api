"""Exercise the real deployment shell script against isolated fake Docker/HTTP tools."""
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'scripts' / 'sub2api-deploy.sh'
REPO = 'ghcr.io/qxsazz/model-gate-sub2api@sha256:'
PREVIOUS, TARGET, STALE = (REPO + c * 64 for c in 'abc')

MOCK = r'''
import json, os, pathlib, sys
state=pathlib.Path(os.environ['FAKE_STATE']); d=json.loads(state.read_text())
args=sys.argv[1:]; name=pathlib.Path(sys.argv[0]).name
with open(os.environ['FAKE_TRACE'],'a') as f: f.write(json.dumps([name,args,os.environ.get('SUB2API_IMAGE')])+'\n')
if name in ['curl','sleep']: sys.exit(0)
if name=='mv':
    if os.environ.get('FAIL_PERSIST') and d['image']==os.environ['TARGET']:
        sys.exit(1)
    os.replace(args[-2],args[-1]); sys.exit(0)
if args[0]=='inspect':
    if any('postgres' in a for a in args): sys.exit(1)
    fmt=args[-1]
    if 'State.Health' in fmt:
        print('unhealthy' if os.environ.get('FAIL_HEALTH') and d['image']==os.environ['TARGET'] else 'healthy')
    else: print(d['image'])
elif args[0]=='compose':
    envfile=pathlib.Path(args[args.index('--env-file')+1])
    image=os.environ.get('SUB2API_IMAGE')
    if not image:
        for line in envfile.read_text().splitlines():
            if line.strip().startswith('SUB2API_IMAGE='): image=line.split('=',1)[1]
    if 'config' in args and '--images' in args:
        if os.environ.get('FAIL_VERIFY') and d['image']==os.environ['TARGET'] and 'SUB2API_IMAGE' not in os.environ: image='wrong-image'
        print(image or 'missing-image')
        print('postgres:18-alpine')
    elif 'up' in args:
        if os.environ.get('FAIL_START') and image==os.environ['TARGET']: sys.exit(1)
        d['image']=image;state.write_text(json.dumps(d))
elif args[0]!='pull': sys.exit(2)
'''


@unittest.skipUnless(os.name == 'posix', 'Deployment shell integration requires POSIX')
class DeploymentImageTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.prod, self.stage = self.root/'production', self.root/'staging'
        self.bin = self.root/'bin'
        for p in [self.prod, self.stage, self.bin]: p.mkdir()
        for name in ['docker', 'curl', 'sleep', 'mv']:
            p = self.bin/name
            p.write_text('#!'+sys.executable+'\n'+MOCK)
            p.chmod(0o700)
        self.script = self.root/'deploy.sh'
        code = SCRIPT.read_text().replace('/opt/sub2api-staging', str(self.stage)).replace('/opt/sub2api', str(self.prod))
        code = code.replace('PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin', 'PATH='+str(self.bin)+':'+os.defpath)
        self.script.write_text(code)
        self.state, self.trace = self.root/'state.json', self.root/'trace.jsonl'
        self.state.write_text(json.dumps({'image': PREVIOUS}))
        self.env = dict(os.environ, FAKE_STATE=str(self.state), FAKE_TRACE=str(self.trace), TARGET=TARGET)
        self.env.pop('SUB2API_IMAGE', None)
        self.original = '# keep comment\nPOSTGRES_PASSWORD=literal$with#symbols\nSUB2API_IMAGE='+STALE+'\nREDIS_PASSWORD=unchanged\n'
        for root in [self.prod, self.stage]:
            (root/'.env').write_text(self.original)
            (root/'.env').chmod(0o640)

    def run_script(self, *args, **flags):
        return subprocess.run(['/bin/sh', str(self.script), *args], env={**self.env, **flags}, text=True, capture_output=True, timeout=15)

    def image(self): return json.loads(self.state.read_text())['image']

    def assert_config(self, root, image):
        self.assertEqual((root/'.env').read_text(), self.original.replace(STALE,image))
        self.assertEqual(stat.S_IMODE((root/'.env').stat().st_mode), 0o640)
        self.assertFalse(list(root.glob('.env.image.*')))

    def trace_calls(self):
        return [json.loads(line) for line in self.trace.read_text().splitlines()]

    def test_success_persists_for_both_environments_without_changing_secrets(self):
        for name,root in [('staging',self.stage),('production',self.prod)]:
            with self.subTest(environment=name):
                r=self.run_script('deploy',name,TARGET)
                self.assertEqual(r.returncode,0,r.stderr)
                self.assert_config(root,TARGET)
                self.assertEqual(self.image(),TARGET)
                calls=[x for x in self.trace_calls() if '--images' in x[1]]
                self.assertTrue(calls)
                self.assertTrue(all(x[2] is None for x in calls),'persistent verification must not inherit the override')

    def test_health_failure_rolls_back_running_and_persistent_image(self):
        r=self.run_script('deploy','production',TARGET,FAIL_HEALTH='1')
        self.assertNotEqual(r.returncode,0)
        self.assertEqual(self.image(),PREVIOUS)
        self.assert_config(self.prod,PREVIOUS)
        self.assertNotIn('completed_at=',next((self.prod/'releases').glob('*.env')).read_text())

    def test_persistence_failure_cannot_be_reported_as_success(self):
        r=self.run_script('deploy','production',TARGET,FAIL_PERSIST='1')
        self.assertNotEqual(r.returncode,0)
        self.assertEqual(self.image(),PREVIOUS)
        self.assert_config(self.prod,PREVIOUS)

    def test_config_verification_failure_rolls_back(self):
        r=self.run_script('deploy','production',TARGET,FAIL_VERIFY='1')
        self.assertNotEqual(r.returncode,0)
        self.assertEqual(self.image(),PREVIOUS)
        self.assert_config(self.prod,PREVIOUS)

    def test_sync_image_does_not_pull_or_recreate(self):
        r=self.run_script('sync-image','staging')
        self.assertEqual(r.returncode,0,r.stderr)
        self.assert_config(self.stage,PREVIOUS)
        self.assertFalse(any(x[0]=='docker' and ('up' in x[1] or 'pull' in x[1]) for x in self.trace_calls()))

    def test_verify_staging_rejects_stale_persistent_configuration(self):
        r=self.run_script('verify-staging',PREVIOUS)
        self.assertNotEqual(r.returncode,0)

    def test_rejects_non_hex_digest(self):
        r=self.run_script('deploy','production',REPO+'g'*64)
        self.assertNotEqual(r.returncode,0)
        self.assertEqual(self.image(),PREVIOUS)

    def test_missing_and_duplicate_image_assignments(self):
        for content in ['# keep\nPASSWORD=literal$unchanged\n',
                        '# keep\nPASSWORD=literal$unchanged\nexport SUB2API_IMAGE='+STALE+'\nSUB2API_IMAGE='+STALE+'\n']:
            with self.subTest(content=content):
                (self.stage/'.env').write_text(content)
                r=self.run_script('sync-image','staging')
                self.assertEqual(r.returncode,0,r.stderr)
                self.assertEqual((self.stage/'.env').read_text(),'# keep\nPASSWORD=literal$unchanged\nSUB2API_IMAGE='+PREVIOUS+'\n')

    def test_start_failure_rolls_back_persistent_config(self):
        r=self.run_script('deploy','production',TARGET,FAIL_START='1')
        self.assertNotEqual(r.returncode,0)
        self.assertEqual(self.image(),PREVIOUS)
        self.assert_config(self.prod,PREVIOUS)

    def test_sync_refuses_symlink_env(self):
        envfile=self.stage/'.env'
        target=self.stage/'real-env'
        envfile.rename(target)
        envfile.symlink_to(target)
        r=self.run_script('sync-image','staging')
        self.assertNotEqual(r.returncode,0)
        self.assertEqual(target.read_text(),self.original)


if __name__ == '__main__': unittest.main()
