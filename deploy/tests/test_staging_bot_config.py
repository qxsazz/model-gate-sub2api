import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"


class StagingBotConfigTests(unittest.TestCase):
    def test_bootstrap_is_private_repeatable_and_preserves_custom_scopes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "sub2api-staging"
            root.mkdir()
            (root / ".env").write_text("JWT_SECRET=unrelated-test-value\n")
            shutil.copy(SCRIPTS / "configure-staging-bot.py", root)
            def run(name, body=None):
                return subprocess.run([sys.executable, str(SCRIPTS / name), str(root / ".env" if name.startswith("configure") else root)], input=body, text=True, capture_output=True, check=True)
            first = run("bootstrap-staging-bot.py")
            token = (root / ".bot-smoke-token").read_text().strip()
            self.assertNotIn(token, first.stdout + first.stderr)
            clients = json.loads((root / ".bot-clients.json").read_text())
            self.assertEqual(clients[0]["token_sha256"], hashlib.sha256(token.encode()).hexdigest())
            clients.append({"id": "real-test-client", "token_sha256": "ab" * 32, "scopes": [{"platform": "qq_official", "bot_id": "real-bot", "group_ids": ["real-group"]}]})
            run("configure-staging-bot.py", json.dumps(clients))
            run("bootstrap-staging-bot.py")
            self.assertEqual((root / ".bot-smoke-token").read_text().strip(), token)
            self.assertEqual(json.loads((root / ".bot-clients.json").read_text()), clients)
            self.assertIn("JWT_SECRET=unrelated-test-value", (root / ".env").read_text())
            for file in [".env", ".bot-smoke-token", ".bot-clients.json"]:
                self.assertEqual((root / file).stat().st_mode & 0o777, 0o600)

    def test_refuses_unknown_existing_scopes_and_production_directory(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "sub2api-staging"
            root.mkdir()
            (root / ".env").write_text("BOT_INTEGRATION_CLIENTS_JSON=existing\n")
            for target in [root, root.parent / "sub2api"]:
                result = subprocess.run([sys.executable, str(SCRIPTS / "bootstrap-staging-bot.py"), str(target)], capture_output=True)
                self.assertNotEqual(result.returncode, 0)
            self.assertEqual((root / ".env").read_text(), "BOT_INTEGRATION_CLIENTS_JSON=existing\n")
