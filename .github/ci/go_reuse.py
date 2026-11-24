"""Reuse a real Go check from an earlier trusted push of the same tree and toolchain."""

import hashlib
import json
import os
import subprocess
from datetime import datetime, timedelta
from pathlib import Path

WORKFLOW = ".github/workflows/backend-ci.yml"
REQUIRED = {
    "test": {"Set up Go for CI", "Unit tests", "Integration tests"},
    "golangci-lint": {"Set up Go for CI", "golangci-lint"},
}


def git(*args):
    return subprocess.check_output(["git", *args], text=True).strip()


def comparable_commit(event_name, ref, event):
    head = git("rev-parse", "HEAD")
    if event_name == "pull_request":
        pr = event["pull_request"]
        if (pr["base"]["ref"] != "main" or pr["head"]["ref"] != "staging"
                or pr["base"]["repo"]["id"] != pr["head"]["repo"]["id"]):
            return None
        candidate = pr["head"]["sha"]
        if git("rev-parse", "HEAD^2") != candidate:
            return None
    elif event_name == "push" and ref == "refs/heads/main":
        parents = git("show", "-s", "--format=%P", "HEAD").split()
        candidate = parents[1] if len(parents) == 2 else head
    elif event_name == "push" and ref == "refs/heads/staging":
        return head
    else:
        return None

    return candidate if git("rev-parse", candidate + "^{tree}") == git("rev-parse", "HEAD^{tree}") else None


def evidence_key(job, tree, env):
    # The tree includes go.mod, go.sum, pinned CI actions, lint version and all test inputs.
    fields = [tree, env["RUNNER_OS"], env["RUNNER_ARCH"], env["ImageOS"], env["ImageVersion"]]
    if not all(fields) or env["CI_RUNNER_ENVIRONMENT"] != "github-hosted":
        raise ValueError("missing hosted runner identity")
    digest = hashlib.sha256("\n".join(fields).encode()).hexdigest()
    return f"go-ci-{job}-{digest}"


def api(repo, path):
    data = subprocess.check_output(["gh", "api", f"repos/{repo}/actions/{path}"], text=True, timeout=12)
    return json.loads(data)


def eligible(run, current, sha, repo_id):
    branch = run["head_branch"] or ""
    elapsed = datetime.fromisoformat(current["created_at"]) - datetime.fromisoformat(run["updated_at"])
    return (run["id"] != current["id"] and run["workflow_id"] == current["workflow_id"]
            and run["path"] == WORKFLOW and run["head_sha"] == sha
            and run["event"] == "push" and run["status"] == "completed" and run["conclusion"] == "success"
            and run["repository"]["id"] == repo_id and (run.get("head_repository") or {}).get("id") == repo_id
            and (branch == "staging" or branch.startswith("ci/")) and timedelta(0) < elapsed <= timedelta(days=1))


def previous_job(repo, repo_id, run_id, sha, job, key):
    current = api(repo, f"runs/{run_id}")
    if current["path"] != WORKFLOW:
        return None
    runs = api(repo, f"workflows/backend-ci.yml/runs?event=push&head_sha={sha}&status=success&per_page=100")
    candidates = (run for run in runs["workflow_runs"] if eligible(run, current, sha, repo_id))
    for index, run in enumerate(candidates):
        if index >= 5:
            break
        # Artifacts are scoped to the run. The attempt suffix excludes receipts from an earlier retry.
        attempt = run["run_attempt"]
        name = f"{key}-a{attempt}"
        artifacts = api(repo, f"runs/{run['id']}/artifacts?name={name}")["artifacts"]
        if not any(a["name"] == name and not a["expired"] for a in artifacts):
            continue
        jobs = api(repo, f"runs/{run['id']}/attempts/{attempt}/jobs?per_page=100")["jobs"]
        for prior in jobs:
            if (prior["name"] != job or prior["conclusion"] != "success"
                    or prior["run_attempt"] != attempt or "ubuntu-latest" not in prior["labels"]):
                continue
            steps = {step["name"] for step in prior["steps"] if step["conclusion"] == "success"}
            if (REQUIRED[job] | {"Publish Go check evidence"}) <= steps:
                return prior["html_url"]
    return None


def plan(env):
    job, ref, event_name = env["GITHUB_JOB"], env["GITHUB_REF"], env["GITHUB_EVENT_NAME"]
    head = git("rev-parse", "HEAD")
    if job not in REQUIRED or head != env["GITHUB_SHA"]:
        return None, "", "Checkout or job identity differs."
    tree = git("rev-parse", "HEAD^{tree}")
    key = evidence_key(job, tree, env)
    attempt = env["GITHUB_RUN_ATTEMPT"]
    event = json.loads(Path(env["GITHUB_EVENT_PATH"]).read_text())
    reason = "Run this check in full."
    if attempt == "1":
        source = comparable_commit(event_name, ref, event)
        if source:
            try:
                link = previous_job(env["GITHUB_REPOSITORY"], int(env["GITHUB_REPOSITORY_ID"]),
                                    env["GITHUB_RUN_ID"], source, job, key)
                if link:
                    return link, "", "Prior check verified."
                reason = "No matching completed check."
            except (subprocess.SubprocessError, OSError, KeyError, ValueError, TypeError):
                reason = "Could not verify the prior run."

    evidence = ""
    if event_name == "push" and (ref == "refs/heads/staging" or ref.startswith("refs/tags/ci/")):
        # The workflow publishes this only after this job executes its actual suite successfully.
        evidence = f"{key}-a{attempt}"
        receipt = {"job": job, "sha": head, "tree": tree, "ref": ref, "run_id": env["GITHUB_RUN_ID"],
                   "attempt": attempt, "runner": f"{env['ImageOS']}/{env['ImageVersion']}/{env['RUNNER_ARCH']}"}
        Path(env["RUNNER_TEMP"], "go-ci-evidence.json").write_text(json.dumps(receipt, indent=2))
    return None, evidence, reason


def main():
    try:
        link, evidence, reason = plan(os.environ)
    except (subprocess.SubprocessError, OSError, KeyError, ValueError, TypeError):
        link, evidence, reason = None, "", "Could not establish the check identity."
    message = f"Reused [the completed {os.environ['GITHUB_JOB']} job]({link})." if link else f"{reason} Running the suite."
    print(message)
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
        print(message, file=summary)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        print(f"reused={str(bool(link)).lower()}\nevidence={evidence}", file=output)


if __name__ == "__main__":
    main()
