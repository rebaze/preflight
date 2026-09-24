"""Exercise retry verification using actual files and the offline GitHub fixture."""

import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class HomebrewVerificationTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        spec = importlib.util.spec_from_file_location("fixture", ROOT / "tools/testdata/release/fixture.py")
        fixture = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(fixture)
        self.__dict__.update(vars(fixture.create(self.root)))
        self.remote = self.root / "remote"
        self.remote.mkdir()
        for path in self.dist.iterdir():
            if path.name != "CHANGELOG.md":
                shutil.copyfile(path, self.remote / path.name)
        shutil.copyfile(self.bundle, self.remote / f"preflight-{self.tag}.intoto.jsonl")
        (self.root / "published").touch()
        (self.root / "tag").write_text(self.tag)

    def command(self, **env):
        return subprocess.run([sys.executable, str(ROOT / "tools/verify-homebrew.py"),
                               "--repo", self.repo, "--tag", self.tag, "--commit", self.commit,
                               "--directory", str(self.root / "downloaded")],
                              env=dict(self.env, **env), text=True, capture_output=True)

    def test_all_four_archives_are_downloaded_and_verified(self):
        result = self.command()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(list((self.root / "downloaded").glob("*.tar.gz"))), 4)
        events = [json.loads(line) for line in (self.root / "events.jsonl").read_text().splitlines()]
        verified = {Path(e[3]).name for e in events if e[:3] == ["gh", "attestation", "verify"]}
        self.assertEqual(verified, {p.name for p in self.remote.glob("*.tar.gz")} | {"checksums.txt"})

    def test_missing_archive_blocks_retry(self):
        (self.remote / self.archive).unlink()
        self.assertNotEqual(self.command().returncode, 0)

    def test_replaced_archive_blocks_retry(self):
        (self.remote / self.archive).write_bytes(b"wrong bytes")
        result = self.command()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum", result.stderr)

    def test_mutable_release_blocks_retry(self):
        result = self.command(FAKE_MUTABLE="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("immutable", result.stderr)

    def test_moved_tag_blocks_retry(self):
        result = self.command(FAKE_TAG_MOVE_AT="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("commit", result.stderr)

    def test_wrong_source_commit_in_attestation_blocks_retry(self):
        path = self.remote / f"preflight-{self.tag}.intoto.jsonl"
        bundle = json.loads(path.read_text())
        bundle["commit"] = "b" * 40
        path.write_text(json.dumps(bundle))
        self.assertNotEqual(self.command().returncode, 0)

    def test_prerelease_blocks_retry(self):
        self.tag = "v1.2.3-rc.1"
        (self.root / "tag").write_text(self.tag)
        self.assertNotEqual(self.command().returncode, 0)


if __name__ == "__main__":
    unittest.main()
