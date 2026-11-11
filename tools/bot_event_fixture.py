#!/usr/bin/env python3
"""Print a new synthetic event. Reuse its exact output for retry tests."""
import argparse
import datetime as dt
import json
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("platform", choices=["qq_official", "wechat_native"])
parser.add_argument("--sender-id", default="synthetic-local-member")
parser.add_argument("--checkin", action="store_true")
args = parser.parse_args()
body = {"identity": {"platform": args.platform, "bot_id": "mg-synthetic-qq" if args.platform == "qq_official" else "mg-synthetic-wx", "group_id": "mg-synthetic-group", "sender_id": args.sender_id}, "event_id": uuid.uuid4().hex}
if args.checkin:
    body["event_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
print(json.dumps(body, indent=2))
