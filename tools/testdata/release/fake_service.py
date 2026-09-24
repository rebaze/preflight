"""OFFLINE TEST DOUBLE. No signatures are authenticated; no network is used."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import sys

tool, *args = sys.argv[1:]
root = Path(os.environ["FAKE_RELEASE_ROOT"])
with (root / "events.jsonl").open("a") as log:
    log.write(json.dumps([tool] + args) + "\n")


def fail(message):
    print("fixture service: " + message, file=sys.stderr)
    sys.exit(1)


def value(flag):
    return args[args.index(flag) + 1]


def metadata():
    remote = root / "remote"
    assets = [{"id": i + 1, "name": path.name, "state": "uploaded",
               "digest": "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()}
              for i, path in enumerate(sorted(remote.iterdir()))]
    published = (root / "published").exists()
    return {"id": 123, "draft": not published, "immutable": published and not os.environ.get("FAKE_MUTABLE"),
            "tag_name": (root / "tag").read_text(), "assets": assets,
            "prerelease": "-" in (root / "tag").read_text()}


if tool == "goreleaser":
    if not any(arg.startswith("--skip=") and "publish" in arg.split("=", 1)[1].split(",")
               for arg in args):
        (root / "published").write_text("published before verification")
elif tool == "cosign":
    if args[0] != "verify-blob" or "--certificate-identity" not in args:
        fail("unexpected cosign invocation")
    if os.environ.get("FAKE_FAIL") == "cosign":
        fail("signature verification failed (synthetic)")
elif tool == "gh" and args[:2] == ["attestation", "verify"]:
    if "--cert-identity" not in args or "--source-ref" not in args:
        fail("missing identity constraints")
    artifact = Path(args[2])
    bundle = json.loads(Path(value("--bundle")).read_text())
    if bundle.get("fixtureOnly") is not True:
        fail("requires synthetic fixture bundle")
    if bundle["subjects"].get(artifact.name) != hashlib.sha256(artifact.read_bytes()).hexdigest():
        fail("attestation digest mismatch")
    if "--source-digest" in args and value("--source-digest") != bundle.get("commit"):
        fail("attestation source commit mismatch")
    predicate = value("--predicate-type")
    failure = os.environ.get("FAKE_FAIL")
    if (failure == "provenance" and "slsa.dev" in predicate) or (
            failure == "sbom" and "cyclonedx.org" in predicate):
        fail("required attestation verification failed (synthetic)")
    if "--format" in args:
        sbom = {"wrong": "document"} if os.environ.get("FAKE_WRONG_SBOM") else bundle["predicate"]
        print(json.dumps([{"verificationResult": {"statement": {"predicate": sbom}}}]))
elif tool == "gh" and args[:2] == ["release", "create"]:
    if "--draft" not in args or "--verify-tag" not in args:
        fail("create must be a draft for an existing tag")
    remote = root / "remote"
    if remote.exists():
        fail("release already exists")
    remote.mkdir()
    (root / "tag").write_text(args[2])
    # The caller provides each asset as an absolute file argument.
    notes = value("--notes-file")
    for arg in args[3:]:
        if arg != notes and Path(arg).is_absolute() and Path(arg).is_file():
            shutil.copyfile(arg, remote / Path(arg).name)
    if os.environ.get("FAKE_REMOTE_CHANGE"):
        next(remote.glob("*.tar.gz")).write_bytes(b"replaced during upload\n")
elif tool == "gh" and args[:2] == ["release", "download"]:
    for path in (root / "remote").iterdir():
        shutil.copyfile(path, Path(value("--dir")) / path.name)
    if os.environ.get("FAKE_AFTER_DOWNLOAD") and not (root / "published").exists():
        next((root / "remote").glob("*.tar.gz")).write_bytes(b"changed after verification download\n")
elif tool == "gh" and args[0] == "api" and "/git/ref/tags/" in args[1]:
    count_file = root / "tag_reads"
    count = int(count_file.read_text()) + 1 if count_file.exists() else 1
    count_file.write_text(str(count))
    sha = os.environ["FAKE_COMMIT"]
    if count >= int(os.environ.get("FAKE_TAG_MOVE_AT", "999")):
        sha = "b" * 40
    if os.environ.get("FAKE_ANNOTATED"):
        print(json.dumps({"object": {"type": "tag", "sha": "c" * 40}}))
    else:
        print(json.dumps({"object": {"type": "commit", "sha": sha}}))
elif tool == "gh" and args[0] == "api" and "/git/tags/" in args[1]:
    if args[1].endswith("c" * 40):
        obj = {"type": "tag", "sha": "d" * 40}
    else:
        obj = {"type": "commit", "sha": os.environ["FAKE_COMMIT"]}
    print(json.dumps({"object": obj}))
elif tool == "gh" and args[0] == "api" and "/releases/tags/" in args[1]:
    print(json.dumps(metadata()))
elif tool == "gh" and args[0] == "api" and "--paginate" in args:
    if os.environ.get("FAKE_FAIL") == "lookup":
        fail("release lookup failed")
    releases = []
    if (root / "remote").exists():
        releases = [{"id": 123, "tag_name": "v1.2.3", "draft": True}]
        if (root / "tag").exists():
            releases[0]["tag_name"] = (root / "tag").read_text()
    print(json.dumps([releases]))
elif tool == "gh" and args[0] == "api" and "--method" in args:
    if value("--method") != "PATCH" or "draft=false" not in args or not args[1].endswith("/123"):
        fail("only ID-bound final publication is supported")
    if os.environ.get("FAKE_DURING_PUBLISH"):
        next((root / "remote").glob("*.tar.gz")).write_bytes(b"changed during publication\n")
    (root / "published").write_text("publication was attempted\n")
    print(json.dumps(metadata()))
elif tool == "gh" and args[0] == "api" and args[1].endswith("/123"):
    print(json.dumps(metadata()))
else:
    fail("unexpected command: " + repr([tool] + args))
