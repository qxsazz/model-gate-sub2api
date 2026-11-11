#!/usr/bin/env python3
"""Create a scoped Bot credential in private files; never print the bearer token."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--client-id", required=True)
parser.add_argument("--scopes-file", type=Path, required=True)
parser.add_argument("--output-dir", type=Path, required=True)
args = parser.parse_args()
scopes = json.loads(args.scopes_file.read_text())
if not isinstance(scopes, list) or not scopes:
    parser.error("a nonempty explicit scope array is required")
for scope in scopes:
    if scope.get("platform") not in ("qq_official", "wechat_native") or not scope.get("bot_id") or scope["bot_id"] == "*" or not scope.get("group_ids") or "*" in scope["group_ids"]:
        parser.error("each scope needs an exact platform, Bot ID and group IDs")
args.output_dir.mkdir(mode=0o700, parents=True, exist_ok=False)
token = "mgbot_" + secrets.token_urlsafe(32)
client = {"id": args.client_id, "token_sha256": hashlib.sha256(token.encode()).hexdigest(), "scopes": scopes}
for name, content in [("bot-token", token + "\n"), ("clients.json", json.dumps([client], indent=2) + "\n")]:
    fd = os.open(args.output_dir / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as output:
        output.write(content)
print("Created private bot-token and clients.json in", args.output_dir)
