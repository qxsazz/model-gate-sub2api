#!/usr/bin/env python3
"""Synthetic HTTPS acceptance on the staging host only. Token arrives on stdin.

Creates two disposable accounts, exercises website JWT + Bot APIs, then removes
their data and budget contribution. No production origin or platform RPCs.
"""
import base64
import datetime as dt
import hashlib
import hmac
import json
from pathlib import Path
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid

ORIGIN = "https://staging.model-gate.cc"
root = Path(sys.argv[1])
if root.name != "sub2api-staging":
    raise SystemExit("staging path required")
token = sys.stdin.read().strip()
if not token.startswith("mgbot_"):
    raise SystemExit("dedicated synthetic Bot credential required")
compose = ["docker", "compose", "--project-name", "sub2api-staging", "--env-file", str(root / ".env"), "-f", str(root / "docker-compose.base.yml"), "-f", str(root / "docker-compose.staging.yml")]


def sql(query):
    result = subprocess.run(compose + ["exec", "-T", "postgres", "sh", "-c", 'exec psql -XAtq -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'], input=query, capture_output=True, text=True, timeout=45)
    if result.returncode:
        raise RuntimeError("synthetic SQL verification failed (details omitted)")
    return result.stdout.strip()


def call(path, credential, body=None, method="POST", expected=200):
    request = urllib.request.Request(ORIGIN + "/api/v1" + path, data=json.dumps(body).encode() if body is not None else None, method=method, headers={"Authorization": "Bearer " + credential, "Content-Type": "application/json", "User-Agent": "ModelGate-Staging-Bot-Acceptance"})
    try:
        response = urllib.request.urlopen(request, timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        status = response.status
        payload = json.load(response)
    if status != expected:
        raise RuntimeError(f"{method} {path.split('/challenges/')[0]}: expected {expected}, got {status}; reason={payload.get('reason', 'unknown')}")
    return payload.get("data", payload)


def b64(value):
    return base64.urlsafe_b64encode(value).rstrip(b"=")


manifest = json.loads((root / "bot-upgrade-migrations.json").read_text())
applied = json.loads(sql("SELECT jsonb_agg(jsonb_build_object('filename',filename,'checksum',checksum) ORDER BY filename) FROM schema_migrations;"))
expected_checksums = {item["filename"]: item["runner_checksum"] for item in manifest["migrations"]}
if {item["filename"]: item["checksum"] for item in applied} != expected_checksums:
    raise SystemExit("deployed migration filenames/checksums differ from the candidate manifest")
print(f"Verified all {len(applied)} deployed migration filenames and checksums; {manifest['existing_sql_files_unchanged']} historical files preserved.")


environment = json.loads(subprocess.check_output(["docker", "inspect", "sub2api-staging", "--format", "{{json .Config.Env}}"], text=True))
secret = next((item.split("=", 1)[1] for item in environment if item.startswith("JWT_SECRET=")), "")
if not secret:
    raise SystemExit("staging must configure JWT_SECRET explicitly")


def jwt(uid, email, password):
    fingerprint = int.from_bytes(hashlib.sha256((email + "\n" + password).encode()).digest()[:8], "big") & 0x7fffffffffffffff
    claims = {"user_id": uid, "email": email, "role": "user", "token_version": fingerprint, "iat": int(time.time()) - 5, "nbf": int(time.time()) - 5, "exp": int(time.time()) + 600}
    content = b64(b'{"alg":"HS256","typ":"JWT"}') + b"." + b64(json.dumps(claims).encode())
    return (content + b"." + b64(hmac.new(secret.encode(), content, hashlib.sha256).digest())).decode()


prefix = uuid.uuid4().hex
users = []
identities = [{"platform": platform, "bot_id": "mg-synthetic-" + short, "group_id": "mg-synthetic-group", "sender_id": prefix + "-" + short} for platform, short in [("qq_official", "qq"), ("wechat_native", "wx")]]


def event(identity):
    return {"identity": identity, "event_id": uuid.uuid4().hex}


def checkin(identity):
    return {**event(identity), "event_at": dt.datetime.now(dt.timezone.utc).isoformat()}


def assert_status(payload, expected):
    if payload.get("status") != expected:
        raise RuntimeError(f"expected receipt {expected}, got {payload.get('status')}")
    if set(payload) - {"status", "request_key", "replayed", "checkin_date"}:
        raise RuntimeError("group receipt includes private account fields")


def bind(identity, user_token):
    request = event(identity)
    challenge = call("/integrations/bot/binding-challenges", token, request)
    duplicate = call("/integrations/bot/binding-challenges", token, request)
    assert duplicate["replayed"] and challenge["challenge_id"] == duplicate["challenge_id"]
    assert challenge["binding_url"].startswith(ORIGIN + "/chat-bindings/authorize/")
    challenge_path = "/user/chat-bindings/challenges/" + challenge["challenge_id"]
    preview = call(challenge_path, user_token, method="GET")
    assert preview["identity"] == identity
    authorization = call(challenge_path + "/authorize", user_token, {"confirm": True})
    wrong = {**identity, "sender_id": identity["sender_id"] + "-wrong"}
    call("/integrations/bot/bindings/confirm", token, {**event(wrong), "code": authorization["code"]}, expected=400)
    confirmation = {**event(identity), "code": authorization["code"]}
    assert_status(call("/integrations/bot/bindings/confirm", token, confirmation), "bound")
    assert call("/integrations/bot/bindings/confirm", token, confirmation)["replayed"]


def recharge(uid):
    sql(f"INSERT INTO payment_orders(user_id,amount,pay_amount,expires_at,status,payment_type,provider_key,provider_snapshot,out_trade_no,paid_at,completed_at) VALUES({uid},10,10,now(),'COMPLETED','alipay','easypay','{{\"currency\":\"CNY\",\"recharge_principal\":10}}','bot-smoke-{uuid.uuid4().hex}',now(),now());")


try:
    for index in range(2):
        email = f"bot-acceptance-{prefix}-{index}@example.invalid"
        password = "synthetic-unusable-" + uuid.uuid4().hex
        uid = int(sql(f"INSERT INTO users(email,password_hash,balance,created_at) VALUES('{email}','{password}',0,now()-interval '2 days') RETURNING id;"))
        users.append((uid, jwt(uid, email, password)))
    (uid, user_token), (other_uid, other_token) = users
    qq, wx = identities
    call("/integrations/bot/binding-challenges", "invalid", event(qq), expected=401)
    call("/integrations/bot/binding-challenges", user_token, event(qq), expected=401)
    call("/user/chat-bindings", token, method="GET", expected=401)
    call("/integrations/bot/binding-challenges", token, event({**qq, "group_id": "outside-scope"}), expected=403)
    unbound = checkin(qq)
    assert_status(call("/integrations/bot/checkin", token, unbound), "not_bound")
    bind(qq, user_token)
    call("/integrations/bot/checkin", token, {**checkin(qq), "user_id": other_uid}, expected=400)
    unpaid = checkin(qq)
    assert_status(call("/integrations/bot/checkin", token, unpaid), "ineligible")
    recharge(uid)
    assert_status(call("/integrations/bot/checkin", token, unpaid), "ineligible")
    paid = checkin(qq)
    first = call("/integrations/bot/checkin", token, paid)
    assert_status(first, "checked_in")
    assert call("/integrations/bot/checkin", token, paid)["replayed"]
    bind(wx, user_token)
    assert_status(call("/integrations/bot/checkin", token, checkin(wx)), "already_checked_in")
    bindings = call("/user/chat-bindings", user_token, method="GET")
    qq_binding = next(item for item in bindings if item["identity"] == qq)
    call("/user/chat-bindings/" + str(qq_binding["id"]), other_token, method="DELETE", expected=404)
    call("/user/chat-bindings/" + str(qq_binding["id"]), user_token, method="DELETE")
    bind(qq, other_token)
    old = call("/integrations/bot/checkin", token, paid)
    assert old["replayed"] and old["request_key"] == first["request_key"]
    assert sql(f"SELECT count(*) FROM achievement_checkins WHERE user_id={other_uid};") == "0"
    recharge(other_uid)
    assert_status(call("/integrations/bot/checkin", token, checkin(qq)), "checked_in")
    old_day = {**checkin(wx), "event_at": (dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=1)).isoformat()}
    assert_status(call("/integrations/bot/checkin", token, old_day), "date_changed")
    assert sql(f"SELECT count(*) FROM achievement_checkins WHERE user_id IN ({uid},{other_uid});") == "2"
    print("Bot HTTPS acceptance passed: scoped credentials, QQ/WeChat binding, eligibility, replay, ownership, unlink/rebind and Shanghai day.")
finally:
    if users:
        ids = ",".join(str(item[0]) for item in users)
        # The shared checkin path updates daily spent even when budget limits are off.
        sql(f"BEGIN; SELECT 1 FROM achievement_config WHERE id=true FOR UPDATE; UPDATE achievement_budget b SET spent=GREATEST(0,b.spent-c.gross) FROM (SELECT day,sum(gross) gross FROM achievement_checkins WHERE user_id IN ({ids}) GROUP BY day) c WHERE b.day=c.day; DELETE FROM chat_bot_events WHERE user_id IN ({ids}) OR identity_key IN (SELECT identity_key FROM chat_binding_challenges WHERE user_id IN ({ids})); DELETE FROM payment_orders WHERE user_id IN ({ids}); DELETE FROM users WHERE id IN ({ids}); COMMIT;")
        # Unbound/wrong-sender receipts have no user association; remove only this run's identities.
        for identity in identities:
            for sender in [identity["sender_id"], identity["sender_id"] + "-wrong"]:
                digest = hashlib.sha256(json.dumps([identity["platform"], identity["bot_id"], identity["group_id"], sender], separators=(",", ":")).encode()).hexdigest()
                sql(f"DELETE FROM chat_bot_events WHERE identity_key='{digest}'; DELETE FROM chat_binding_challenges WHERE identity_key='{digest}';")
        print("Synthetic accounts, payments and checkin budget contribution removed.")
