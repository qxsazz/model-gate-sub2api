"""Observe potential backend CI reuse. This report never supplies a required check."""

import json
import os
import subprocess
from pathlib import Path

REQUIRED_STEPS = {
    "test": {"Verify Go version", "Unit tests", "Integration tests"},
    "golangci-lint": {"Verify Go version", "golangci-lint"},
}


def git(*args):
    return subprocess.check_output(["git", *args], text=True).strip()


def comparable_commit(event_name, ref, event):
    head = git("rev-parse", "HEAD")
    if event_name == "pull_request":
        candidate = event["pull_request"]["head"]["sha"]
    elif ref == "refs/heads/main":
        parents = git("show", "-s", "--format=%P", "HEAD").split()
        candidate = parents[1] if len(parents) == 2 else head
    else:
        candidate = head
    # The complete tree includes the workflow, Go toolchain declaration and lint config.
    if git("rev-parse", candidate + "^{tree}") == git("rev-parse", "HEAD^{tree}"):
        return candidate
    return None


def qualifying_jobs(jobs):
    links = {}
    for job in jobs:
        name = job["name"]
        if name not in REQUIRED_STEPS or job["conclusion"] != "success":
            continue
        executed = {step["name"] for step in job["steps"] if step["conclusion"] == "success"}
        if REQUIRED_STEPS[name] <= executed and "ubuntu-latest" in job["labels"]:
            links[name] = job["html_url"]
    return links if links.keys() == REQUIRED_STEPS.keys() else None


def api(repo, path):
    raw = subprocess.check_output(["gh", "api", f"repos/{repo}/actions/{path}"], text=True, timeout=30)
    return json.loads(raw)


def compare(repo, run_id, candidate):
    if not candidate:
        return "The tested merge tree differs from the candidate."
    current = api(repo, f"runs/{run_id}")
    runs = api(repo, f"workflows/backend-ci.yml/runs?event=push&head_sha={candidate}&status=success&per_page=100")
    for run in runs["workflow_runs"]:
        branch = run["head_branch"] or ""
        if (run["id"] == int(run_id) or run["head_sha"] != candidate
                or (run.get("head_repository") or {}).get("full_name") != repo
                or run["updated_at"] >= current["created_at"]
                or not (branch == "staging" or branch.startswith("ci/"))):
            continue
        jobs = api(repo, f"runs/{run['id']}/jobs?per_page=100")
        links = qualifying_jobs(jobs["jobs"])
        if links:
            return (f"Earlier run with a matching tree and completed suites: [CI]({run['html_url']}), "
                    f"[Go tests]({links['test']}), [lint]({links['golangci-lint']}).")
    return "No earlier completed candidate or staging run with matching Go tests and lint was found."


def main():
    try:
        event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
        candidate = comparable_commit(os.environ["GITHUB_EVENT_NAME"], os.environ["GITHUB_REF"], event)
        result = compare(os.environ["GITHUB_REPOSITORY"], os.environ["GITHUB_RUN_ID"], candidate)
    except (subprocess.SubprocessError, OSError, KeyError, ValueError):
        result = "Comparison unavailable. See the check log."
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
        print("### Backend CI reuse observation\n", file=summary)
        print(result, file=summary)
        print(f"\nCurrent run: Go tests **{os.environ['TEST_RESULT']}**, lint **{os.environ['LINT_RESULT']}**.", file=summary)


if __name__ == "__main__":
    main()
