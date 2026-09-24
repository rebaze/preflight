#!/usr/bin/env python3
"""Install this plugin in an isolated, new Codex home; never copies credentials."""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path, help="new private directory for marketplace and Codex home")
    args = parser.parse_args()
    source = Path(__file__).resolve().parents[1]
    destination = args.destination.absolute()
    destination.mkdir(mode=0o700, parents=True, exist_ok=False)
    marketplace = destination / "marketplace"
    plugin = marketplace / "plugins/preflight"
    plugin.mkdir(parents=True)
    for relative in ("plugin.json", ".codex-plugin", "skills", "LICENSE"):
        item = source / relative
        if item.is_dir():
            shutil.copytree(item, plugin / relative)
        else:
            shutil.copy2(item, plugin / relative)
    catalog = marketplace / ".agents/plugins/marketplace.json"
    catalog.parent.mkdir(parents=True)
    catalog.write_text(json.dumps({
        "name": "preflight-local",
        "interface": {"displayName": "Preflight local candidate"},
        "plugins": [{"name": "preflight", "source": {"source": "local", "path": "./plugins/preflight"},
            "policy": {"installation": "AVAILABLE", "authentication": "ON_USE"}, "category": "Developer Tools"}],
    }, indent=2) + "\n")
    codex_home = destination / "codex-home"
    codex_home.mkdir(mode=0o700)
    env = dict(os.environ, CODEX_HOME=str(codex_home))
    subprocess.run(["codex", "plugin", "marketplace", "add", str(marketplace), "--json"], env=env, check=True)
    subprocess.run(["codex", "plugin", "add", "preflight@preflight-local", "--json"], env=env, check=True)
    print(json.dumps({"codex_home": str(codex_home), "marketplace": str(marketplace),
        "authentication": "not copied; use the harness's supported login flow for this home"}))


if __name__ == "__main__":
    main()
