#!/usr/bin/env python3
"""Inspect all release archives and smoke-test the host's packaged executable."""

import hashlib
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile
import tempfile


def check(directory):
    required = {"preflight", "README.md", "LICENSE", "NOTICE", "share/preflight/policy/main.rego",
                "share/preflight/profiles/frontend-vitest.json",
                "share/preflight/runtime/run-tests.sh",
                "share/preflight/schemas/report-v1.json", "docs/development.md"}
    expected = {(os, arch) for os in ("darwin", "linux") for arch in ("amd64", "arm64")}
    found = set()
    versions = set()
    host = (platform.system().lower(), {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine(), platform.machine()))
    checked_native = False
    checksums = {}
    for line in (directory / "checksums.txt").read_text().splitlines():
        sha, name = line.split()
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
            names = archive.getnames()
            if len(names) != len(set(names)) or not required.issubset(names):
                raise ValueError("duplicate or missing archive paths: " + path.name + " " + repr(required - set(names)))
            if (os, arch) == host:
                with tempfile.TemporaryDirectory(prefix="preflight-package-") as temp:
                    binary = Path(temp) / "preflight"
                    binary.write_bytes(archive.extractfile("preflight").read())
                    binary.chmod(0o755)
                    result = subprocess.check_output([str(binary), "version"], text=True)
                    if not result.startswith(f"preflight {version} (commit "):
                        raise ValueError("packaged version does not match archive name: " + result)
                    subprocess.run([str(binary), "--help"], check=True, stdout=subprocess.DEVNULL)
                    checked_native = True
        print("OK " + path.name)
    if found != expected or len(versions) != 1 or len(checksums) != 4 or not checked_native:
        raise ValueError("requires four consistent archives and a native executable smoke test")


if __name__ == "__main__":
    check(Path(sys.argv[1]))
