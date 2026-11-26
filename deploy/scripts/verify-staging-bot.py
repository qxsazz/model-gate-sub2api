#!/usr/bin/env python3
"""Synthetic HTTPS acceptance on the staging host only. Token arrives on stdin.

Creates two disposable accounts, exercises website JWT + Bot APIs, then removes
their data and budget contribution. No production origin or platform RPCs.
"""
import base64
import datetime as dt
from decimal import Decimal
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
expected_files = {item["filename"]: item for item in manifest["migrations"]}
if {item["filename"] for item in applied} != set(expected_files):
    raise SystemExit("deployed migration filenames differ from the candidate manifest")
legacy = []
for item in applied:
    expected = expected_files[item["filename"]]
    if item["checksum"] != expected["runner_checksum"]:
        if item["checksum"] not in expected.get("accepted_historical_checksums", []):
            raise SystemExit("deployed migration checksum mismatch: " + item["filename"])
        legacy.append(item)
(root / ".bot-migration-receipt.json").write_text(json.dumps(applied, indent=2) + "\n")
print(f"Verified all {len(applied)} deployed migration filenames and checksums; {manifest['existing_sql_files_unchanged']} historical files preserved; {len(legacy)} match existing compatibility rules.")
for item in legacy:
    print("Historical checksum:", item["filename"], item["checksum"])


environment = json.loads(subprocess.check_output(["docker", "inspect", "sub2api-staging", "--format", "{{json .Config.Env}}"], text=True))
checkin_admin_email = next((item.split("=", 1)[1].strip() for item in environment if item.startswith("MG_STAGING_CHECKIN_ADMIN_EMAIL=")), "")
if not checkin_admin_email:
    raise SystemExit("staging check-in administrator has not been configured")
email_hex = checkin_admin_email.encode().hex()
eligible_admin = sql(f"""
WITH target AS (SELECT convert_from(decode('{email_hex}', 'hex'), 'UTF8') AS email)
SELECT EXISTS (
  SELECT 1 FROM users u JOIN target ON u.email = target.email
  WHERE achievement_console_checkin_exempt(u.id, target.email)
);
""")
if eligible_admin != "t":
    raise SystemExit("configured staging check-in administrator is not active")
print("Staging administrator check-in eligibility verified (read-only).")
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
    public_fields = {"status", "request_key", "replayed", "checkin_date"}
    if expected == "ineligible":
        public_fields.update({"access_reason", "minimum_recharge_cny"})
    if expected == "checkin_unavailable":
        public_fields.add("access_reason")
    if set(payload) - public_fields:
        raise RuntimeError("group receipt includes private account fields")


def bind(identity, user_token, *, email=None):
    request = event(identity)
    if email:
        request["requested_email"] = email
    challenge = call("/integrations/bot/binding-challenges", token, request)
    duplicate = call("/integrations/bot/binding-challenges", token, request)
    assert duplicate["replayed"] and challenge["challenge_id"] == duplicate["challenge_id"]
    assert challenge["binding_url"].startswith(ORIGIN + "/chat-bindings/authorize/")
    challenge_path = "/user/chat-bindings/challenges/" + challenge["challenge_id"]
    preview = call(challenge_path, user_token, method="GET")
    assert preview["identity"] == identity
    other_session = next(item[1] for item in users if item[1] != user_token)
    confirmation = {"confirm": True, "account_confirmation": preview["account_confirmation"]}
    if email:
        assert preview["binding_mode"] == "email"
        assert email not in challenge["binding_url"]
        call(challenge_path, other_session, method="GET", expected=403)
        call("/integrations/bot/binding-challenges", token, {**request, "requested_email": "different@example.invalid"}, expected=409)
    call(challenge_path + "/authorize", other_session, confirmation, expected=409)
    authorization = call(challenge_path + "/authorize", user_token, confirmation)
    if email:
        assert authorization["status"] == "bound" and "code" not in authorization
        assert call(challenge_path, user_token, method="GET")["state"] == "completed"
        call(challenge_path + "/authorize", user_token, confirmation, expected=409)
        return
    wrong = {**identity, "sender_id": identity["sender_id"] + "-wrong"}
    call("/integrations/bot/bindings/confirm", token, {**event(wrong), "code": authorization["code"]}, expected=400)
    confirmation = {**event(identity), "code": authorization["code"]}
    assert_status(call("/integrations/bot/bindings/confirm", token, confirmation), "bound")
    assert call("/integrations/bot/bindings/confirm", token, confirmation)["replayed"]


def recharge(uid, principal):
    sql(f"INSERT INTO payment_orders(user_id,amount,pay_amount,expires_at,status,payment_type,provider_key,provider_snapshot,out_trade_no,paid_at,completed_at) VALUES({uid},{principal},{principal},now(),'COMPLETED','alipay','easypay','{{\"currency\":\"CNY\",\"recharge_principal\":{principal}}}','bot-smoke-{uuid.uuid4().hex}',now(),now());")


def verify_trial_credit(uid, qq, wx):
    path = "/integrations/bot/claim-trial-credit"
    request = event(qq)
    enabled = sql("SELECT enabled FROM chat_trial_credit_policy WHERE id;") == "t"
    receipt = call(path, token, request)
    if enabled:
        assert_status(receipt, "claimed")
        assert call(path, token, request) == {**receipt, "replayed": True}
        assert_status(call(path, token, event(wx)), "already_claimed")
        verified = sql(f"""
            SELECT c.amount=p.amount AND c.credited_amount=p.amount AND u.balance=c.credited_amount
               AND (achievement_access_snapshot(u.id)->>'qualified_recharge_cny')::numeric=0
               AND (SELECT count(*) FROM redeem_codes WHERE used_by=u.id AND type='trial_credit')=1
            FROM users u JOIN chat_trial_credit_claims c ON c.user_id=u.id
            CROSS JOIN chat_trial_credit_policy p WHERE u.id={uid} AND p.id;
        """)
        assert verified == "t"
        print("Trial credit HTTPS passed: QQ grant, event replay, WeChat duplicate, one credit entry and zero recharge principal.")
    else:
        assert_status(receipt, "disabled")
        assert sql(f"SELECT balance=0 AND NOT EXISTS (SELECT 1 FROM chat_trial_credit_claims WHERE user_id={uid}) FROM users WHERE id={uid};") == "t"
        print("Trial credit HTTPS passed with the staging policy disabled: no account was credited.")


try:
    for index in range(2):
        email = f"bot-acceptance-{prefix}-{index}@example.invalid"
        password = "synthetic-unusable-" + uuid.uuid4().hex
        created_at = "clock_timestamp()" if index == 0 else "now()-interval '2 days'"
        uid = int(sql(f"INSERT INTO users(email,password_hash,balance,created_at) VALUES('{email}','{password}',0,{created_at}) RETURNING id;"))
        users.append((uid, jwt(uid, email, password), email))
    (uid, user_token, user_email), (other_uid, other_token, _) = users
    minimum = json.loads(sql(f"SELECT achievement_access_snapshot({uid});"))["minimum_recharge_cny"]
    assert type(minimum) in (int, float) and minimum > 0
    qq, wx = identities
    call("/integrations/bot/binding-challenges", "invalid", event(qq), expected=401)
    call("/integrations/bot/binding-challenges", user_token, event(qq), expected=401)
    call("/user/chat-bindings", token, method="GET", expected=401)
    call("/integrations/bot/binding-challenges", token, event({**qq, "group_id": "outside-scope"}), expected=403)
    unbound = checkin(qq)
    assert_status(call("/integrations/bot/checkin", token, unbound), "not_bound")
    bind(qq, user_token)
    bind(wx, user_token, email=user_email)
    call("/integrations/bot/checkin", token, {**checkin(qq), "user_id": other_uid}, expected=400)
    unpaid = checkin(wx)
    rejected = call("/integrations/bot/checkin", token, unpaid)
    assert_status(rejected, "ineligible")
    assert rejected["access_reason"] == "recharge_required" and rejected["minimum_recharge_cny"] == minimum
    assert sql(f"SELECT count(*) FROM achievement_checkins WHERE user_id={uid};") == "0"
    assert sql(f"SELECT balance=0 FROM users WHERE id={uid};") == "t"
    verify_trial_credit(uid, qq, wx)
    recharge(uid, minimum)
    wallet_before = call("/user/balance", user_token, method="GET")
    assert call("/integrations/bot/checkin", token, unpaid) == {**rejected, "replayed": True}
    paid = checkin(qq)
    first = call("/integrations/bot/checkin", token, paid)
    assert_status(first, "checked_in")
    assert call("/integrations/bot/checkin", token, paid)["replayed"]
    assert_status(call("/integrations/bot/checkin", token, checkin(wx)), "already_checked_in")
    wallet_after = call("/user/balance", user_token, method="GET")
    monthly = wallet_after["checkin_credit"]
    start = dt.date.fromisoformat(monthly["month"] + "-01")
    beijing = dt.timezone(dt.timedelta(hours=8))
    next_month = dt.datetime(start.year + int(start.month == 12), start.month % 12 + 1, 1, tzinfo=beijing)
    assert dt.datetime.fromisoformat(monthly["expires_at"]) == next_month
    assert not wallet_after.get("legacy_checkin_credit") and wallet_after["frozen_balance"] == 0
    assert wallet_after["other_available"] == wallet_before["other_available"]
    spendable = Decimal(str(wallet_after["available_balance"]))
    assert spendable == Decimal(str(wallet_after["other_available"])) + Decimal(str(monthly["available"]))
    print("Wallet HTTPS passed: current-month cutoff, spendable total and other balance.")
    bindings = call("/user/chat-bindings", user_token, method="GET")
    qq_binding = next(item for item in bindings if item["identity"] == qq)
    call("/user/chat-bindings/" + str(qq_binding["id"]), other_token, method="DELETE", expected=404)
    call("/user/chat-bindings/" + str(qq_binding["id"]), user_token, method="DELETE")
    bind(qq, other_token)
    old = call("/integrations/bot/checkin", token, paid)
    assert old["replayed"] and old["request_key"] == first["request_key"]
    assert sql(f"SELECT count(*) FROM achievement_checkins WHERE user_id={other_uid};") == "0"
    recharge(other_uid, minimum)
    assert_status(call("/integrations/bot/checkin", token, checkin(qq)), "checked_in")
    old_day = {**checkin(wx), "event_at": (dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=1)).isoformat()}
    assert_status(call("/integrations/bot/checkin", token, old_day), "date_changed")
    assert sql(f"SELECT count(*) FROM achievement_checkins WHERE user_id IN ({uid},{other_uid});") == "2"
    print(f"Bot HTTPS acceptance passed: email confirmation, wrong-account denial, legacy QQ binding, {minimum} CNY eligibility reason, replay, unlink/rebind and Shanghai day.")
finally:
    if users:
        ids = ",".join(str(item[0]) for item in users)
        # The shared checkin path updates daily spent even when budget limits are off.
        sql(f"BEGIN; SELECT 1 FROM achievement_config WHERE id=true FOR UPDATE; UPDATE achievement_budget b SET spent=GREATEST(0,b.spent-c.gross) FROM (SELECT day,sum(gross) gross FROM achievement_checkins WHERE user_id IN ({ids}) GROUP BY day) c WHERE b.day=c.day; DELETE FROM chat_bot_events WHERE user_id IN ({ids}) OR identity_key IN (SELECT identity_key FROM chat_binding_challenges WHERE user_id IN ({ids})); DELETE FROM payment_orders WHERE user_id IN ({ids}); DELETE FROM redeem_codes WHERE type='trial_credit' AND used_by IN ({ids}); DELETE FROM users WHERE id IN ({ids}); COMMIT;")
        # Unbound/wrong-sender receipts have no user association; remove only this run's identities.
        for identity in identities:
            for sender in [identity["sender_id"], identity["sender_id"] + "-wrong"]:
                digest = hashlib.sha256(json.dumps([identity["platform"], identity["bot_id"], identity["group_id"], sender], separators=(",", ":")).encode()).hexdigest()
                sql(f"DELETE FROM chat_bot_events WHERE identity_key='{digest}'; DELETE FROM chat_binding_challenges WHERE identity_key='{digest}';")
        print("Synthetic accounts, payments and checkin budget contribution removed.")
