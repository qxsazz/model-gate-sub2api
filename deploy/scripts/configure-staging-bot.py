#!/usr/bin/env python3
"""Read the client array from stdin; configure only the staging .env without logging it."""
import json
import os
from pathlib import Path
import sys

path = Path(sys.argv[1])
if path.name != ".env" or path.parent.name != "sub2api-staging":
    raise SystemExit("expected the sub2api-staging .env")
clients = json.load(sys.stdin)
if not isinstance(clients, list) or not clients:
    raise SystemExit("explicit Bot client scopes required")
values = {
    "BOT_INTEGRATION_ENABLED": "true",
    "BOT_INTEGRATION_PUBLIC_URL": "https://staging.model-gate.cc",
    "BOT_INTEGRATION_CLIENTS_JSON": json.dumps(clients, separators=(",", ":")),
}
lines = [line for line in path.read_text().splitlines() if line.split("=", 1)[0] not in values]
for key, value in values.items():
    lines.append(key + "='" + value.replace("'", "\\'") + "'")
temporary = path.with_name(".env.bot-new")
fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
try:
    with os.fdopen(fd, "w") as output:
        output.write("\n".join(lines) + "\n")
    os.replace(temporary, path)
finally:
    temporary.unlink(missing_ok=True)
snapshot = path.parent / ".bot-clients.json"
fd = os.open(snapshot, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w") as output:
    output.write(json.dumps(clients) + "\n")
snapshot.chmod(0o600)
print("Staging Bot scopes configured; credentials omitted.")
