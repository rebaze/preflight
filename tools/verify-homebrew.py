#!/usr/bin/env python3
"""Verify immutable published assets before allowing a Homebrew update."""

import argparse
import json
from pathlib import Path
import re
import sys

from release_common import archive_checksums, digest, run, validate_identity, verify_release, verify_tag


def verify(args):
    validate_identity(args.repo, args.tag, args.commit)
    if not re.fullmatch(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", args.tag):
        raise ValueError("Homebrew requires a stable version tag")
    verify_tag(args.repo, args.tag, args.commit)
    release = json.loads(run(["gh", "api", f"repos/{args.repo}/releases/tags/{args.tag}"], capture=True))
    if (release.get("immutable") is not True or release.get("draft") is not False
            or release.get("prerelease") is not False):
        raise ValueError("Homebrew requires an immutable published stable release")
    args.directory.mkdir()  # A previous/partial verification must not be reused.
    run(["gh", "release", "download", args.tag, "--repo", args.repo, "--dir", args.directory])
    hashes = archive_checksums(args.tag, args.directory / "checksums.txt")
    required = set(hashes) | {"checksums.txt", "checksums.txt.sigstore.json",
                             f"preflight-{args.tag}.intoto.jsonl", f"preflight-{args.tag}-source.cdx.json"}
    if {p.name for p in args.directory.iterdir()} != required:
        raise ValueError("published release has missing or unexpected assets")
    downloaded = {name: digest(args.directory / name) for name in required}
    if any(downloaded[name] != sha for name, sha in hashes.items()):
        raise ValueError("published archive checksum mismatch")
    verify_release(release, release.get("id"), args.tag, downloaded, published=True)
    identity = f"https://github.com/{args.repo}/.github/workflows/release.yaml@refs/tags/{args.tag}"
    for name in sorted(set(hashes) | {"checksums.txt"}):
        run(["gh", "attestation", "verify", args.directory / name,
             "--bundle", args.directory / f"preflight-{args.tag}.intoto.jsonl",
             "--repo", args.repo, "--source-ref", "refs/tags/" + args.tag,
             "--source-digest", args.commit, "--cert-identity", identity,
             "--predicate-type", "https://slsa.dev/provenance/v1"])
    verify_tag(args.repo, args.tag, args.commit)
    current = json.loads(run(["gh", "api", f"repos/{args.repo}/releases/{release['id']}"], capture=True))
    verify_release(current, release["id"], args.tag, downloaded, published=True)
    print("VERIFIED: immutable release and all four archives at source commit " + args.commit)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "tag", "commit"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--directory", type=Path, required=True)
    try:
        verify(parser.parse_args())
    except (OSError, ValueError, TypeError) as error:
        print("BLOCKED: " + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
