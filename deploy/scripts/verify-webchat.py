"""Run inside the private Open WebUI container after a managed release."""
import json
import os
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import Request, urlopen


def request(url, *, body=None, headers=None, status=200):
    req = Request(url, data=json.dumps(body).encode() if body is not None else None,
                  headers={'Content-Type': 'application/json', **(headers or {})})
    try:
        response = urlopen(req, timeout=15)
    except HTTPError as error:
        response = error
    with response:
        if response.status != status:
            raise SystemExit(f'Webchat check expected HTTP {status}, got {response.status} for {url}')
        return json.load(response)


# The application's secrets, user keys and conversation history stay outside the probe.
sidecar = 'http://localhost:8080'
login = request(sidecar + '/api/v1/auths/signin', body={
    'email': os.environ['WEBUI_ADMIN_EMAIL'],
    'password': Path('/run/secrets/webchat_password').read_text().strip(),
})
models = request(sidecar + '/openai/models', headers={'Authorization': 'Bearer ' + login['token']})
if [model['id'] for model in models['data']] != ['modelgate-chat']:
    raise SystemExit('Open WebUI has an unexpected provider configuration')
request('http://sub2api:8080/api/v1/webchat/keys', status=401)
# A nonexistent, syntactically valid ticket exercises the provider round trip
# without resolving a user key or invoking a billable model.
blocked = request(sidecar + '/openai/chat/completions',
                  body={'model': 'modelgate-chat', 'messages': [{'role': 'user', 'content': 'release check'}],
                        'metadata': {'message_id': '0' * 64}, 'stream': False},
                  headers={'Authorization': 'Bearer ' + login['token']}, status=403)
if blocked.get('error', {}).get('message') != 'chat request unavailable':
    raise SystemExit('Open WebUI did not return the ModelGate callback ticket refusal')
print('Webchat verification passed: private Open WebUI service login, provider, user auth and callback ticket.')
