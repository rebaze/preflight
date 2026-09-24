package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func captureRun(ctx context.Context, o Options, mode string) (pinnedState, Profile, Report, Snapshot, string, error) {
	s, p, e := loadState(o.StateDir)
	if e != nil {
		return s, p, Report{}, Snapshot{}, "", e
	}
	r := newReport(mode, s, p)
	runs := filepath.Join(o.StateDir, "runs")
	if e = os.MkdirAll(runs, 0700); e != nil {
		return s, p, r, Snapshot{}, "", e
	}
	dir, e := os.MkdirTemp(runs, mode+"-")
	if e != nil {
		return s, p, r, Snapshot{}, "", e
	}
	snap, e := CaptureSnapshot(ctx, s.Repo, s.Baseline, o.Base, s.ProfileDigest, s.PolicyDigest, filepath.Join(dir, "snapshot"), p)
	r.Subject = snap.Subject
	return s, p, r, snap, dir, e
}
func runnerOptions(o Options, s pinnedState, p Profile, snap Snapshot, dir string) (RunnerOptions, error) {
	d, e := DependencyInputDigest(snap.Dir, p)
	npmrc := []SnapshotFile{}
	for _, f := range snap.Files {
		if filepath.Base(f.Path) == ".npmrc" {
			npmrc = append(npmrc, f)
		}
	}
	b, _ := json.Marshal(npmrc)
	return RunnerOptions{SnapshotDir: snap.Dir, StateDir: o.StateDir, RunDir: dir, DependencyDigest: d, ProfileDigest: s.ProfileDigest, PolicyDigest: s.PolicyDigest, EvaluatorDigest: s.ConftestDigest, NPMConfigDigest: pfDigest(b), RequiredFiles: s.RequiredFiles, Profile: p}, e
}
func collectFacts(s pinnedState, p Profile, snap Snapshot, stateDir string) (Facts, []byte, []byte, error) {
	facts := Facts{SchemaVersion: 1, Profile: p, Subject: snap.Subject, Tests: TestEvidence{Status: "not_applicable", ReasonCode: "no_scoped_change", Message: "No frontend or protected CI change; use --all to run the full suite.", Files: []string{}}, Scope: PolicyScope{ChangedTestPaths: []string{}}}
	baseLock, e := readBoundedFile(filepath.Join(stateDir, "baseline-lock.json"), 20<<20)
	if e != nil {
		return facts, nil, nil, e
	}
	facts.NPM, e = CollectNPM(snap.Dir, baseLock, p)
	if e != nil {
		facts.NPM.Complete = false
		facts.NPM.Errors = append(facts.NPM.Errors, e.Error())
	}
	baseline, e := readBoundedFile(filepath.Join(stateDir, "baseline-ci.yaml"), 20<<20)
	if e != nil {
		facts.Scope.BaselineCIMissing = true
	}
	candidate, e := readBoundedFile(filepath.Join(snap.Dir, workflowPath), 20<<20)
	if os.IsNotExist(e) {
		facts.Scope.CandidateCIMissing = true
	} else if e != nil {
		return facts, baseline, nil, e
	}
	for _, path := range snap.Subject.ChangedPaths {
		if strings.HasPrefix(path, testsPrefix) || path == frontendRoot+"/scripts/dependency-compatibility.test.mjs" || (strings.HasPrefix(path, frontendRoot+"/apps/web/") && (strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".spec.ts"))) {
			facts.Scope.ChangedTestPaths = append(facts.Scope.ChangedTestPaths, path)
		}
	}
	return facts, baseline, candidate, nil
}
func evaluateFacts(ctx context.Context, o Options, s pinnedState, dir string, facts Facts, baseline, candidate []byte) ([]Finding, error) {
	return Evaluate(ctx, EvalOptions{ConftestPath: s.ConftestPath, Digest: s.ConftestDigest, PolicyDir: filepath.Join(o.StateDir, "policy"), WorkDir: filepath.Join(dir, "evaluation"), Facts: facts, BaselineCI: baseline, CandidateCI: candidate})
}
func testsApplicable(snap Snapshot, all bool) bool {
	if all {
		return true
	}
	for _, p := range snap.Subject.ChangedPaths {
		if strings.HasPrefix(p, frontendRoot+"/") || p == workflowPath {
			return true
		}
	}
	return false
}
func testPrerequisite(s pinnedState, snap Snapshot) TestEvidence {
	f := TestEvidence{Files: []string{}}
	for _, name := range s.RequiredFiles {
		if _, e := os.Stat(filepath.Join(snap.Dir, name)); e != nil {
			f.Status = "fail"
			f.ReasonCode = "required_test_file_removed"
			f.Message = "Restore baseline test file: " + name
			return f
		}
	}
	b, e := readBoundedFile(filepath.Join(snap.Dir, "applications/frontend/apps/web/package.json"), 1<<20)
	if e != nil {
		f.Status = "missing"
		f.ReasonCode = "test_manifest_missing"
		f.Message = "EER package manifest is required."
		return f
	}
	var m struct {
		Scripts map[string]string `json:"scripts"`
	}
	if e = json.Unmarshal(b, &m); e != nil {
		f.Status = "error"
		f.ReasonCode = "test_manifest_invalid"
		f.Message = "EER package manifest is malformed."
		return f
	}
	if m.Scripts["test"] != "vitest run" {
		f.Status = "fail"
		f.ReasonCode = "test_script_changed"
		f.Message = "Approved EER test script must remain exactly vitest run; changed script was not executed."
	}
	return f
}
func npmConfigReview(snap Snapshot) []Finding {
	if len(snap.UnsupportedNPMKeys) == 0 {
		return nil
	}
	f := NewFinding("npm.configuration", "review_required", "unsupported_npmrc_keys", "Source npm configuration contains unsupported settings; only safe generated configuration is supported. Values were not copied. Explicit review is required before preparation.")
	f.Paths = snap.UnsupportedNPMKeys
	return []Finding{f}
}
func verifySnapshotCurrent(ctx context.Context, o Options, s pinnedState, p Profile, snap Snapshot, r *Report) {
	now, e := CaptureSnapshot(ctx, s.Repo, s.Baseline, o.Base, s.ProfileDigest, s.PolicyDigest, "", p)
	if e != nil || now.Subject.SnapshotDigest != snap.Subject.SnapshotDigest {
		r.Current = false
		f := NewFinding("evidence.validity", "missing", "workspace_changed_during_run", "Workspace changed during capture or execution. Evidence describes the captured snapshot; run check again for the current workspace.")
		f.NextActions = []NextAction{{Description: "Re-evaluate the current workspace", Command: "preflight", Args: []string{"check", "--state-dir", o.StateDir, "--all"}}}
		r.Findings = append(r.Findings, f)
	}
}
func Prepare(ctx context.Context, o Options, allowDownloads bool) error {
	if !allowDownloads {
		return fmt.Errorf("prepare requires explicit --allow-downloads")
	}
	s, p, r, snap, dir, e := captureRun(ctx, o, "prepare")
	if e != nil {
		return e
	}
	if len(snap.UnsupportedNPMKeys) > 0 {
		return fmt.Errorf("unsupported source npmrc settings require review: %s", strings.Join(snap.UnsupportedNPMKeys, ", "))
	}
	facts, baseline, candidate, e := collectFacts(s, p, snap, o.StateDir)
	if e != nil {
		return e
	}
	findings, e := evaluateFacts(ctx, o, s, dir, facts, baseline, candidate)
	if e != nil {
		return e
	}
	for _, f := range findings {
		if f.ControlID == "npm.dependencies" && (f.Status == "fail" || f.Status == "missing" || f.Status == "error") {
			return fmt.Errorf("dependency preparation refused: %s: %s", f.ReasonCode, f.Message)
		}
	}
	ro, e := runnerOptions(o, s, p, snap, dir)
	if e != nil {
		return e
	}
	_, e = PrepareRuntime(ctx, ro)
	if e != nil {
		return e
	}
	verifySnapshotCurrent(ctx, o, s, p, snap, &r)
	if !r.Current {
		return fmt.Errorf("workspace_changed_during_run: source changed during preparation; run prepare again")
	}
	return nil
}
func Check(ctx context.Context, o Options) (Report, error) {
	s, p, r, snap, dir, e := captureRun(ctx, o, "check")
	if e != nil {
		return r, e
	}
	facts, baseline, candidate, e := collectFacts(s, p, snap, o.StateDir)
	if e != nil {
		return r, e
	}
	r.Findings = append(r.Findings, npmConfigReview(snap)...)
	if testsApplicable(snap, o.All) {
		facts.Tests = testPrerequisite(s, snap)
		if facts.Tests.Status == "" {
			ro, depErr := runnerOptions(o, s, p, snap, dir)
			prep, prepErr := LoadPreparation(o.StateDir)
			malformedReceipt := prepErr != nil && !os.IsNotExist(prepErr)
			if depErr == nil && prepErr == nil {
				prepErr = PreparationCompatible(ro, prep)
			}
			switch {
			case depErr != nil:
				facts.Tests = TestEvidence{Status: "missing", ReasonCode: "dependency_inputs_unavailable", Message: "Valid dependency/runtime inputs are needed for preparation: " + depErr.Error(), Files: []string{}}
			case len(snap.UnsupportedNPMKeys) > 0:
				facts.Tests = TestEvidence{Status: "missing", ReasonCode: "npm_configuration_review_required", Message: "Resolve unsupported source npm configuration before isolated preparation.", Files: []string{}}
			case malformedReceipt:
				facts.Tests = TestEvidence{Status: "error", ReasonCode: "preparation_receipt_invalid", Message: "Preparation receipt is malformed or unreadable: " + prepErr.Error(), Files: []string{}}
			case prepErr != nil:
				facts.Tests = TestEvidence{Status: "missing", ReasonCode: "preparation_required", Message: "No compatible preparation receipt. Run preflight prepare --state-dir " + o.StateDir + " --allow-downloads.", Files: []string{}}
			default:
				r.Runtime.RunnerImageID = prep.ImageID
				r.Runtime.PreparationInputDigest = prep.InputDigest
				facts.Tests, e = RunTests(ctx, ro, prep)
				if e != nil {
					facts.Tests.Status = "error"
					facts.Tests.ReasonCode = "runner_execution_error"
					facts.Tests.Message = e.Error()
				}
			}
		}
	}
	findings, evalErr := evaluateFacts(ctx, o, s, dir, facts, baseline, candidate)
	r.Findings = append(r.Findings, findings...)
	if evalErr != nil {
		r.Findings = append(r.Findings, NewFinding("policy.evaluation", "error", "evaluator_error", evalErr.Error()))
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		if f.Owner == "" {
			f.Owner = p.Owner
		}
		if f.ControlID == "eer.tests" {
			for kind, path := range map[string]string{"junit": facts.Tests.JUnitPath, "vitest-json": facts.Tests.JSONPath} {
				if path != "" {
					if b, e := readBoundedFile(path, 20<<20); e == nil {
						f.Evidence = append(f.Evidence, Evidence{Kind: kind, Path: path, Digest: pfDigest(b)})
					}
				}
			}
			if f.Status == "missing" {
				f.NextActions = append(f.NextActions, NextAction{Description: "Prepare dependencies explicitly", Command: "preflight", Args: []string{"prepare", "--state-dir", o.StateDir, "--allow-downloads"}})
			}
		}
	}
	obligations(&r, p)
	verifySnapshotCurrent(ctx, o, s, p, snap, &r)
	r = finishReport(r)
	if e = stateWrite(filepath.Join(dir, "report.json"), r); e != nil {
		return r, e
	}
	return r, nil
}
