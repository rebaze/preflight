#!/usr/bin/env python3
"""Validate exactly four CLI archives, their embedded plugin, and native discovery."""

import hashlib
import json
from pathlib import Path, PurePosixPath
import platform
import re
import subprocess
import sys
import tarfile
import tempfile

PLUGIN_ROOT = "share/preflight/plugins/preflight/"
CATALOG = "share/preflight/.agents/plugins/marketplace.json"
REQUIRED = {"preflight", "README.md", "LICENSE", "NOTICE", "share/preflight/policy/main.rego",
            "share/preflight/profiles/frontend-vitest.json", "share/preflight/runtime/run-tests.sh",
            "share/preflight/schemas/report-v1.json", "share/preflight/schemas/discovery-v1.json",
            "share/preflight/schemas/capabilities-v1.json",
            "docs/development.md", "docs/try-preflight.html", "docs/plans/issue-4.md", "evaluation/README.md", CATALOG}
PLATFORMS = {(os, arch) for os in ("darwin", "linux") for arch in ("amd64", "arm64")}


def strict_json(data):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate JSON key: " + key)
            result[key] = value
        return result
    return json.loads(data, object_pairs_hook=unique)


def plugin_files(source):
    """Compare every public skill file, including nested references/scripts, to source."""
    items = [source / "plugin.json", source / ".codex-plugin/plugin.json", source / "LICENSE"]
    items += sorted((source / "skills/preflight").rglob("*"))
    expected = {}
    for path in items:
        relative = path.relative_to(source)
        if any(source.joinpath(*relative.parts[:i]).is_symlink() for i in range(1, len(relative.parts) + 1)):
            raise ValueError("symlink in public plugin source: " + str(path))
        if path.is_dir():
            continue
        if not path.is_file():
            raise ValueError("missing public plugin source: " + str(path))
        if "__pycache__" in path.parts or path.suffix == ".pyc":
            raise ValueError("generated file in public plugin source: " + str(path))
        expected[PLUGIN_ROOT + path.relative_to(source).as_posix()] = path.read_bytes()
    if PLUGIN_ROOT + "skills/preflight/SKILL.md" not in expected:
        raise ValueError("missing public preflight skill")
    return expected


def validate_plugin_metadata(files, catalog_bytes):
    root = strict_json(files[PLUGIN_ROOT + "plugin.json"])
    overlay = strict_json(files[PLUGIN_ROOT + ".codex-plugin/plugin.json"])
    if root.get("name") != "preflight" or not root.get("version"):
        raise ValueError("invalid plugin identity/version")
    if root.get("$schema") != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json":
        raise ValueError("unsupported portable plugin manifest")
    for key in ("name", "version", "description", "license"):
        if root.get(key) != overlay.get(key):
            raise ValueError("portable and Codex plugin metadata mismatch: " + key)
    if overlay.get("skills") != "./skills/":
        raise ValueError("unexpected Codex skill path")
    catalog = strict_json(catalog_bytes)
    entries = catalog.get("plugins", [])
    if catalog.get("name") != "preflight" or len(entries) != 1:
        raise ValueError("expected the single-plugin preflight marketplace")
    if entries[0].get("category") != "Developer Tools" or entries[0].get("name") != "preflight" or entries[0].get("source") != {"source": "local", "path": "./plugins/preflight"}:
        raise ValueError("marketplace does not resolve the embedded plugin")


def read_archive(archive):
    files = {}
    seen = set()
    total = 0
    for member in archive:
        path = PurePosixPath(member.name)
        if (not member.name or member.name.startswith("/") or "\\" in member.name
                or any(part in ("", ".", "..") for part in member.name.rstrip("/").split("/"))
                or path.is_absolute() or ":" in path.parts[0]):
            raise ValueError("unsafe archive path: " + member.name)
        canonical = str(path)
        if canonical in seen:
            raise ValueError("duplicate archive path: " + member.name)
        seen.add(canonical)
        if member.isdir():
            continue
        if not member.isfile() or member.mode & 0o6000:
            raise ValueError("nonregular or privileged archive member: " + member.name)
        total += member.size
        if member.size > 64 << 20 or total > 256 << 20:
            raise ValueError("archive exceeds bounded inspection size")
        files[canonical] = archive.extractfile(member).read()
    return files


def smoke_native(binary_bytes, version):
    # Only the inspected executable is written to a fixed name in private temp
    # storage. No member path is ever used for extraction and no project runs.
    with tempfile.TemporaryDirectory(prefix="preflight-package-") as temp:
        binary = Path(temp) / "preflight"
        binary.write_bytes(binary_bytes)
        binary.chmod(0o755)
        result = subprocess.check_output([str(binary), "version"], text=True, timeout=10)
        if not result.startswith(f"preflight {version} (commit "):
            raise ValueError("packaged version does not match archive name: " + result)
        subprocess.run([str(binary), "--help"], check=True, stdout=subprocess.DEVNULL, timeout=10)
        capabilities = strict_json(subprocess.check_output([str(binary), "capabilities", "--format", "json"], timeout=10))
        features = capabilities.get("features")
        if (capabilities.get("schema") != "preflight.capabilities/v1"
                or type(capabilities.get("skillProtocol")) is not int
                or capabilities.get("skillProtocol") != 1
                or capabilities.get("discoverySchema") != "preflight.discovery/v1"
                or capabilities.get("version") != version
                or not isinstance(features, list)
                or any(not isinstance(feature, str) for feature in features)
                or len(features) != len(set(features))
                or set(features) != {"inspect", "github", "compare"}):
            raise ValueError("packaged CLI does not implement the plugin protocol")
        empty = Path(temp) / "empty"
        empty.mkdir()
        result = subprocess.run([str(binary), "inspect", "--repo", str(empty), "--format", "json"],
                                capture_output=True, timeout=50)
        observation = strict_json(result.stdout)
        if (result.returncode != 2 or observation.get("exitCode") != 2
                or observation.get("schema") != "preflight.discovery/v1"
                or observation.get("sources") != []):
            raise ValueError("packaged discovery did not preserve an honest empty-directory result")
        if list(empty.iterdir()):
            raise ValueError("packaged discovery modified the inspected directory")


def check(directory, source=None):
    source = source or Path(__file__).resolve().parents[1]
    expected_plugin = plugin_files(source)
    expected_catalog = (source / "package/marketplace.json").read_bytes()
    validate_plugin_metadata(expected_plugin, expected_catalog)
    required = REQUIRED | expected_plugin.keys()
    found, versions = set(), set()
    host = (platform.system().lower(), {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine(), platform.machine()))
    checked_native = False
    checksums = {}
    for line in (directory / "checksums.txt").read_text().splitlines():
        fields = line.split()
        if len(fields) != 2 or not re.fullmatch("[0-9a-f]{64}", fields[0]):
            raise ValueError("invalid archive checksum line")
        sha, name = fields
        if name in checksums:
            raise ValueError("duplicate checksum: " + name)
        checksums[name] = sha
    for path in sorted(directory.glob("*.tar.gz")):
        match = re.fullmatch(r"preflight_(.+)_(darwin|linux)_(amd64|arm64)\.tar\.gz", path.name)
        if not match:
            raise ValueError("unexpected archive: " + path.name)
        version, os, arch = match.groups()
        if (os, arch) in found:
            raise ValueError("duplicate platform archive")
        found.add((os, arch))
        versions.add(version)
        if hashlib.sha256(path.read_bytes()).hexdigest() != checksums.get(path.name):
            raise ValueError("archive checksum mismatch: " + path.name)
        with tarfile.open(path) as archive:
            files = read_archive(archive)
        if not required.issubset(files):
            raise ValueError("missing archive paths: " + repr(required - files.keys()))
        actual_plugin = {name: body for name, body in files.items() if name.startswith(PLUGIN_ROOT)}
        if actual_plugin.keys() != expected_plugin.keys():
            raise ValueError("unexpected or missing public plugin files")
        if actual_plugin != expected_plugin or files[CATALOG] != expected_catalog:
            raise ValueError("packaged plugin/catalog differs from the versioned source")
        validate_plugin_metadata(actual_plugin, files[CATALOG])
        if (os, arch) == host:
            smoke_native(files["preflight"], version)
            checked_native = True
        print("OK " + path.name)
    if found != PLATFORMS or len(versions) != 1 or len(checksums) != 4 or not checked_native:
        raise ValueError("requires exactly four consistent archives and a native executable smoke test")


if __name__ == "__main__":
    check(Path(sys.argv[1]))
