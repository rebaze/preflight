import hashlib
import importlib.util
from pathlib import Path
import tempfile
import unittest


class FormulaTest(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location("formula", Path(__file__).with_name("homebrew-formula.py"))
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.checksums = Path(self.tmp.name) / "checksums.txt"
        self.entries = {
            f"preflight_1.2.3_{os}_{arch}.tar.gz": hashlib.sha256(f"{os}/{arch}".encode()).hexdigest()
            for os in ("darwin", "linux") for arch in ("amd64", "arm64")
        }
        self.checksums.write_text("".join(f"{sha}  {name}\n" for name, sha in self.entries.items()))

    def test_formula_contains_every_platform_and_supporting_data(self):
        formula = self.module.render("v1.2.3", self.checksums)
        for name, sha in self.entries.items():
            self.assertIn(name.replace("1.2.3", "#{version}"), formula)
            self.assertIn(sha, formula)
        self.assertIn('pkgshare.install Dir["share/preflight/*"]', formula)
        self.assertIn('pkgshare.install "share/preflight/.agents"', formula)
        self.assertIn('Plugin marketplace: #{pkgshare}/.agents/plugins/marketplace.json', formula)
        self.assertIn('plugins/preflight/skills/preflight/SKILL.md', formula)
        self.assertIn('depends_on "conftest"', formula)
        self.assertIn('bin.install "preflight"', formula)

    def test_prerelease_or_unsafe_tag_cannot_replace_stable_formula(self):
        for tag in ("v1.2.3-rc.1", "v1.2.3\n", "v01.2.3", "v1.2", "1.2.3", 'v1.2.3"'):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                self.module.render(tag, self.checksums)

    def test_missing_duplicate_wrong_version_or_invalid_checksum_is_rejected(self):
        original = self.checksums.read_text()
        for content in ("", original.splitlines()[0] + "\n", original + original,
                        original.replace("1.2.3", "2.0.0"), original.replace(next(iter(self.entries.values())), "bad")):
            self.checksums.write_text(content)
            with self.subTest(content=content), self.assertRaises(ValueError):
                self.module.render("v1.2.3", self.checksums)

    def test_refuses_to_downgrade_existing_tap_formula(self):
        current = Path(self.tmp.name) / "preflight.rb"
        current.write_text('class Preflight < Formula\n  version "2.0.0"\nend\n')
        with self.assertRaises(ValueError):
            self.module.render("v1.2.3", self.checksums, current)
        current.write_text('class Preflight < Formula\n  version "1.2.3"\nend\n')
        self.assertIn('version "1.2.3"', self.module.render("v1.2.3", self.checksums, current))


if __name__ == "__main__":
    unittest.main()
