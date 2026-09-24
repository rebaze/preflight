package preflight

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func appFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	os.Mkdir(repo, 0700)
	put := func(name, body string) {
		t.Helper()
		f := filepath.Join(repo, name)
		os.MkdirAll(filepath.Dir(f), 0700)
		if err := os.WriteFile(f, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("applications/frontend/.node-version", "24.18.0\n")
	put("applications/frontend/package.json", `{"name":"fixture","packageManager":"npm@11.17.0","workspaces":["apps/*"],"overrides":{"brace-expansion":"5.0.9","js-yaml":"4.3.1"}}`)
	put("applications/frontend/apps/web/package.json", `{"name":"@example/frontend","scripts":{"test":"vitest run"}}`)
	put("applications/frontend/package-lock.json", `{"lockfileVersion":3,"packages":{"":{"name":"fixture"},"apps/web":{"name":"@example/frontend"},"node_modules/@example/frontend":{"resolved":"apps/web","link":true}}}`)
	put("applications/frontend/apps/web/test/example.test.ts", "// synthetic baseline\n")
	put(".github/workflows/ci.yml", "on: [push, pull_request]\njobs:\n  changes: {runs-on: ubuntu-latest}\n  frontend-eer-run: {needs: changes}\n  frontend-operator-run: {needs: changes}\n  build-and-test: {needs: [changes, frontend-eer-run, frontend-operator-run], if: 'always()'}\n")
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-C", repo}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git: %s %v", b, e)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-b", "main")
	git("config", "user.email", "fixture@example.invalid")
	git("config", "user.name", "Fixture")
	git("add", ".")
	git("commit", "-m", "synthetic baseline")
	return repo, git("rev-parse", "HEAD"), filepath.Join(root, "state")
}
func initFixture(t *testing.T) (string, string) {
	t.Helper()
	evaluator, _ := testEvaluator(t)
	repo, base, state := appFixture(t)
	e := Init(context.Background(), InitOptions{Repo: repo, Baseline: base, ProfilePath: "../../profiles/frontend-vitest.json", PolicyDir: "../../policy", StateDir: state, ConftestPath: evaluator})
	if e != nil {
		t.Fatal(e)
	}
	return repo, state
}
func TestPolicyBaselineIndependentOfBase(t *testing.T) {
	repo, state := initFixture(t)
	r, e := Explain(context.Background(), Options{StateDir: state, Base: "HEAD"})
	if e != nil {
		t.Fatal(e)
	}
	expected, err := canonicalPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if r.Policy.BaselineCommit == "" || r.Subject.RepoRoot != expected || r.Policy.BaselineCommit != r.Subject.BaselineCommit {
		t.Fatal(r)
	}
}
func TestStatusRejectsChangedInput(t *testing.T) {
	repo, state := initFixture(t)
	r, e := Explain(context.Background(), Options{StateDir: state})
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(state, "report.json")
	b, _ := json.Marshal(r)
	os.WriteFile(p, b, 0600)
	os.WriteFile(filepath.Join(repo, "applications/frontend/new file.ts"), []byte("second edit"), 0600)
	result, e := Status(context.Background(), Options{StateDir: state}, p)
	if e != nil {
		t.Fatal(e)
	}
	if result.Current || result.ExitCode != 2 {
		t.Fatalf("current=%v exit=%v", result.Current, result.ExitCode)
	}
}
func TestExplainDoesNotExecute(t *testing.T) {
	repo, state := initFixture(t)
	sentinel := filepath.Join(t.TempDir(), "executed")
	os.WriteFile(filepath.Join(repo, "applications/frontend/sentinel.sh"), []byte("touch "+sentinel), 0700)
	r, e := Explain(context.Background(), Options{StateDir: state})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(sentinel); !os.IsNotExist(e) {
		t.Fatal("repository code executed")
	}
	if len(r.Findings) == 0 {
		t.Fatal("missing explanations")
	}
}
func TestInitRefusesInsideAndNonemptyState(t *testing.T) {
	evaluator, _ := testEvaluator(t)
	repo, base, state := appFixture(t)
	o := InitOptions{Repo: repo, Baseline: base, ProfilePath: "../../profiles/frontend-vitest.json", PolicyDir: "../../policy", StateDir: filepath.Join(repo, "state"), ConftestPath: evaluator}
	if Init(context.Background(), o) == nil {
		t.Fatal("accepted inside state")
	}
	os.Mkdir(state, 0700)
	os.WriteFile(filepath.Join(state, "existing"), []byte("keep"), 0600)
	o.StateDir = state
	if Init(context.Background(), o) == nil {
		t.Fatal("overwrote nonempty state")
	}
}
func TestHumanJSONSameFindings(t *testing.T) {
	_, state := initFixture(t)
	r, e := Explain(context.Background(), Options{StateDir: state})
	if e != nil {
		t.Fatal(e)
	}
	var a, b bytes.Buffer
	if e = WriteReport(&a, r, "text"); e != nil {
		t.Fatal(e)
	}
	if e = WriteReport(&b, r, "json"); e != nil {
		t.Fatal(e)
	}
	var rr Report
	if e = DecodeStrict(b.Bytes(), &rr); e != nil {
		t.Fatal(e)
	}
	for _, f := range rr.Findings {
		if !strings.Contains(a.String(), f.ControlID) || !strings.Contains(a.String(), f.ReasonCode) || !strings.Contains(a.String(), f.Message) {
			t.Fatalf("text lost finding: %+v", f)
		}
	}
}

func appTrustFixture(t *testing.T) (string, string, InitOptions) {
	t.Helper()
	repo, base, state := appFixture(t)
	policy := t.TempDir()
	if err := os.WriteFile(filepath.Join(policy, "main.rego"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	evaluator := filepath.Join(t.TempDir(), "conftest")
	if err := os.WriteFile(evaluator, []byte("#!/bin/sh\nprintf 'synthetic evaluator\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return repo, state, InitOptions{Repo: repo, Baseline: base, ProfilePath: "../../profiles/frontend-vitest.json", PolicyDir: policy, StateDir: state, ConftestPath: evaluator}
}

func TestInitRepoSubdirectoryStillProtectsWholeWorktree(t *testing.T) {
	repo, _, o := appTrustFixture(t)
	o.Repo = filepath.Join(repo, "applications/frontend")
	o.StateDir = filepath.Join(repo, "local-state")
	if err := Init(context.Background(), o); err == nil {
		t.Fatal("accepted state inside real Git root")
	}
	if _, err := os.Stat(o.StateDir); !os.IsNotExist(err) {
		t.Fatal("wrote state into source")
	}
}

func TestInitSubdirectoryRecordsActualRoot(t *testing.T) {
	repo, state, o := appTrustFixture(t)
	o.Repo = filepath.Join(repo, "applications/frontend")
	if err := Init(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	s, _, err := loadState(state)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := canonicalPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if s.Repo != expected {
		t.Fatalf("pinned subdirectory instead of root: %q", s.Repo)
	}
}

func TestOutputCannotOverwriteTrustedInputs(t *testing.T) {
	repo, state, o := appTrustFixture(t)
	if err := Init(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"state.json", "profile.json", "baseline-ci.yaml", "baseline-lock.json", "baseline-package.json", "preparation.json", "preparation.json.tmp", "policy/main.rego"} {
		if err := ValidateOutputPath(state, filepath.Join(state, rel)); err == nil {
			t.Errorf("accepted protected output %s", rel)
		}
	}
	alias := filepath.Join(t.TempDir(), "repo-alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOutputPath(state, filepath.Join(alias, "report.json")); err == nil {
		t.Fatal("accepted output into source alias")
	}
	if err := ValidateOutputPath(state, filepath.Join(state, "report.json")); err != nil {
		t.Fatal(err)
	}
}

func TestStatusPreservesRuntimeAndDetectsChangedReceipt(t *testing.T) {
	_, state, o := appTrustFixture(t)
	if err := Init(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	r, err := Explain(context.Background(), Options{StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	r.Runtime.RunnerImageID = "sha256:" + strings.Repeat("a", 64)
	r.Runtime.PreparationInputDigest = strings.Repeat("b", 64)
	receipt := Preparation{SchemaVersion: 1, ImageID: r.Runtime.RunnerImageID, InputDigest: r.Runtime.PreparationInputDigest, BaseDigest: "node@sha256:" + strings.Repeat("c", 64), Architecture: "arm64", DockerfileDigest: runnerHash([]byte(runnerDockerfile)), EntrypointDigest: runnerHash([]byte(runnerEntrypoint)), Volume: "preflight-dependencies-" + strings.Repeat("d", 24), PreparedAt: "2026-09-23T00:00:00Z"}
	if err := stateWrite(filepath.Join(state, "preparation.json"), receipt); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(state, "report.json")
	if err := stateWrite(reportPath, r); err != nil {
		t.Fatal(err)
	}
	got, err := Status(context.Background(), Options{StateDir: state}, reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Current || got.Runtime != r.Runtime {
		t.Fatalf("lost runtime/current: %+v", got.Runtime)
	}
	receipt.ImageID = "sha256:" + strings.Repeat("e", 64)
	if err := stateWrite(filepath.Join(state, "preparation.json"), receipt); err != nil {
		t.Fatal(err)
	}
	got, err = Status(context.Background(), Options{StateDir: state}, reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Current || got.ExitCode != 2 {
		t.Fatalf("changed image still current: %v %v", got.Current, got.ExitCode)
	}
}

func TestInitEvaluatorVersionOutputBounded(t *testing.T) {
	_, _, o := appTrustFixture(t)
	if err := os.WriteFile(o.ConftestPath, []byte("#!/bin/sh\n/usr/bin/head -c 1048577 /dev/zero\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Init(context.Background(), o); err == nil {
		t.Fatal("accepted oversized evaluator output")
	}
}

func TestStatusRejectsMismatchedRecordedRevision(t *testing.T) {
	_, state, o := appTrustFixture(t)
	if err := Init(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	r, err := Explain(context.Background(), Options{StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"head", "mergeBase", "comparisonBase", "baselineCommit"} {
		t.Run(name, func(t *testing.T) {
			old := r
			switch name {
			case "head":
				old.Subject.Head = "incorrect"
			case "mergeBase":
				old.Subject.MergeBase = "incorrect"
			case "comparisonBase":
				old.Subject.ComparisonBase = "HEAD"
			case "baselineCommit":
				old.Subject.BaselineCommit = "incorrect"
			}
			reportPath := filepath.Join(state, "report.json")
			if err := stateWrite(reportPath, old); err != nil {
				t.Fatal(err)
			}
			got, err := Status(context.Background(), Options{StateDir: state, Base: r.Subject.ComparisonBase}, reportPath)
			if err != nil {
				t.Fatal(err)
			}
			if got.Current || got.ExitCode != 2 {
				t.Fatalf("inconsistent %s still current", name)
			}
		})
	}
}

func TestCheckEvaluatesStaticWhenRunnerMissing(t *testing.T) {
	_, state := initFixture(t)
	r, e := Check(context.Background(), Options{StateDir: state, All: true})
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]string{}
	for _, f := range r.Findings {
		seen[f.ControlID] = f.Status
	}
	if seen["eer.tests"] != "missing" || seen["npm.dependencies"] != "pass" || seen["ci.integrity"] != "pass" {
		t.Fatal(seen)
	}
}
func TestPolicyEditCannotSelfApprove(t *testing.T) {
	repo, state := initFixture(t)
	os.MkdirAll(filepath.Join(repo, "policy"), 0700)
	os.WriteFile(filepath.Join(repo, "policy/main.rego"), []byte("package main\n"), 0600)
	os.WriteFile(filepath.Join(repo, ".github/workflows/ci.yml"), []byte("on: push\njobs: {}\n"), 0600)
	r, e := Check(context.Background(), Options{StateDir: state})
	if e != nil {
		t.Fatal(e)
	}
	failed := false
	review := false
	for _, f := range r.Findings {
		if f.ControlID == "ci.integrity" && f.Status == "fail" {
			failed = true
		}
		if f.ControlID == "scope.review" {
			review = true
		}
	}
	if !failed || !review {
		t.Fatalf("fail=%v review=%v", failed, review)
	}
}
func TestEditEvaluateEditStatusRerun(t *testing.T) {
	repo, state := initFixture(t)
	r, e := Check(context.Background(), Options{StateDir: state})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExitCode != 0 {
		t.Fatalf("initial report: %+v", r.Findings)
	}
	report := filepath.Join(state, "saved.json")
	b, _ := json.Marshal(r)
	os.WriteFile(report, b, 0600)
	os.WriteFile(filepath.Join(repo, "applications/frontend/extra.ts"), []byte("first change"), 0600)
	st, e := Status(context.Background(), Options{StateDir: state}, report)
	if e != nil || st.Current || st.ExitCode != 2 {
		t.Fatalf("stale=%+v err=%v", st, e)
	}
	again, e := Check(context.Background(), Options{StateDir: state})
	if e != nil {
		t.Fatal(e)
	}
	if again.Subject.SnapshotDigest == r.Subject.SnapshotDigest {
		t.Fatal("rerun retained old identity")
	}
	if !again.Current {
		t.Fatal("rerun not current")
	}
}

func TestWorkspaceChangesDuringRun(t *testing.T) {
	repo, state := initFixture(t)
	o := Options{StateDir: state}
	s, p, r, snap, _, e := captureRun(context.Background(), o, "check")
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(repo, "applications/frontend/after-capture.ts"), []byte("changed during execution"), 0600)
	verifySnapshotCurrent(context.Background(), o, s, p, snap, &r)
	r = finishReport(r)
	if r.Current || r.ExitCode != 2 {
		t.Fatalf("current=%v exit=%v", r.Current, r.ExitCode)
	}
	found := false
	for _, f := range r.Findings {
		if f.ReasonCode == "workspace_changed_during_run" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing explicit invalidation")
	}
}

func TestMalformedPreparationIsError(t *testing.T) {
	_, state := initFixture(t)
	os.WriteFile(filepath.Join(state, "preparation.json"), []byte(`{"schemaVersion":1,"schemaVersion":2}`), 0600)
	r, e := Check(context.Background(), Options{StateDir: state, All: true})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExitCode != 3 {
		t.Fatalf("malformed receipt must be error; got %+v", r.Findings)
	}
}

func TestCompatibilityAndAdditionalEERTestsRequireReview(t *testing.T) {
	repo, state := initFixture(t)
	for _, name := range []string{"applications/frontend/scripts/dependency-compatibility.test.mjs", "applications/frontend/apps/web/extra.spec.ts"} {
		f := filepath.Join(repo, name)
		os.MkdirAll(filepath.Dir(f), 0700)
		os.WriteFile(f, []byte("// changed test implementation"), 0600)
	}
	r, e := Check(context.Background(), Options{StateDir: state})
	if e != nil {
		t.Fatal(e)
	}
	found := map[string]bool{}
	for _, f := range r.Findings {
		if f.ControlID == "eer.tests" && f.Status == "review_required" {
			for _, p := range f.Paths {
				found[p] = true
			}
		}
	}
	if len(found) != 2 {
		t.Fatalf("missing review for changed test implementations: %v", found)
	}
}
