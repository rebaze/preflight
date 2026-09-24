import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch


class PackageTest(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location("packages", Path(__file__).with_name("check-packages.py"))
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.source = self.root / "source"
        self.dist = self.root / "dist"
        self.dist.mkdir()
        manifest = {"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                    "name": "preflight", "version": "0.1.0-rc.2", "description": "Synthetic fixture", "license": "Apache-2.0"}
        overlay = dict(manifest, skills="./skills/")
        for name, data in {
            "plugin.json": json.dumps(manifest), ".codex-plugin/plugin.json": json.dumps(overlay),
            "LICENSE": "Synthetic license", "skills/preflight/SKILL.md": "Synthetic public skill",
            "skills/preflight/references/deep/reference.md": "Synthetic nested reference",
            "skills/preflight/scripts/compatibility.sh": "#!/bin/sh\nexit 0\n",
            "package/marketplace.json": json.dumps({"name": "preflight", "plugins": [{"name": "preflight", "category": "Developer Tools", "source": {"source": "local", "path": "./plugins/preflight"}}]}),
        }.items():
            path = self.source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(data)
        self.files = {name: b"synthetic fixture" for name in self.module.REQUIRED}
        self.files.update(self.module.plugin_files(self.source))
        self.files[self.module.CATALOG] = (self.source / "package/marketplace.json").read_bytes()

    def write_archives(self, mutation=None):
        hashes = {}
        for os, arch in sorted(self.module.PLATFORMS):
            path = self.dist / f"preflight_0.1.0-rc.2_{os}_{arch}.tar.gz"
            files = dict(self.files)
            if mutation:
                mutation(files)
            with tarfile.open(path, "w:gz") as archive:
                for name, data in files.items():
                    member = tarfile.TarInfo(name)
                    member.size = len(data)
                    archive.addfile(member, io.BytesIO(data))
            hashes[path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
        (self.dist / "checksums.txt").write_text("".join(f"{sha}  {name}\n" for name, sha in hashes.items()))

    def check(self):
        with patch.object(self.module, "smoke_native") as smoke:
            self.module.check(self.dist, self.source)
        return smoke

    def test_four_archives_include_every_nested_skill_file(self):
        self.write_archives()
        self.check().assert_called_once_with(b"synthetic fixture", "0.1.0-rc.2")

    def test_missing_manifest_skill_and_nested_reference_are_rejected(self):
        for suffix in ("plugin.json", ".codex-plugin/plugin.json", "skills/preflight/SKILL.md", "skills/preflight/references/deep/reference.md"):
            with self.subTest(suffix=suffix):
                self.write_archives(lambda files: files.pop(self.module.PLUGIN_ROOT + suffix))
                with self.assertRaises(ValueError):
                    self.check()

    def test_unexpected_plugin_file_and_changed_bytes_are_rejected(self):
        for mutation in (
            lambda files: files.update({self.module.PLUGIN_ROOT + "hooks/hooks.json": b"{}"}),
            lambda files: files.update({self.module.PLUGIN_ROOT + "skills/preflight/SKILL.md": b"changed"}),
            lambda files: files.update({self.module.CATALOG: b"{}"}),
        ):
            self.write_archives(mutation)
            with self.assertRaises(ValueError):
                self.check()

    def test_manifest_and_marketplace_metadata_must_agree(self):
        original = self.module.plugin_files(self.source)
        catalog = self.files[self.module.CATALOG]
        bad = dict(original)
        overlay = json.loads(bad[self.module.PLUGIN_ROOT + ".codex-plugin/plugin.json"])
        overlay["version"] = "99.0.0"
        bad[self.module.PLUGIN_ROOT + ".codex-plugin/plugin.json"] = json.dumps(overlay).encode()
        with self.assertRaisesRegex(ValueError, "metadata mismatch"):
            self.module.validate_plugin_metadata(bad, catalog)
        for field, value in (("name", "other"), ("plugins", [])):
            changed = json.loads(catalog)
            changed[field] = value
            with self.assertRaises(ValueError):
                self.module.validate_plugin_metadata(original, json.dumps(changed))
        changed = json.loads(catalog)
        changed["plugins"][0]["source"]["path"] = "../../outside"
        with self.assertRaises(ValueError):
            self.module.validate_plugin_metadata(original, json.dumps(changed))

    def test_unsafe_members_are_rejected_without_extraction(self):
        for name, kind in (("../outside", tarfile.REGTYPE), ("/outside", tarfile.REGTYPE),
                           ("safe/../../outside", tarfile.REGTYPE), ("C:/outside", tarfile.REGTYPE),
                           ("safe\\outside", tarfile.REGTYPE), ("./preflight", tarfile.REGTYPE),
                           ("preflight", tarfile.SYMTYPE), ("preflight", tarfile.LNKTYPE),
                           ("preflight", tarfile.FIFOTYPE)):
            with self.subTest(name=name, kind=kind):
                data = io.BytesIO()
                with tarfile.open(fileobj=data, mode="w") as archive:
                    member = tarfile.TarInfo(name)
                    member.type = kind
                    member.linkname = "../../outside"
                    archive.addfile(member)
                data.seek(0)
                with tarfile.open(fileobj=data) as archive, self.assertRaises(ValueError):
                    self.module.read_archive(archive)
        self.assertFalse((self.root / "outside").exists())

    def test_duplicate_and_privileged_members_are_rejected(self):
        for duplicate in (True, False):
            data = io.BytesIO()
            with tarfile.open(fileobj=data, mode="w") as archive:
                member = tarfile.TarInfo("preflight")
                member.mode = 0o755 if duplicate else 0o4755
                archive.addfile(member)
                if duplicate:
                    archive.addfile(member)
            data.seek(0)
            with tarfile.open(fileobj=data) as archive, self.assertRaises(ValueError):
                self.module.read_archive(archive)

    def test_archive_and_checksum_inventories_remain_exact(self):
        self.write_archives()
        (self.dist / "checksums.txt").write_text((self.dist / "checksums.txt").read_text() + "a" * 64 + "  plugin.zip\n")
        with self.assertRaises(ValueError):
            self.check()
        self.write_archives()
        next(self.dist.glob("*.tar.gz")).unlink()
        with self.assertRaises(ValueError):
            self.check()

    def test_public_source_symlink_parent_is_rejected(self):
        overlay = self.source / ".codex-plugin/plugin.json"
        external = self.root / "outside-manifest"
        external.mkdir()
        (external / "plugin.json").write_bytes(overlay.read_bytes())
        overlay.unlink()
        overlay.parent.rmdir()
        overlay.parent.symlink_to(external, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "symlink"):
            self.module.plugin_files(self.source)

    def test_native_smoke_checks_capabilities_and_empty_discovery(self):
        capabilities = {"schema": "preflight.capabilities/v1", "skillProtocol": 1,
                        "discoverySchema": "preflight.discovery/v1", "features": ["inspect", "github", "compare"],
                        "version": "0.1.0-rc.2", "commit": "synthetic"}
        observation = {"schema": "preflight.discovery/v1", "exitCode": 2, "sources": []}
        for compatible in (True, False):
            cap = dict(capabilities, skillProtocol=1 if compatible else 99)
            with patch.object(self.module.subprocess, "check_output", side_effect=[
                    "preflight 0.1.0-rc.2 (commit synthetic, built today)\n", json.dumps(cap).encode()]), \
                    patch.object(self.module.subprocess, "run") as run:
                run.return_value.returncode = 2
                run.return_value.stdout = json.dumps(observation).encode()
                if compatible:
                    self.module.smoke_native(b"synthetic fixture", "0.1.0-rc.2")
                    self.assertEqual(run.call_args_list[-1].args[0][1], "inspect")
                else:
                    with self.assertRaisesRegex(ValueError, "plugin protocol"):
                        self.module.smoke_native(b"synthetic fixture", "0.1.0-rc.2")


if __name__ == "__main__":
    unittest.main()
