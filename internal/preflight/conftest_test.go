package preflight

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func staticFacts(t *testing.T) Facts {
	dir, l, p := npmFixture(t)
	return Facts{SchemaVersion: 1, Profile: p, NPM: npmCollectFixture(t, dir, l, p), Tests: TestEvidence{Status: "not_applicable", ReasonCode: "no_scoped_change", Message: "No scoped change", Files: []string{}}, Scope: PolicyScope{ChangedTestPaths: []string{}}}
}
func TestGateDeletionFails(t *testing.T) {
	f := staticFacts(t)
	candidate := strings.Split(baselineWorkflow, "  build-and-test:")[0]
	if !hasViolation(evaluateFixture(t, f, baselineWorkflow, candidate), "ci.integrity") {
		t.Fatal("gate removal passed")
	}
}
func TestPathFilterAddedFails(t *testing.T) {
	f := staticFacts(t)
	candidate := strings.Replace(baselineWorkflow, "branches: [main]", "branches: [main], paths: ['applications/frontend/**']", 1)
	if !hasViolation(evaluateFixture(t, f, baselineWorkflow, candidate), "ci.integrity") {
		t.Fatal("path filter passed")
	}
}
func TestWorkflowFormattingOnlyPasses(t *testing.T) {
	f := staticFacts(t)
	candidate := "# a comment\n" + strings.ReplaceAll(baselineWorkflow, "ubuntu-latest", "'ubuntu-latest'")
	if hasViolation(evaluateFixture(t, f, baselineWorkflow, candidate), "ci.integrity") {
		t.Fatal("format-only change failed")
	}
}
func TestConftestMalformedOutputErrors(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "conftest")
	b := []byte("#!/bin/sh\nprintf 'not-json'\n")
	os.WriteFile(binary, b, 0700)
	fs, e := Evaluate(context.Background(), EvalOptions{ConftestPath: binary, Digest: pfDigest(b), PolicyDir: t.TempDir(), WorkDir: t.TempDir(), Facts: staticFacts(t), BaselineCI: []byte(baselineWorkflow), CandidateCI: []byte(baselineWorkflow)})
	if e == nil {
		t.Fatalf("malformed evaluator accepted: %+v", fs)
	}
}
func TestCandidateConfigCannotOverridePolicy(t *testing.T) {
	f := staticFacts(t)
	t.Setenv("CONFTEST_POLICY", "/candidate/empty-policy")
	t.Setenv("OPA_LOG_LEVEL", "debug")
	if !hasViolation(evaluateFixture(t, f, baselineWorkflow, "{}\n"), "ci.integrity") {
		t.Fatal("candidate config changed policy")
	}
}

func TestWorkflowQuotedOnEquivalent(t *testing.T) {
	f := staticFacts(t)
	candidate := strings.Replace(baselineWorkflow, "on:", "'on':", 1)
	if hasViolation(evaluateFixture(t, f, baselineWorkflow, candidate), "ci.integrity") {
		t.Fatal("quoted trigger key should be equivalent")
	}
}
func TestGateDependencyOutsideProtectedJobsRequired(t *testing.T) {
	f := staticFacts(t)
	baseline := strings.Replace(baselineWorkflow, "needs: [changes,", "needs: [extra-job, changes,", 1) + "  extra-job: {runs-on: ubuntu-latest}\n"
	candidate := strings.TrimSuffix(baseline, "  extra-job: {runs-on: ubuntu-latest}\n")
	if !hasViolation(evaluateFixture(t, f, baseline, candidate), "ci.integrity") {
		t.Fatal("unprotected baseline gate dependency may not disappear")
	}
}
func TestGateNeedsInvalidFails(t *testing.T) {
	f := staticFacts(t)
	baseline := strings.Replace(baselineWorkflow, "needs: [changes, frontend-eer-run, frontend-operator-run]", "needs: {}", 1)
	if !hasViolation(evaluateFixture(t, f, baseline, baseline), "ci.integrity") {
		t.Fatal("invalid baseline needs accepted")
	}
}
func TestConftestZeroRulesErrors(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "conftest")
	b := []byte("#!/bin/sh\nprintf '%s' '[{\"filename\":\"Combined\",\"namespace\":\"main\",\"successes\":0}]'\n")
	os.WriteFile(binary, b, 0700)
	_, e := Evaluate(context.Background(), EvalOptions{ConftestPath: binary, Digest: pfDigest(b), PolicyDir: t.TempDir(), WorkDir: t.TempDir(), Facts: staticFacts(t), BaselineCI: []byte(baselineWorkflow), CandidateCI: []byte(baselineWorkflow)})
	if e == nil {
		t.Fatal("zero evaluated rules accepted")
	}
}
func TestConftestDigestMismatchRefusesExecution(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	binary := filepath.Join(dir, "conftest")
	b := []byte("#!/bin/sh\ntouch '" + marker + "'\n")
	os.WriteFile(binary, b, 0700)
	_, e := Evaluate(context.Background(), EvalOptions{ConftestPath: binary, Digest: "incorrect", PolicyDir: t.TempDir(), WorkDir: t.TempDir(), Facts: staticFacts(t)})
	if e == nil {
		t.Fatal("changed evaluator accepted")
	}
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("untrusted evaluator was executed")
	}
}

func errorCaseEvalOptions(t *testing.T, f Facts, candidate string) EvalOptions {
	t.Helper()
	binary, digest := testEvaluator(t)
	policy, e := filepath.Abs("../../policy")
	if e != nil {
		t.Fatal(e)
	}
	return EvalOptions{ConftestPath: binary, Digest: digest, PolicyDir: policy, WorkDir: t.TempDir(), Facts: f, BaselineCI: []byte(baselineWorkflow), CandidateCI: []byte(candidate)}
}
func TestConftestPreservesTestOutcomeWhenYAMLInvalid(t *testing.T) {
	for _, status := range []string{"fail", "missing", "error"} {
		t.Run(status, func(t *testing.T) {
			f := staticFacts(t)
			f.Tests = TestEvidence{Status: status, ReasonCode: "known_test_outcome", Message: "Known test outcome before evaluation", Files: []string{}}
			fs, e := Evaluate(context.Background(), errorCaseEvalOptions(t, f, "jobs: [\n"))
			if e == nil {
				t.Fatal("malformed workflow accepted")
			}
			found := false
			for _, finding := range fs {
				if finding.ControlID == "eer.tests" && finding.Status == status && finding.ReasonCode == "known_test_outcome" {
					found = true
				}
			}
			if !found {
				t.Fatalf("known %s outcome lost: %+v; evaluator error: %v", status, fs, e)
			}
		})
	}
}
func TestConftestPreservesIncompleteNPMAlongsideViolation(t *testing.T) {
	f := staticFacts(t)
	f.NPM.Complete = false
	f.NPM.Errors = []string{"collector could not parse remaining records"}
	f.NPM.LockfileVersion = 2
	fs, e := Evaluate(context.Background(), errorCaseEvalOptions(t, f, baselineWorkflow))
	if e != nil {
		t.Fatal(e)
	}
	sawFail, sawError := false, false
	for _, finding := range fs {
		if finding.ControlID == "npm.dependencies" {
			sawFail = sawFail || finding.Status == "fail"
			sawError = sawError || finding.Status == "error"
		}
	}
	if !sawFail || !sawError || ExitCode(fs) != 3 {
		t.Fatalf("collector error and known violation must coexist: %+v", fs)
	}
}
func TestConftestSuccessfulEvaluationDoesNotDuplicateCollectorFindings(t *testing.T) {
	for _, status := range []string{"fail", "missing", "error"} {
		t.Run(status, func(t *testing.T) {
			f := staticFacts(t)
			f.Tests = TestEvidence{Status: status, ReasonCode: "known_test_outcome", Message: "Known test outcome", Files: []string{}}
			fs, e := Evaluate(context.Background(), errorCaseEvalOptions(t, f, baselineWorkflow))
			if e != nil {
				t.Fatal(e)
			}
			count := 0
			for _, finding := range fs {
				if finding.ControlID == "eer.tests" && finding.Status == status && finding.ReasonCode == "known_test_outcome" {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("want exactly one known finding, got %d: %+v", count, fs)
			}
		})
	}
}
func TestCheckRetainsPrerequisiteFailureAlongsideInvalidWorkflow(t *testing.T) {
	repo, state := initFixture(t)
	if e := os.Remove(filepath.Join(repo, "applications/frontend/apps/einfache-erechnung/test/example.test.ts")); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(repo, ".github/workflows/ci.yml"), []byte("jobs: [\n"), 0600); e != nil {
		t.Fatal(e)
	}
	r, e := Check(context.Background(), Options{StateDir: state, All: true})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExitCode != 3 || !hasViolation(r.Findings, "eer.tests") {
		t.Fatalf("known prerequisite failure must survive evaluator error: %+v", r)
	}
	sawError := false
	for _, f := range r.Findings {
		if f.ControlID == "policy.evaluation" && f.Status == "error" {
			sawError = true
		}
	}
	if !sawError {
		t.Fatalf("missing evaluator error: %+v", r.Findings)
	}
}

func TestConftestRequiresExpectedControlsNotUnrelatedRules(t *testing.T) {
	f := staticFacts(t)
	o := errorCaseEvalOptions(t, f, "{}\n")
	o.PolicyDir = t.TempDir()
	unrelated := []byte("package main\nimport rego.v1\nviolation_unrelated_a := []\nviolation_unrelated_b := []\nviolation_unrelated_c := []\n")
	if e := os.WriteFile(filepath.Join(o.PolicyDir, "main.rego"), unrelated, 0600); e != nil {
		t.Fatal(e)
	}
	fs, e := Evaluate(context.Background(), o)
	if e == nil {
		t.Fatalf("unrelated rules invented success: %+v", fs)
	}
	for _, finding := range fs {
		if finding.Status == "pass" {
			t.Fatalf("unchecked control passed: %+v", finding)
		}
	}
}
