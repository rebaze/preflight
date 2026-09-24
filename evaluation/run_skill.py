#!/usr/bin/env python3
"""Run synthetic skill scenarios in new read-only Codex sessions with private logs."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
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
    parser.add_argument("--marketplace", type=Path, help="installed archive's share/preflight directory; defaults to installation/marketplace")
    parser.add_argument("--cli", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new private directory outside fixtures")
    parser.add_argument("--reuse-current-auth", action="store_true", help="reuse normal harness auth; copy only public plugin cache, never credentials or config")
    parser.add_argument("--model", help="optional explicit supported harness model; otherwise record default as unspecified")
    parser.add_argument("--reasoning-effort", choices=("low", "medium", "high", "xhigh"))
    parser.add_argument("--scenario", action="append", help="run only named scenarios (repeatable)")
    parser.add_argument("--timed-only", action="store_true", help="run the declared five-scenario measurement set")
    parser.add_argument("--disable-delegation", action="store_true", help="request CLI multi_agent disable; verify actual tool availability separately")
    parser.add_argument("--persist-session", action="store_true", help="retain a fresh private harness session when native delegation needs thread storage")
    args = parser.parse_args()
    fixtures, installation, cli, output = (p.resolve() for p in (args.fixtures, args.installation, args.cli, args.output))
    if output.is_relative_to(fixtures):
        parser.error("private output must be outside fixture repositories")
    output.mkdir(parents=True, mode=0o700, exist_ok=False)
    marketplace = args.marketplace.resolve() if args.marketplace else installation / "marketplace"
    catalog = json.loads((marketplace / ".agents/plugins/marketplace.json").read_text())
    marketplace_name = catalog["name"]
    if not isinstance(marketplace_name, str) or not re.fullmatch(r"[A-Za-z0-9_-]+", marketplace_name):
        parser.error("unsupported marketplace name")
    plugin_id = "preflight@" + marketplace_name
    manifest = json.loads((marketplace / "plugins/preflight/plugin.json").read_text())
    version = manifest["version"]
    env = dict(os.environ)
    if args.reuse_current_auth:
        cache = Path(env.get("CODEX_HOME", str(Path.home() / ".codex"))) / "plugins/cache" / marketplace_name / "preflight" / version
        source = installation / "codex-home/plugins/cache" / marketplace_name / "preflight" / version
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
                "cliSHA256": hashlib.sha256(cli.read_bytes()).hexdigest(),
                "cliCapabilities": json.loads(subprocess.check_output([str(cli), "capabilities", "--format", "json"], text=True)),
                "model": args.model or "harness default (not exposed in exec JSON)",
                "reasoning_effort": args.reasoning_effort or "harness default",
                "auth": "existing harness session" if args.reuse_current_auth else "isolated Codex home",
                "human_observation": False}
    installation_metrics = installation / "installation-metrics.json"
    if installation_metrics.exists():
        metadata["installation"] = json.loads(installation_metrics.read_text())
    (output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    results = []
    scenarios = json.loads((fixtures / "scenarios.json").read_text())
    if args.scenario and set(args.scenario) - {scenario["name"] for scenario in scenarios}:
        parser.error("unknown evaluation scenario")
    for scenario in scenarios:
        if args.scenario and scenario["name"] not in args.scenario:
            continue
        if args.timed_only and not scenario.get("timed"):
            continue
        name, repo = scenario["name"], fixtures / scenario["repo"]
        before = digests(repo)
        command = ["codex", "exec", "--ignore-user-config", "--sandbox", "read-only",
                   "-c", 'marketplaces.' + marketplace_name + '.source_type="local"',
                   "-c", "marketplaces." + marketplace_name + ".source=" + json.dumps(str(marketplace)),
                   "-c", 'plugins={' + json.dumps(plugin_id) + '={enabled=true}}',
                   "--cd", str(repo), "--json", "--output-last-message", str(output / (name + "-final.txt"))]
        if not args.persist_session:
            command.append("--ephemeral")
        if args.model:
            command.extend(["--model", args.model])
        if args.reasoning_effort:
            command.extend(["-c", "model_reasoning_effort=" + json.dumps(args.reasoning_effort)])
        if args.disable_delegation:
            command.extend(["--disable", "multi_agent"])
        command.append(scenario["prompt"])
        start = time.monotonic()
        messages = []
        event_timings = []
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
                    event_timings.append({"elapsed": elapsed, "type": event.get("type"), "item": {key: event.get("item", {}).get(key) for key in ("id", "type", "tool", "status") if key in event.get("item", {})}})
                    if event.get("type") == "item.completed" and event.get("item", {}).get("type") == "agent_message":
                        messages.append({"elapsed": elapsed, "text": event["item"]["text"]})
                code = process.wait()
            finally:
                timer.cancel()
        result = {"scenario": name, "exit": code, "duration": round(time.monotonic() - start, 3),
                  "messages": messages, "event_timings": event_timings, "unchanged": before == digests(repo),
                  "tripwires": [str(p.relative_to(repo)) for p in repo.rglob("*_EXECUTED")],
                  "command": command,
                  "time_to_wow": "requires source/action review of timestamped messages; first token is not success"}
        if "installation" in metadata:
            result["secondsSinceInstallationStartAtInvocation"] = round(start - metadata["installation"]["startedMonotonic"], 3)
        results.append(result)
        (output / "results.json").write_text(json.dumps(results, indent=2) + "\n")
        print(json.dumps({"scenario": name, "exit": code, "duration": result["duration"], "unchanged": result["unchanged"]}), flush=True)
    if not results:
        raise SystemExit("no scenarios selected")
    if any(result["exit"] != 0 or not result["unchanged"] or result["tripwires"] for result in results):
        raise SystemExit("one or more harness runs failed; inspect preserved private results")


if __name__ == "__main__":
    main()
