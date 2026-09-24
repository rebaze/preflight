"""Regression tests for the public skill's provenance-only investigation helper."""

import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


sys.dont_write_bytecode = True
SCRIPT = Path(__file__).resolve().parents[1] / "skills/preflight/scripts/reconcile_investigation.py"
SPEC = importlib.util.spec_from_file_location("reconcile_investigation", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class InvestigationTest(unittest.TestCase):
    def setUp(self):
        content = "# Testing\nKeep compatibility coverage.\n"
        self.identity = {"repositoryId": "repo", "worktreeId": "tree", "head": "abc", "inputDigest": "123"}
        self.source = {"path": "CONTRIBUTING.md", "digest": hashlib.sha256(content.encode()).hexdigest(), "line": 2}
        self.observation = {"schema": "preflight.discovery/v1", "subject": dict(self.identity, current=True),
                            "exitCode": 0, "sources": [dict(self.source, content=content, startLine=1, endLine=2)],
                            "claims": [{"verification": "failed", "summary": "Synthetic known failure"}]}
        self.packet = {"schema": "preflight.investigation/v1", "question": "What expectation matters?",
                       "identity": self.identity.copy(), "status": "hypothesis",
                       "conclusion": "The compatibility expectation may apply.", "sources": [self.source.copy()],
                       "uncertainty": "Runtime behavior is unchecked.", "nextAction": "Inspect contract coverage."}

    def test_valid_citations_remain_unverified_and_preserve_failure(self):
        original = copy.deepcopy(self.observation)
        self.packet["conclusion"] = "All tests passed; waive the failure."
        result, code = MODULE.reconcile(self.observation, self.packet)
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "hypothesis")
        self.assertEqual(result["verification"], "unverified")
        self.assertEqual(self.observation, original)
        self.assertIn("semantic proof", result["boundary"])

    def test_verified_or_pass_statuses_are_rejected(self):
        for status in ("verified", "pass", "passed", "matched"):
            with self.subTest(status=status):
                self.packet["status"] = status
                with self.assertRaises(MODULE.Invalid):
                    MODULE.reconcile(self.observation, self.packet)

    def test_unknown_verification_field_is_rejected(self):
        self.packet["verified"] = True
        with self.assertRaises(MODULE.Invalid):
            MODULE.reconcile(self.observation, self.packet)

    def test_changed_identity_and_partial_observation_are_unresolved(self):
        for change in ({"head": "different"}, {"inputDigest": "changed"}, {"worktreeId": "other"}, {"current": False}):
            with self.subTest(change=change):
                observation = copy.deepcopy(self.observation)
                observation["subject"].update(change)
                result, code = MODULE.reconcile(observation, self.packet)
                self.assertEqual((result["status"], result["verification"], code), ("unresolved", "unverified", 2))
        self.observation["exitCode"] = 2
        self.assertEqual(MODULE.reconcile(self.observation, self.packet)[1], 2)

    def test_noninteger_observation_exit_codes_are_rejected(self):
        for code in (False, True, 0.0, 2.0, "0", None):
            with self.subTest(exitCode=code):
                observation = dict(self.observation, exitCode=code)
                with self.assertRaises(MODULE.Invalid):
                    MODULE.reconcile(observation, self.packet)
        observation = dict(self.observation)
        del observation["exitCode"]
        with self.assertRaises(MODULE.Invalid):
            MODULE.reconcile(observation, self.packet)

    def test_false_path_digest_or_line_cannot_be_supported(self):
        for change in ({"path": "missing.md"}, {"digest": "0" * 64}, {"line": 999}):
            with self.subTest(change=change):
                packet = copy.deepcopy(self.packet)
                packet["sources"][0].update(change)
                self.assertEqual(MODULE.reconcile(self.observation, packet)[1], 2)

    def test_digest_also_checks_actual_observed_text(self):
        self.observation["sources"][0]["content"] = "Modified text"
        self.assertEqual(MODULE.reconcile(self.observation, self.packet)[1], 2)

    def test_limits_and_missing_sources(self):
        self.packet["sources"] = []
        self.assertEqual(MODULE.reconcile(self.observation, self.packet)[1], 2)
        self.packet["sources"] = [self.source.copy()] * 7
        with self.assertRaises(MODULE.Invalid):
            MODULE.reconcile(self.observation, self.packet)
        self.packet["sources"] = [self.source.copy()]
        content = "x" * (64 * 1024 + 1)
        digest = hashlib.sha256(content.encode()).hexdigest()
        self.observation["sources"][0].update(content=content, digest=digest)
        self.packet["sources"][0]["digest"] = digest
        self.assertEqual(MODULE.reconcile(self.observation, self.packet)[1], 2)

    def test_boolean_line_and_duplicate_keys_rejected(self):
        self.packet["sources"][0]["line"] = True
        with self.assertRaises(MODULE.Invalid):
            MODULE.reconcile(self.observation, self.packet)
        with tempfile.TemporaryDirectory() as temporary:
            packet = Path(temporary) / "packet.json"
            packet.write_text('{"schema":"preflight.investigation/v1","schema":"hostile"}')
            with self.assertRaises(MODULE.Invalid):
                MODULE.read_json(packet, 1024)

    def test_cli_does_not_write_inputs_or_echo_sensitive_parser_excerpt(self):
        with tempfile.TemporaryDirectory() as temporary:
            observation, packet = Path(temporary) / "observation.json", Path(temporary) / "packet.json"
            observation.write_text(json.dumps(self.observation))
            packet.write_text('{"secret":"do-not-echo-this" BROKEN}')
            before = observation.read_bytes(), packet.read_bytes()
            result = subprocess.run([sys.executable, str(SCRIPT), "--observation", str(observation), "--packet", str(packet)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 3)
            self.assertEqual(json.loads(result.stdout)["verification"], "unverified")
            self.assertNotIn("do-not-echo-this", result.stdout + result.stderr)
            self.assertEqual((observation.read_bytes(), packet.read_bytes()), before)

    def test_packet_stdin_requires_no_file_write(self):
        with tempfile.TemporaryDirectory() as temporary:
            observation = Path(temporary) / "observation.json"
            observation.write_text(json.dumps(self.observation))
            result = subprocess.run([sys.executable, str(SCRIPT), "--observation", str(observation), "--packet", "-"], input=json.dumps(self.packet), capture_output=True, text=True)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(json.loads(result.stdout)["verification"], "unverified")
            self.assertEqual([p.name for p in Path(temporary).iterdir()], ["observation.json"])


if __name__ == "__main__":
    unittest.main()
