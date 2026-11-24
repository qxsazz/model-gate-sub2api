import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from go_reuse import REQUIRED, comparable_commit, evidence_key, plan, previous_job
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
        event = {"pull_request": {
            "base": {"ref": "main", "sha": self.base, "repo": {"id": 1}},
            "head": {"ref": "staging", "sha": self.middle, "repo": {"id": 1}},
        }}
        paths, _ = changed_paths("pull_request", "refs/pull/1/merge", event)
        self.assertIn("backend/main.go", paths)
        self.assertIsNone(comparable_commit("push", "refs/heads/feature", {}))
        self.assertEqual(comparable_commit("push", "refs/heads/staging", {}), self.head)
        for ref, before in (("refs/heads/staging", "0" * 40), ("refs/heads/main", self.head), ("refs/tags/v1", self.base)):
            self.assertEqual(changed_paths("push", ref, {"before": before})[0], [])
        self.git("checkout", "-q", self.base)
        self.assertEqual(changed_paths("push", "refs/heads/main", {"before": self.head})[0], [])

    def test_main_merge_only_matches_an_identical_staging_tree(self):
        self.git("checkout", "-q", self.base)
        self.git("merge", "--no-ff", "-qm", "promote", self.head)
        self.assertEqual(comparable_commit("push", "refs/heads/main", {}), self.head)


    def test_pr_requires_same_repo_staging_head_and_matching_merge_tree(self):
        self.git("checkout", "-q", self.base)
        self.git("merge", "--no-ff", "-qm", "promote", self.head)
        pr = {"base": {"ref": "main", "repo": {"id": 1}},
              "head": {"ref": "staging", "sha": self.head, "repo": {"id": 1}}}
        def candidate():
            return comparable_commit("pull_request", "refs/pull/1/merge", {"pull_request": pr})
        self.assertEqual(candidate(), self.head)
        pr["head"]["repo"]["id"] = 2
        self.assertIsNone(candidate())
        pr["head"]["repo"]["id"] = 1
        pr["head"]["sha"] = self.middle
        self.assertIsNone(candidate())
        pr["head"]["sha"] = self.head
        Path("base-only.txt").write_text("merge differs from candidate")
        self.git("add", "-A")
        self.git("commit", "--amend", "-qm", "merged with base change")
        self.assertIsNone(candidate())

    def test_only_real_check_on_trusted_push_can_publish_evidence(self):
        Path("event.json").write_text("{}")
        env = {"CI_RUNNER_ENVIRONMENT": "github-hosted", "RUNNER_OS": "Linux", "ImageOS": "ubuntu26",
               "ImageVersion": "100", "RUNNER_ARCH": "X64", "RUNNER_TEMP": ".", "GITHUB_EVENT_PATH": "event.json",
               "GITHUB_SHA": self.head, "GITHUB_RUN_ATTEMPT": "1", "GITHUB_RUN_ID": "3",
               "GITHUB_JOB": "test", "GITHUB_REPOSITORY_ID": "1", "GITHUB_REPOSITORY": "org/repo",
               "GITHUB_EVENT_NAME": "push", "GITHUB_REF": "refs/tags/ci/candidate"}
        link, evidence, _ = plan(env)
        self.assertFalse(link)
        self.assertTrue(evidence)
        self.assertTrue(Path("go-ci-evidence.json").is_file())
        self.assertFalse(plan(env | {"GITHUB_REF": "refs/heads/ci/candidate"})[1])
        self.assertFalse(plan(env | {"GITHUB_SHA": self.base})[1])
        staging = env | {"GITHUB_REF": "refs/heads/staging"}
        with patch("go_reuse.previous_job", return_value="https://example.test/job") as lookup:
            self.assertEqual(plan(staging)[:2], ("https://example.test/job", ""))
            self.assertFalse(plan(staging | {"GITHUB_RUN_ATTEMPT": "2"})[0])
            self.assertEqual(lookup.call_count, 1)
        with patch("go_reuse.previous_job", side_effect=subprocess.TimeoutExpired("gh", 12)):
            self.assertFalse(plan(staging)[0])

    def test_runner_and_tree_are_part_of_evidence(self):
        env = {"RUNNER_OS": "Linux", "RUNNER_ARCH": "X64", "ImageOS": "ubuntu26", "ImageVersion": "100",
               "CI_RUNNER_ENVIRONMENT": "github-hosted"}
        key = evidence_key("test", self.head, env)
        for change in ({"ImageVersion": "101"}, {"RUNNER_ARCH": "ARM64"}):
            self.assertNotEqual(key, evidence_key("test", self.head, env | change))
        self.assertNotEqual(key, evidence_key("test", self.base, env))
        self.assertNotEqual(key, evidence_key("golangci-lint", self.head, env))


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.current = {"id": 2, "workflow_id": 7, "path": ".github/workflows/backend-ci.yml",
                        "created_at": "2026-01-03T00:00:00Z"}
        self.prior = self.current | {"id": 1, "run_attempt": 1, "event": "push", "head_branch": "ci/release",
                                    "status": "completed", "conclusion": "success", "head_sha": "sha",
                                    "repository": {"id": 5}, "head_repository": {"id": 5},
                                    "updated_at": "2026-01-02T23:00:00Z"}
        self.receipt = {"name": "go-ci-test-key-a1", "expired": False}
        self.job = {"name": "test", "conclusion": "success", "run_attempt": 1, "labels": ["ubuntu-latest"],
                    "html_url": "https://example.test/job", "steps": [
                        {"name": name, "conclusion": "success"} for name in REQUIRED["test"] | {"Publish Go check evidence"}
                    ]}

    def lookup(self, *responses):
        with patch("go_reuse.api", side_effect=[self.current, *responses]):
            return previous_job("org/repo", 5, "2", "sha", "test", "go-ci-test-key")

    def test_prior_run_and_job_must_match(self):
        runs = {"workflow_runs": [self.prior]}
        artifacts = {"artifacts": [self.receipt]}
        self.assertEqual(self.lookup(runs, artifacts, {"jobs": [self.job]}), self.job["html_url"])
        for changes in ({"head_sha": "other"}, {"head_repository": {"id": 6}}, {"workflow_id": 8},
                        {"event": "pull_request"}, {"conclusion": "failure"}, {"head_branch": "unrelated"},
                        {"updated_at": "2026-01-03T01:00:00Z"}, {"updated_at": "2026-01-01T00:00:00Z"}):
            with self.subTest(changes=changes):
                self.assertIsNone(self.lookup({"workflow_runs": [self.prior | changes]}))
        for receipt in (self.receipt | {"expired": True}, self.receipt | {"name": "go-ci-test-key-a2"}):
            self.assertIsNone(self.lookup(runs, {"artifacts": [receipt]}))
        for changes in ({"run_attempt": 2}, {"labels": ["self-hosted"]}, {"conclusion": "skipped"},
                        {"steps": [s | {"conclusion": "skipped"} for s in self.job["steps"]]}):
            self.assertIsNone(self.lookup(runs, artifacts, {"jobs": [self.job | changes]}))


if __name__ == "__main__":
    unittest.main()
