import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from ci_reuse_report import REQUIRED_STEPS, comparable_commit, compare, qualifying_jobs
from ci_scope import changed_paths, select


class ScopeTests(unittest.TestCase):
    def test_docs_and_mixed_changes(self):
        doc = "frontend/src/content/docs/tutorial/api-key.md"
        docs = [doc, "frontend/src/docs/registry.ts", "frontend/src/views/docs/__tests__/DocsView.spec.ts"]
        self.assertEqual(select(docs)["frontend"], "docs")
        self.assertEqual(select([doc])["backend"], "false")
        mixed = select([doc, "backend/go.mod", "frontend/src/main.ts"])
        self.assertEqual((mixed["frontend"], mixed["backend"]), ("full", "true"))
        self.assertEqual(select(["docs/guide.md"])["frontend"], "none")

    def test_shared_inputs(self):
        for path, suites in {
            "frontend/src/constants/platformCatalog.builtin.json": ("frontend", "backend"),
            "backend/internal/pkg/adminaccess/catalog.go": ("frontend", "backend"),
            "canvas/web/src/lib/config-access.ts": ("frontend", "canvas"),
            "docs/legal/privacy.md": ("frontend",),
            "deploy/.env.example": ("apple", "compose", "release"),
            "deploy/sql/verify.sql": ("backend", "compose"),
            "backend/resources/prices.json": ("backend", "compose", "release"),
        }.items():
            with self.subTest(path=path):
                selected = select([path])
                for suite in suites:
                    self.assertIn(selected[suite], ("true", "full"))
        self.assertEqual(select(["deploy/Caddyfile"])["apple"], "false")
        self.assertEqual(select(["deploy/tests/apple-container-test.sh"])["compose"], "false")

    def test_unknown_and_empty_run_everything(self):
        for paths in ([], [".github/ci/ci_scope.py"], ["new-build-system"], [".github/workflows/frontend-ci.yml"]):
            self.assertEqual(set(select(paths).values()), {"true", "full"})


class GitComparisonTests(unittest.TestCase):
    def setUp(self):
        folder = tempfile.TemporaryDirectory()
        self.addCleanup(folder.cleanup)
        self.addCleanup(os.chdir, os.getcwd())
        os.chdir(folder.name)
        self.git("init", "-q")
        self.git("config", "user.email", "ci@example.invalid")
        self.git("config", "user.name", "CI tests")
        Path("base.md").write_text("base")
        self.base = self.commit()
        self.git("update-ref", "refs/remotes/origin/staging", self.base)
        Path("backend").mkdir()
        Path("backend/main.go").write_text("earlier change")
        self.middle = self.commit()
        self.git("mv", "base.md", "guide.md")
        self.head = self.commit()

    def git(self, *args):
        return subprocess.check_output(["git", *args], text=True).strip()

    def commit(self):
        self.git("add", "-A")
        self.git("commit", "-qm", "fixture")
        return self.git("rev-parse", "HEAD")

    def test_scope_uses_entire_candidate_and_both_sides_of_rename(self):
        expected = {"backend/main.go", "base.md", "guide.md"}
        for ref in ("refs/tags/ci/candidate", "refs/heads/feature"):
            paths, _ = changed_paths("push", ref, {"before": self.middle})
            self.assertEqual(set(paths), expected)
        for ref in ("refs/heads/staging", "refs/heads/main"):
            paths, _ = changed_paths("push", ref, {"before": self.middle})
            self.assertEqual(set(paths), {"base.md", "guide.md"})

    def test_pr_diff_and_ambiguous_bases(self):
        event = {"pull_request": {"base": {"sha": self.base}, "head": {"sha": self.middle}}}
        paths, _ = changed_paths("pull_request", "refs/pull/1/merge", event)
        self.assertIn("backend/main.go", paths)
        self.assertIsNone(comparable_commit("pull_request", "refs/pull/1/merge", event))
        self.assertEqual(comparable_commit("push", "refs/heads/staging", {}), self.head)
        for ref, before in (("refs/heads/staging", "0" * 40), ("refs/heads/main", self.head), ("refs/tags/v1", self.base)):
            self.assertEqual(changed_paths("push", ref, {"before": before})[0], [])
        self.git("checkout", "-q", self.base)
        self.assertEqual(changed_paths("push", "refs/heads/main", {"before": self.head})[0], [])

    def test_main_merge_only_matches_an_identical_staging_tree(self):
        self.git("checkout", "-q", self.base)
        self.git("merge", "--no-ff", "-qm", "promote", self.head)
        self.assertEqual(comparable_commit("push", "refs/heads/main", {}), self.head)


class ObservationTests(unittest.TestCase):
    def test_prior_run_requires_completed_go_steps(self):
        jobs = [
            {"name": name, "conclusion": "success", "html_url": "url", "labels": ["ubuntu-latest"],
             "steps": [{"name": step, "conclusion": "success"} for step in steps]}
            for name, steps in REQUIRED_STEPS.items()
        ]
        self.assertEqual(set(qualifying_jobs(jobs)), {"test", "golangci-lint"})
        jobs[0]["steps"][0]["conclusion"] = "skipped"
        self.assertIsNone(qualifying_jobs(jobs))

    def test_concurrent_and_unrelated_runs_are_ineligible(self):
        previous = {"id": 1, "head_sha": "candidate", "head_branch": "staging",
                    "updated_at": "2026-01-02", "head_repository": {"full_name": "org/repo"}}
        for changes in ({"updated_at": "2026-01-04"}, {"head_branch": "feature"}, {"head_sha": "other"}):
            results = [{"created_at": "2026-01-03"}, {"workflow_runs": [previous | changes]}]
            with patch("ci_reuse_report.api", side_effect=results) as api:
                self.assertIn("No earlier", compare("org/repo", "2", "candidate"))
                self.assertEqual(api.call_count, 2)


if __name__ == "__main__":
    unittest.main()
