"""Candidate installation must reject incompatible CLIs before installing a plugin."""

import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import tarfile
import unittest


sys.dont_write_bytecode = True
SCRIPT = Path(__file__).with_name("install_candidate.py")
SPEC = importlib.util.spec_from_file_location("install_candidate", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class OnboardingTest(unittest.TestCase):
    def setUp(self):
        self.capabilities = {"schema": "preflight.capabilities/v1", "version": "0.1.0-rc.2", "commit": "example-commit",
                             "discoverySchema": "preflight.discovery/v1", "skillProtocol": 1,
                             "features": ["inspect", "github", "compare"]}

    def test_valid_candidate(self):
        self.assertEqual(MODULE.validate_capabilities(json.dumps(self.capabilities)), self.capabilities)

    def test_unsupported_missing_and_wrong_typed_contracts(self):
        for field, value in (("schema", "future/v2"), ("skillProtocol", 2), ("skillProtocol", True),
                             ("discoverySchema", "preflight.discovery/v2"), ("version", ""),
                             ("features", ["inspect"]), ("features", ["inspect", "github", "compare", "inspect"])):
            with self.subTest(field=field, value=value):
                candidate = dict(self.capabilities, **{field: value})
                with self.assertRaises(ValueError):
                    MODULE.validate_capabilities(json.dumps(candidate))
        for field in self.capabilities:
            candidate = dict(self.capabilities)
            del candidate[field]
            with self.assertRaises(ValueError):
                MODULE.validate_capabilities(json.dumps(candidate))

    def test_feature_vocabulary_and_array_shape_are_strict(self):
        supported = ["inspect", "github", "compare"]
        for features in (supported + ["execute-host"], supported + ["inspect"],
                         supported + [42], supported + [{}], {key: True for key in supported},
                         "inspect github compare", None):
            with self.subTest(features=features), self.assertRaises(ValueError):
                MODULE.validate_capabilities(json.dumps(dict(self.capabilities, features=features)))
        reordered = dict(self.capabilities, features=list(reversed(supported)))
        self.assertEqual(MODULE.validate_capabilities(json.dumps(reordered)), reordered)

    def test_unknown_duplicate_and_text_output(self):
        for raw in ('{"schema":"one","schema":"two"}', 'Preflight capabilities',
                    json.dumps(dict(self.capabilities, downloadLatest=True))):
            with self.assertRaises(ValueError):
                MODULE.validate_capabilities(raw)

    def test_fake_incompatible_cli_prevents_codex_install(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            marketplace = root / "marketplace"
            catalog = marketplace / ".agents/plugins/marketplace.json"
            manifest = marketplace / "plugins/preflight/plugin.json"
            catalog.parent.mkdir(parents=True)
            manifest.parent.mkdir(parents=True)
            catalog.write_text('{"name":"synthetic"}')
            manifest.write_text('{"version":"0.1.0-rc.2"}')
            cli = root / "preflight"
            cli.write_text('#!/bin/sh\n[ "$1 $2 $3" = "capabilities --format json" ] || exit 9\nprintf \'{"schema":"incompatible/v9"}\\n\'\n')
            cli.chmod(0o755)
            codex = root / "codex"
            codex.write_text('#!/bin/sh\ntouch "' + str(root / "CODEX_INSTALL_RAN") + '"\n')
            codex.chmod(0o755)
            result = subprocess.run([sys.executable, str(SCRIPT), "--marketplace", str(marketplace), "--cli", str(cli),
                                     "--destination", str(root / "install")], env=dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"]), capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("incompatible installed CLI", result.stderr)
            self.assertFalse((root / "CODEX_INSTALL_RAN").exists())

    def test_archive_parent_traversal_and_symlinks_are_refused(self):
        for name, kind in (("../escaped", tarfile.REGTYPE), ("preflight", tarfile.SYMTYPE)):
            with self.subTest(name=name, kind=kind), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                archive = root / "unsafe.tar.gz"
                with tarfile.open(archive, "w:gz") as stream:
                    member = tarfile.TarInfo(name)
                    member.type = kind
                    if kind == tarfile.SYMTYPE:
                        member.linkname = "/outside"
                        stream.addfile(member)
                    else:
                        member.size = 1
                        stream.addfile(member, io.BytesIO(b"x"))
                result = subprocess.run([sys.executable, str(SCRIPT), "--archive", str(archive),
                                         "--destination", str(root / "install")], capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("unsafe archive member", result.stderr)
                self.assertFalse((root / "escaped").exists())


if __name__ == "__main__":
    unittest.main()
