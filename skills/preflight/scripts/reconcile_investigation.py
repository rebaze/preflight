#!/usr/bin/env python3
"""Check investigation provenance without promoting model opinion to verification."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import sys


class Invalid(ValueError):
    pass


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise Invalid("duplicate JSON field")
        result[key] = value
    return result


def invalid_constant(_value):
    raise Invalid("invalid JSON number")


def read_json(path, limit):
    if path == "-":
        raw = sys.stdin.buffer.read(limit + 1)
    else:
        with Path(path).open("rb") as stream:
            raw = stream.read(limit + 1)
    if len(raw) > limit:
        raise Invalid("input exceeds size limit")
    return json.loads(raw, object_pairs_hook=unique_object, parse_constant=invalid_constant)


def keys(value, expected):
    if not isinstance(value, dict) or set(value) != set(expected):
        raise Invalid("missing or unknown packet field")


def text(value, limit):
    if not isinstance(value, str) or not value.strip() or len(value) > limit:
        raise Invalid("invalid packet text")


def reconcile(observation, packet):
    keys(packet, ("schema", "question", "identity", "status", "conclusion", "sources", "uncertainty", "nextAction"))
    if packet["schema"] != "preflight.investigation/v1":
        raise Invalid("unsupported investigation schema")
    if packet["status"] not in ("hypothesis", "unresolved"):
        raise Invalid("investigations cannot supply verification statuses")
    for field, limit in (("question", 1000), ("conclusion", 2000), ("uncertainty", 2000), ("nextAction", 1000)):
        text(packet[field], limit)
    identity_fields = ("repositoryId", "worktreeId", "head", "inputDigest")
    keys(packet["identity"], identity_fields)
    if any(not isinstance(packet["identity"][field], str) for field in identity_fields):
        raise Invalid("invalid identity")
    sources = packet["sources"]
    if not isinstance(sources, list) or len(sources) > 6:
        raise Invalid("source budget exceeded")
    for source in sources:
        keys(source, ("path", "digest", "line"))
        text(source["path"], 4096)
        if not isinstance(source["digest"], str) or not re.fullmatch(r"[0-9a-f]{64}", source["digest"]):
            raise Invalid("invalid source digest")
        if type(source["line"]) is not int or source["line"] < 1:
            raise Invalid("invalid source line")
    if not isinstance(observation, dict) or observation.get("schema") != "preflight.discovery/v1":
        raise Invalid("unsupported observation schema")
    subject = observation.get("subject")
    observed_sources = observation.get("sources")
    if not isinstance(subject, dict) or not isinstance(observed_sources, list):
        raise Invalid("observation missing identity or sources")
    reasons = []
    if subject.get("current") is not True or observation.get("exitCode") != 0:
        reasons.append("fresh observation has incomplete or stale coverage")
    if any(not subject.get(field) or subject.get(field) != packet["identity"][field] for field in identity_fields):
        reasons.append("observation identity changed or is incomplete")
    by_path = {}
    for source in observed_sources:
        if not isinstance(source, dict) or not isinstance(source.get("path"), str) or source["path"] in by_path:
            raise Invalid("invalid or duplicate observation source")
        by_path[source["path"]] = source
    cited = set()
    total = 0
    for source in sources:
        found = by_path.get(source["path"])
        if found is None:
            reasons.append("cited source is absent from fresh observation")
            continue
        content = found.get("content")
        if not isinstance(content, str):
            raise Invalid("observation source has no text")
        digest = hashlib.sha256(content.encode()).hexdigest()
        if source["digest"] != found.get("digest") or digest != source["digest"]:
            reasons.append("cited source digest does not match observed content")
        start, end = found.get("startLine"), found.get("endLine")
        if type(start) is not int or type(end) is not int or not start <= source["line"] <= end:
            reasons.append("cited source line is outside observed bounds")
        if source["path"] not in cited:
            cited.add(source["path"])
            total += len(content.encode())
    if total > 64 * 1024:
        reasons.append("cited source text exceeds investigation budget")
    if not sources:
        reasons.append("no supporting source citation")
    if packet["status"] == "unresolved":
        reasons.append("investigator left the question unresolved")
    status = "unresolved" if reasons else "hypothesis"
    result = {"schema": "preflight.investigation-review/v1", "status": status,
              "verification": "unverified", "question": packet["question"],
              "investigatorConclusion": packet["conclusion"], "sources": sources,
              "uncertainty": packet["uncertainty"], "nextAction": packet["nextAction"],
              "reasons": sorted(set(reasons)),
              "boundary": "Citation integrity is not semantic proof. Original observations and executed-check statuses are unchanged."}
    return result, 2 if reasons else 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--observation", required=True)
    parser.add_argument("--packet", required=True, help="private packet path, or - for stdin")
    args = parser.parse_args()
    if args.observation == "-" and args.packet == "-":
        parser.error("only one input may use stdin")
    try:
        result, code = reconcile(read_json(args.observation, 16 * 1024 * 1024), read_json(args.packet, 64 * 1024))
    except (Invalid, ValueError, TypeError, OSError) as error:
        # Do not echo raw file contents, credential-bearing paths or parser excerpts.
        result, code = {"schema": "preflight.investigation-review/v1", "status": "invalid",
                        "verification": "unverified", "reason": str(error) if isinstance(error, Invalid) else "cannot decode investigation inputs"}, 3
    print(json.dumps(result, indent=2))
    return code


if __name__ == "__main__":
    sys.exit(main())
