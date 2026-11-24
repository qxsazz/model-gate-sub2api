"""Select CI jobs from the complete diff of the tree that Actions will test."""

import json
import os
import subprocess
from pathlib import Path

SUITES = {"backend", "frontend", "canvas", "apple", "compose", "release"}


def suites_for(path):
    if path.startswith("backend/"):
        suites = {"backend"}
        if path.startswith("backend/internal/pkg/adminaccess/"):
            suites.add("frontend")
        if path.startswith("backend/resources/") or path == "backend/cmd/server/VERSION":
            suites.update(("compose", "release"))
        return suites
    if path == "frontend/src/constants/platformCatalog.builtin.json":
        return {"backend", "frontend"}
    doc_test_dirs = ("frontend/src/docs/__tests__/", "frontend/src/components/docs/__tests__/",
                     "frontend/src/views/docs/__tests__/")
    if (path == "frontend/src/docs/registry.ts"
            or path.startswith("frontend/src/content/docs/") and path.endswith(".md")
            or path.startswith(doc_test_dirs) and path.endswith(".spec.ts")
            or path in ("frontend/src/router/__tests__/docs-route.spec.ts",
                        "frontend/src/components/layout/__tests__/docUrlSanitization.spec.ts")):
        return {"docs"}
    if path.startswith(("frontend/", "docs/legal/")):
        return {"frontend"}
    if path.startswith("canvas/"):
        return {"canvas", "frontend"}
    if path in ("deploy/apple-container.sh", "deploy/tests/apple-container-test.sh") or path.startswith("deploy/tests/fixtures/bin/"):
        return {"apple"}
    if path == "deploy/.env.example":
        return {"apple", "compose", "release"}
    if path in ("deploy/Dockerfile", "deploy/Makefile"):
        return SUITES
    if path.startswith("deploy/sql/"):
        return {"compose", "release", "backend"}
    if path.startswith("deploy/"):
        return {"compose", "release"}
    if path.startswith((".github/release-tools/", "tools/")) or path == "docs/bot-upgrade-migrations.json":
        return {"release"}
    if path.endswith(".md") and ("/" not in path or path.startswith(("docs/", "openspec/"))):
        return set()
    # Workflow definitions, build inputs and unfamiliar files get the full suite.
    return SUITES


def select(paths):
    suites = set() if paths else SUITES.copy()
    for path in paths:
        suites.update(suites_for(path))
    result = {name: str(name in suites).lower() for name in SUITES}
    result["frontend"] = "full" if "frontend" in suites else "docs" if "docs" in suites else "none"
    return result


def changed_paths(event_name, ref, event):
    if event_name == "pull_request":
        base = event["pull_request"]["base"]["sha"]
    elif event_name == "push" and ref in ("refs/heads/staging", "refs/heads/main"):
        base = event.get("before")
    elif event_name == "push" and ref.startswith(("refs/heads/", "refs/tags/ci/")):
        # Measure the entire candidate, including earlier commits on its branch.
        base = "refs/remotes/origin/staging"
    else:
        return [], "no applicable comparison"
    if not base or not base.strip("0"):
        return [], "missing base"
    try:
        subprocess.run(["git", "merge-base", "--is-ancestor", base, "HEAD"], check=True)
        names = subprocess.check_output(["git", "diff", "--name-only", "--no-renames", "-z", base, "HEAD", "--"])
    except subprocess.CalledProcessError:
        return [], "base unavailable or diverged"
    paths = [os.fsdecode(name) for name in names.split(b"\0") if name]
    return paths, f"{len(paths)} changed paths" if paths else "empty diff"


def main():
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    paths, reason = changed_paths(os.environ["GITHUB_EVENT_NAME"], os.environ["GITHUB_REF"], event)
    result = select(paths)
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        for key, value in sorted(result.items()):
            print(f"{key}={value}", file=output)
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
        print(f"CI selection ({reason})\n\n| Suite | Selection |\n| --- | --- |", file=summary)
        for key, value in sorted(result.items()):
            print(f"| {key} | {value} |", file=summary)


if __name__ == "__main__":
    main()
