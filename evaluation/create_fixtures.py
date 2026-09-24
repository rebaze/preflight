#!/usr/bin/env python3
"""Create only synthetic, disposable inspection fixtures in a new directory."""

import argparse
import json
from pathlib import Path
import subprocess


def git(repo, *args):
    return subprocess.run(
        ["git", "-c", "core.hooksPath=/dev/null", "-C", str(repo), *args],
        check=True, capture_output=True, text=True,
    ).stdout.strip()


def repository(root, name, files):
    repo = root / name
    repo.mkdir()
    for relative, content in files.items():
        path = repo / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
    git(repo, "init", "--initial-branch=main")
    git(repo, "add", ".")
    git(repo, "-c", "user.name=Synthetic Evaluation", "-c", "user.email=fixture@example.invalid",
        "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "Synthetic baseline")
    # A tripwire for accidental execution; inspection must never invoke it.
    hook = repo / ".git/hooks/post-checkout"
    hook.write_text("#!/bin/sh\ntouch HOOK_EXECUTED\n")
    hook.chmod(0o755)
    return repo


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path, help="new private directory outside any project")
    args = parser.parse_args()
    root = args.destination.absolute()
    root.mkdir(mode=0o700, parents=True, exist_ok=False)
    repository(root, "go-service", {
        "README.md": "# Synthetic catalog\nA tiny JSON API fixture.\n",
        "CONTRIBUTING.md": "# API changes\n\nPreserve existing response fields for older clients. Add contract coverage before changing response semantics.\n",
        "go.mod": "module example.invalid/catalog\n\ngo 1.24\n",
        "internal/api/item.go": "package api\ntype Item struct { ID string; Label string }\n",
        "internal/api/item_test.go": "package api\n// Synthetic test placeholder; no test has run.\n",
        "Makefile": "test:\n\ttouch SCRIPT_EXECUTED\n",
    })
    js = repository(root, "js-workspace", {
        "README.md": "# Synthetic search workspace\nThe web UI is in clients/browser.\n",
        "CONTRIBUTING.md": "# Search UI changes\n\nKeep keyboard navigation available when changing search results. Add a focused accessibility test for interaction changes.\n",
        "package.json": json.dumps({"name": "synthetic-search", "private": True,
            "workspaces": ["clients/*"], "scripts": {"test": "touch SCRIPT_EXECUTED"}}, indent=2) + "\n",
        "clients/browser/package.json": json.dumps({"name": "@example/browser", "private": True,
            "scripts": {"test": "touch SCRIPT_EXECUTED"}}, indent=2) + "\n",
        "clients/browser/src/results.js": "export const keyboardNavigation = true;\n",
        ".github/workflows/ci.yml": "name: checks\non: [pull_request]\njobs:\n  ui:\n    runs-on: ubuntu-latest\n    steps:\n      - run: npm test\n",
    })
    (js / "clients/browser/src/results.js").write_text("export const keyboardNavigation = false;\n")
    (js / "clients/browser/src/new-panel.js").write_text("export const panel = {};\n")
    repository(root, "empty", {})
    scenarios = [
        {"name": "go-compatibility", "repo": "go-service", "prompt": "$preflight I plan to remove Label from the item API response. What should I know before editing?",
         "expected_source": "CONTRIBUTING.md:3", "expectation": "Preserve existing response fields", "next_action": "inspect contract coverage"},
        {"name": "js-keyboard", "repo": "js-workspace", "prompt": "$preflight I am changing search results to mouse-only selection. What should I know before editing?",
         "expected_source": "CONTRIBUTING.md:3", "expectation": "Keep keyboard navigation", "next_action": "retain keyboard interaction and examine accessibility coverage"},
        {"name": "empty", "repo": "empty", "prompt": "$preflight I plan to add a documentation page. What should I know before editing?",
         "expected_source": None, "expectation": "no invented expectation", "next_action": "identify relevant conventions if any"},
    ]
    (root / "scenarios.json").write_text(json.dumps(scenarios, indent=2) + "\n")
    print(root)


if __name__ == "__main__":
    main()
