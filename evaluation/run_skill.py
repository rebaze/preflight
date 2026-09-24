#!/usr/bin/env python3
"""Run synthetic skill scenarios in new read-only Codex sessions with private logs."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading
import time


def digests(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in root.rglob("*") if p.is_file()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--installation", type=Path, required=True)
    parser.add_argument("--cli", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new private directory outside fixtures")
    parser.add_argument("--reuse-current-auth", action="store_true", help="reuse normal harness auth; copy only public plugin cache, never credentials or config")
    parser.add_argument("--model", help="optional explicit supported harness model; otherwise record default as unspecified")
    args = parser.parse_args()
    fixtures, installation, cli, output = (p.resolve() for p in (args.fixtures, args.installation, args.cli, args.output))
    if output.is_relative_to(fixtures):
        parser.error("private output must be outside fixture repositories")
    output.mkdir(parents=True, mode=0o700, exist_ok=False)
    manifest = json.loads((installation / "marketplace/plugins/preflight/plugin.json").read_text())
    version = manifest["version"]
    env = dict(os.environ)
    if args.reuse_current_auth:
        cache = Path(env.get("CODEX_HOME", str(Path.home() / ".codex"))) / "plugins/cache/preflight-local/preflight" / version
        source = installation / "codex-home/plugins/cache/preflight-local/preflight" / version
        if cache.exists():
            if digests(cache) != digests(source):
                parser.error("existing public plugin cache differs; select a fresh candidate version")
        else:
            shutil.copytree(source, cache)
    else:
        env["CODEX_HOME"] = str(installation / "codex-home")
    env["PATH"] = str(cli.parent) + os.pathsep + env["PATH"]
    metadata = {"codex": subprocess.check_output(["codex", "--version"], text=True).strip(),
                "plugin": version, "cli": subprocess.check_output([str(cli), "version"], text=True).strip(),
                "model": args.model or "harness default (not exposed in exec JSON)",
                "auth": "existing harness session" if args.reuse_current_auth else "isolated Codex home",
                "human_observation": False}
    (output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    results = []
    for scenario in json.loads((fixtures / "scenarios.json").read_text()):
        name, repo = scenario["name"], fixtures / scenario["repo"]
        before = digests(repo)
        command = ["codex", "exec", "--ignore-user-config", "--sandbox", "read-only", "--ephemeral",
                   "-c", 'marketplaces.preflight-local.source_type="local"',
                   "-c", "marketplaces.preflight-local.source=" + json.dumps(str(installation / "marketplace")),
                   "-c", 'plugins={"preflight@preflight-local"={enabled=true}}',
                   "--cd", str(repo), "--json", "--output-last-message", str(output / (name + "-final.txt"))]
        if args.model:
            command.extend(["--model", args.model])
        command.append(scenario["prompt"])
        start = time.monotonic()
        messages = []
        with (output / (name + "-events.jsonl")).open("w") as events, (output / (name + "-stderr.log")).open("w") as errors:
            process = subprocess.Popen(command, env=env, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=errors, text=True)
            timer = threading.Timer(120, process.kill)
            timer.start()
            try:
                for line in process.stdout:
                    elapsed = round(time.monotonic() - start, 3)
                    events.write(line)
                    events.flush()
                    try:
                        event = json.loads(line)
                    except ValueError:
                        continue
                    if event.get("type") == "item.completed" and event.get("item", {}).get("type") == "agent_message":
                        messages.append({"elapsed": elapsed, "text": event["item"]["text"]})
                code = process.wait()
            finally:
                timer.cancel()
        result = {"scenario": name, "exit": code, "duration": round(time.monotonic() - start, 3),
                  "messages": messages, "unchanged": before == digests(repo),
                  "tripwires": [str(p.relative_to(repo)) for p in repo.rglob("*_EXECUTED")],
                  "command": command,
                  "time_to_wow": "requires source/action review of timestamped messages; first token is not success"}
        results.append(result)
        (output / "results.json").write_text(json.dumps(results, indent=2) + "\n")
        print(json.dumps({"scenario": name, "exit": code, "duration": result["duration"], "unchanged": result["unchanged"]}), flush=True)


if __name__ == "__main__":
    main()
