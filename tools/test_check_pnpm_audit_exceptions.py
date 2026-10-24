import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("check_pnpm_audit_exceptions.py")
spec = importlib.util.spec_from_file_location("audit_checker", SCRIPT)
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class AuditReportValidationTest(unittest.TestCase):
    def test_completed_empty_pnpm_and_npm_reports_are_valid(self):
        for inventory in ["advisories", "vulnerabilities"]:
            self.assertIsNone(checker.validate_audit_report({
                inventory: {}, "metadata": {"vulnerabilities": {"high": 0, "critical": 0}}
            }))

    def test_registry_errors_missing_inventory_and_incomplete_reports_fail(self):
        for report in [None, [], {}, {"error": {"code": "ECONNRESET"}},
                       {"advisories": {}},
                       {"advisories": {}, "metadata": {"vulnerabilities": {"high": 1, "critical": 0}}},
                       {"advisories": {}, "metadata": {"vulnerabilities": {"high": "0", "critical": 0}}}]:
            with self.subTest(report=report):
                self.assertIsNotNone(checker.validate_audit_report(report))

    def test_cli_refuses_registry_error_instead_of_passing_exceptions(self):
        with tempfile.TemporaryDirectory() as directory:
            report = Path(directory) / "audit.json"
            exceptions = Path(directory) / "exceptions.yml"
            report.write_text(json.dumps({"error": {"code": "ECONNRESET"}}))
            exceptions.write_text("version: 1\nexceptions:\n")
            result = subprocess.run([sys.executable, str(SCRIPT), "--audit", str(report),
                                     "--exceptions", str(exceptions)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 1)
            self.assertIn("did not complete", result.stderr)
            self.assertNotIn("validated", result.stdout)

    def test_real_high_advisory_can_be_checked_by_exception_policy(self):
        report = {"advisories": {"1": {"module_name": "test", "severity": "high",
                   "github_advisory_id": "GHSA-test"}},
                  "metadata": {"vulnerabilities": {"high": 1, "critical": 0}}}
        self.assertIsNone(checker.validate_audit_report(report))


if __name__ == "__main__":
    unittest.main()
