#!/usr/bin/env python3
"""Provision a synthetic-only credential on the staging host, never in CI logs."""
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys

root = Path(sys.argv[1])
if root.name != "sub2api-staging":
    raise SystemExit("staging directory required")
snapshot = root / ".bot-clients.json"
if not snapshot.exists() and any(line.startswith("BOT_INTEGRATION_CLIENTS_JSON=") for line in (root / ".env").read_text().splitlines()):
    raise SystemExit("existing Bot configuration needs explicit migration; refusing to overwrite scopes")
clients = json.loads(snapshot.read_text()) if snapshot.exists() else []
token_path = root / ".bot-smoke-token"
if token_path.exists():
    token = token_path.read_text().strip()
    if not token.startswith("mgbot_") or len(token) < 40:
        raise SystemExit("invalid saved synthetic credential")
else:
    token = "mgbot_" + secrets.token_urlsafe(32)
    fd = os.open(token_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as output:
        output.write(token + "\n")
token_path.chmod(0o600)
client = {"id": "staging-synthetic", "token_sha256": hashlib.sha256(token.encode()).hexdigest(), "scopes": [
    {"platform": "qq_official", "bot_id": "mg-synthetic-qq", "group_ids": ["mg-synthetic-group"]},
    {"platform": "wechat_native", "bot_id": "mg-synthetic-wx", "group_ids": ["mg-synthetic-group"]},
]}
existing = next((item for item in clients if item["id"] == client["id"]), None)
if existing is not None and existing != client:
    raise SystemExit("synthetic client does not match saved credential; explicit rotation required")
if existing is None:
    clients.append(client)
subprocess.run([sys.executable, str(root / "configure-staging-bot.py"), str(root / ".env")], input=json.dumps(clients), text=True, check=True)
print("Synthetic credential is confined to the staging host (0600); only synthetic groups allowed.")
