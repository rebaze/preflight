#!/usr/bin/env python3
"""Measure installation from an extracted CLI archive into a fresh private Codex home."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tarfile
import time


def unique_fields(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate capability field")
        result[key] = value
    return result


def validate_capabilities(raw):
    capabilities = json.loads(raw, object_pairs_hook=unique_fields)
    required = {"schema", "version", "commit", "discoverySchema", "skillProtocol", "features"}
    if not isinstance(capabilities, dict) or set(capabilities) != required:
        raise ValueError("missing or unknown capability field")
    if capabilities["schema"] != "preflight.capabilities/v1" or capabilities["discoverySchema"] != "preflight.discovery/v1":
        raise ValueError("incompatible capability/discovery schema")
    if type(capabilities["skillProtocol"]) is not int or capabilities["skillProtocol"] != 1:
        raise ValueError("incompatible skill protocol")
    if any(not isinstance(capabilities[key], str) or not capabilities[key] for key in ("version", "commit")):
        raise ValueError("missing CLI version identity")
    features = capabilities["features"]
    if not isinstance(features, list) or any(not isinstance(feature, str) for feature in features) or len(features) != len(set(features)) or not {"inspect", "github", "compare"}.issubset(features):
        raise ValueError("missing or invalid candidate features")
    return capabilities


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    inputs = parser.add_mutually_exclusive_group(required=True)
    inputs.add_argument("--archive", type=Path, help="already locally available native CLI archive; measure extraction too")
    inputs.add_argument("--marketplace", type=Path, help="already extracted archive's share/preflight directory")
    parser.add_argument("--cli", type=Path, help="required with --marketplace")
    parser.add_argument("--destination", type=Path, required=True, help="new private directory")
    args = parser.parse_args()
    started = time.monotonic()
    started_at = datetime.now(timezone.utc).isoformat()
    destination = args.destination.resolve()
    destination.mkdir(parents=True, mode=0o700, exist_ok=False)
    archive_identity = None
    if args.archive:
        extracted = destination / "extracted"
        extracted.mkdir()
        archive_identity = hashlib.sha256(args.archive.read_bytes()).hexdigest()
        with tarfile.open(args.archive) as archive:
            members = archive.getmembers()
            if len(members) > 1000 or sum(member.size for member in members) > 256 * 1024 * 1024:
                raise SystemExit("candidate archive exceeds evaluation bounds")
            for member in members:
                name = Path(member.name)
                if name.is_absolute() or ".." in name.parts or not (member.isfile() or member.isdir()):
                    raise SystemExit("candidate contains an unsafe archive member")
            for member in members:
                target = extracted / member.name
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                else:
                    target.parent.mkdir(parents=True, exist_ok=True)
                    with archive.extractfile(member) as stream:
                        target.write_bytes(stream.read())
                    target.chmod(member.mode & 0o777)
        marketplace, cli = extracted / "share/preflight", extracted / "preflight"
    else:
        if args.cli is None:
            parser.error("--marketplace requires --cli")
        marketplace, cli = args.marketplace.resolve(), args.cli.resolve()
    codex_home = destination / "codex-home"
    codex_home.mkdir(mode=0o700)
    catalog = json.loads((marketplace / ".agents/plugins/marketplace.json").read_text())
    manifest = json.loads((marketplace / "plugins/preflight/plugin.json").read_text())
    try:
        capabilities = validate_capabilities(subprocess.check_output([str(cli), "capabilities", "--format", "json"], text=True, timeout=10))
    except (ValueError, subprocess.SubprocessError):
        raise SystemExit("incompatible installed CLI capability contract")
    env = dict(os.environ, CODEX_HOME=str(codex_home))
    env["PATH"] = str(cli.parent) + os.pathsep + env["PATH"]
    subprocess.run(["codex", "plugin", "marketplace", "add", str(marketplace), "--json"], env=env, check=True)
    subprocess.run(["codex", "plugin", "add", "preflight@" + catalog["name"], "--json"], env=env, check=True)
    completed = time.monotonic()
    result = {"startedAt": started_at, "startedMonotonic": started, "completedMonotonic": completed,
              "installationSeconds": round(completed - started, 3), "marketplace": str(marketplace),
              "pluginVersion": manifest["version"], "cliCapabilities": capabilities,
              "cliPath": str(cli), "archiveSHA256": archive_identity,
              "codexVersion": subprocess.check_output(["codex", "--version"], text=True).strip(),
              "credentialsCopied": False, "authentication": "fresh home requires supported login; evaluation may reuse existing harness auth without copying it",
              "measurementBoundary": "local native archive extraction, CLI PATH/prerequisite check and plugin installation included" if args.archive else "already-extracted candidate; CLI PATH/prerequisite check and plugin installation included",
              "excludedCosts": "obtaining/building the local candidate archive and establishing the existing harness login; report these separately"}
    (destination / "installation-metrics.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result))


if __name__ == "__main__":
    main()
