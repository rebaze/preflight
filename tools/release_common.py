"""Shared release identity and remote-state checks; no publication side effects."""

import hashlib
import json
import re
import subprocess


def run(command, capture=False):
    try:
        result = subprocess.run([str(arg) for arg in command], check=True,
                                stdout=subprocess.PIPE if capture else None, text=True)
        return result.stdout
    except subprocess.CalledProcessError as error:
        raise ValueError("command failed: " + " ".join(str(arg) for arg in command[:3])) from error


def digest(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size == 0:
        raise ValueError(str(path) + " must be a nonempty regular file")
    result = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            result.update(block)
    return result.hexdigest()


def validate_identity(repo, tag, commit):
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
        raise ValueError("invalid GitHub repository")
    if not re.fullmatch(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?", tag):
        raise ValueError("invalid release tag")
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("expected a full source commit SHA")


def archive_checksums(tag, path):
    expected = {f"preflight_{tag[1:]}_{os}_{arch}.tar.gz"
                for os in ("darwin", "linux") for arch in ("amd64", "arm64")}
    hashes = {}
    for line in path.read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64}) [ *]([A-Za-z0-9._-]+)", line)
        if not match or match[2] in hashes:
            raise ValueError("invalid or duplicate checksum entry")
        hashes[match[2]] = match[1]
    if set(hashes) != expected:
        raise ValueError("checksums must contain exactly the four release archives")
    return hashes


def verify_tag(repo, tag, commit):
    """Resolve lightweight or nested annotated tags, never trusting a ref name alone."""
    validate_identity(repo, tag, commit)
    value = json.loads(run(["gh", "api", f"repos/{repo}/git/ref/tags/{tag}"], capture=True))
    seen = set()
    for _ in range(16):
        if not isinstance(value, dict) or not isinstance(value.get("object"), dict):
            raise ValueError("invalid remote tag response")
        obj = value.get("object", {})
        sha = obj.get("sha", "")
        if not isinstance(sha, str) or not re.fullmatch(r"[0-9a-f]{40}", sha) or sha in seen:
            raise ValueError("invalid or cyclic remote tag object")
        if obj.get("type") == "commit":
            if sha != commit:
                raise ValueError("remote tag no longer points to the expected source commit")
            return
        if obj.get("type") != "tag":
            raise ValueError("remote tag does not resolve to a commit")
        seen.add(sha)
        value = json.loads(run(["gh", "api", f"repos/{repo}/git/tags/{sha}"], capture=True))
    raise ValueError("remote annotated tag chain is too deep")


def verify_release(release, release_id, tag, expected, *, published):
    if (not isinstance(release, dict) or release.get("id") != release_id or type(release_id) is not int
            or release.get("tag_name") != tag or release.get("draft") is not (not published)):
        raise ValueError("release identity or state changed")
    if published and release.get("immutable") is not True:
        raise ValueError("published release is not immutable; enable immutable releases")
    assets = release.get("assets")
    if not isinstance(assets, list):
        raise ValueError("release asset inventory is missing")
    found = {}
    ids = set()
    for asset in assets:
        if (not isinstance(asset, dict) or asset.get("name") not in expected
                or asset.get("name") in found or type(asset.get("id")) is not int
                or asset["id"] in ids or asset.get("state") != "uploaded"):
            raise ValueError("invalid or duplicate remote asset")
        ids.add(asset["id"])
        found[asset["name"]] = asset.get("digest")
    if found != {name: "sha256:" + sha for name, sha in expected.items()}:
        raise ValueError("remote asset digests differ from verified bytes")
