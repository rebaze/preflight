package preflight

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const frontendRoot = "applications/frontend"
const workflowPath = ".github/workflows/ci.yml"
const testsPrefix = "applications/frontend/apps/web/test/"

type pinnedState struct {
	SchemaVersion   int               `json:"schemaVersion"`
	Repo            string            `json:"repo"`
	Baseline        string            `json:"baseline"`
	ProfileDigest   string            `json:"profileDigest"`
	PolicyDigest    string            `json:"policyDigest"`
	ConftestPath    string            `json:"conftestPath"`
	ConftestDigest  string            `json:"conftestDigest"`
	ConftestVersion string            `json:"conftestVersion"`
	RequiredFiles   []string          `json:"requiredFiles"`
	BaselineDigests map[string]string `json:"baselineDigests"`
}

func pfDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func readBoundedFile(path string, max int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if int64(len(b)) > max {
		return nil, fmt.Errorf("file exceeds %d byte limit", max)
	}
	return b, e
}
func canonicalPath(p string) (string, error) {
	return snapshotCanonicalPath(p)
}
func outsideRepo(repo, path string) error {
	p, e := canonicalPath(path)
	if e != nil {
		return e
	}
	r, e := canonicalPath(repo)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(r, p)
	if e != nil {
		return e
	}
	if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("state/output must be outside the pilot worktree")
	}
	return nil
}

// ValidateOutputPath prevents report writes into the read-only source checkout.
func ValidateOutputPath(state, path string) error {
	s, _, e := loadState(state)
	if e != nil {
		return e
	}
	if e = outsideRepo(s.Repo, path); e != nil {
		return e
	}
	output, e := canonicalPath(path)
	if e != nil {
		return e
	}
	stateRoot, e := canonicalPath(state)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(stateRoot, output)
	if e != nil {
		return e
	}
	if rel == "." || rel == "policy" || strings.HasPrefix(rel, "policy"+string(filepath.Separator)) {
		return fmt.Errorf("report output cannot replace trusted state")
	}
	for _, name := range []string{"state.json", "profile.json", "baseline-ci.yaml", "baseline-lock.json", "baseline-package.json", "preparation.json", "preparation.json.tmp"} {
		if rel == name {
			return fmt.Errorf("report output cannot replace trusted state %s", name)
		}
	}
	if output == s.ConftestPath {
		return fmt.Errorf("report output cannot replace the pinned evaluator")
	}
	return nil
}
func hashPolicyDir(dir string) (string, error) {
	var names []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || !d.Type().IsRegular() {
			return fmt.Errorf("policy must contain regular files")
		}
		if !strings.HasSuffix(p, ".rego") {
			return fmt.Errorf("unsupported policy file %s", filepath.Base(p))
		}
		r, _ := filepath.Rel(dir, p)
		names = append(names, r)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", fmt.Errorf("empty policy package")
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		b, e := readBoundedFile(filepath.Join(dir, n), 20<<20)
		if e != nil {
			return "", e
		}
		fmt.Fprintf(h, "%d:%s%d:", len(n), n, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func stateWrite(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func Init(ctx context.Context, o InitOptions) error {
	if o.Baseline == "" || o.Repo == "" || o.StateDir == "" || o.ConftestPath == "" {
		return fmt.Errorf("init requires explicit repo, baseline, profile, policy-dir, state-dir and conftest")
	}
	repo, e := canonicalPath(o.Repo)
	if e != nil {
		return e
	}
	rootBytes, e := snapshotGitRead(ctx, repo, "rev-parse", "--show-toplevel")
	if e != nil {
		return e
	}
	repo, e = canonicalPath(strings.TrimSuffix(string(rootBytes), "\n"))
	if e != nil {
		return e
	}
	state, e := canonicalPath(o.StateDir)
	if e != nil {
		return e
	}
	if e = outsideRepo(repo, state); e != nil {
		return e
	}
	entries, e := os.ReadDir(state)
	if e == nil && len(entries) > 0 {
		return fmt.Errorf("state directory is nonempty; initialize a fresh directory")
	}
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	p, e := LoadProfile(o.ProfilePath)
	if e != nil {
		return e
	}
	pb, e := readBoundedFile(o.ProfilePath, 1<<20)
	if e != nil {
		return e
	}
	pd, e := hashPolicyDir(o.PolicyDir)
	if e != nil {
		return e
	}
	evaluator, e := filepath.Abs(o.ConftestPath)
	if e != nil {
		return e
	}
	evaluator, e = filepath.EvalSymlinks(evaluator)
	if e != nil {
		return e
	}
	eb, e := readBoundedFile(evaluator, 256<<20)
	if e != nil {
		return e
	}
	versionCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	verCmd := exec.CommandContext(versionCtx, evaluator, "--version")
	verCmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/nonexistent"}
	verCmd.WaitDelay = time.Second
	versionOut := &boundedOutput{limit: 1 << 20}
	versionErr := &boundedOutput{limit: 1 << 20}
	verCmd.Stdout = versionOut
	verCmd.Stderr = versionErr
	if e = verCmd.Run(); e != nil {
		return fmt.Errorf("conftest --version prerequisite: %w", e)
	}
	if versionOut.exceeded || versionErr.exceeded {
		return fmt.Errorf("conftest --version output exceeds 1 MiB")
	}
	vb := versionOut.buf.Bytes()
	snap, e := CaptureSnapshot(ctx, repo, o.Baseline, o.Baseline, pfDigest(pb), pd, "", p)
	if e != nil {
		return e
	}
	baseline := snap.Subject.BaselineCommit
	files, e := BaselineFiles(ctx, repo, baseline, p)
	if e != nil {
		return e
	}
	required := []string{}
	for _, f := range files {
		if strings.HasPrefix(f, testsPrefix) && strings.HasSuffix(f, ".test.ts") {
			required = append(required, f)
		}
	}
	if len(required) == 0 {
		return fmt.Errorf("baseline contains no required EER test files")
	}
	material := map[string][]byte{}
	for dst, src := range map[string]string{"baseline-ci.yaml": workflowPath, "baseline-lock.json": frontendRoot + "/package-lock.json", "baseline-package.json": frontendRoot + "/package.json"} {
		b, e := ReadBaselineFile(ctx, repo, baseline, src)
		if e != nil {
			return fmt.Errorf("baseline %s: %w", src, e)
		}
		material[dst] = b
	}
	if e = os.MkdirAll(filepath.Dir(state), 0700); e != nil {
		return e
	}
	temp, e := os.MkdirTemp(filepath.Dir(state), ".preflight-init-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	s := pinnedState{1, repo, baseline, pfDigest(pb), pd, evaluator, pfDigest(eb), strings.TrimSpace(string(vb)), required, map[string]string{}}
	if e = os.WriteFile(filepath.Join(temp, "profile.json"), pb, 0600); e != nil {
		return e
	}
	if e = os.Mkdir(filepath.Join(temp, "policy"), 0700); e != nil {
		return e
	}
	e = filepath.WalkDir(o.PolicyDir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(o.PolicyDir, path)
		dest := filepath.Join(temp, "policy", rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		b, e := readBoundedFile(path, 20<<20)
		if e != nil {
			return e
		}
		return os.WriteFile(dest, b, 0600)
	})
	if e != nil {
		return e
	}
	copied, e := hashPolicyDir(filepath.Join(temp, "policy"))
	if e != nil || copied != pd {
		return fmt.Errorf("policy changed during initialization")
	}
	for name, b := range material {
		s.BaselineDigests[name] = pfDigest(b)
		if e = os.WriteFile(filepath.Join(temp, name), b, 0600); e != nil {
			return e
		}
	}
	if e = stateWrite(filepath.Join(temp, "state.json"), s); e != nil {
		return e
	}
	if _, e = os.Stat(state); e == nil {
		if e = os.Remove(state); e != nil {
			return e
		}
	}
	return os.Rename(temp, state)
}
func loadState(dir string) (pinnedState, Profile, error) {
	var s pinnedState
	var p Profile
	b, e := readBoundedFile(filepath.Join(dir, "state.json"), 1<<20)
	if e != nil {
		return s, p, e
	}
	if e = DecodeStrict(b, &s); e != nil {
		return s, p, e
	}
	if s.SchemaVersion != 1 {
		return s, p, fmt.Errorf("unsupported state schema")
	}
	if e = outsideRepo(s.Repo, dir); e != nil {
		return s, p, e
	}
	pb, e := readBoundedFile(filepath.Join(dir, "profile.json"), 1<<20)
	if e != nil {
		return s, p, e
	}
	if pfDigest(pb) != s.ProfileDigest {
		return s, p, fmt.Errorf("pinned profile digest changed")
	}
	p, e = LoadProfile(filepath.Join(dir, "profile.json"))
	if e != nil {
		return s, p, e
	}
	pd, e := hashPolicyDir(filepath.Join(dir, "policy"))
	if e != nil {
		return s, p, e
	}
	if pd != s.PolicyDigest {
		return s, p, fmt.Errorf("pinned policy digest changed")
	}
	eb, e := readBoundedFile(s.ConftestPath, 256<<20)
	if e != nil {
		return s, p, fmt.Errorf("evaluator unavailable: %w", e)
	}
	if pfDigest(eb) != s.ConftestDigest {
		return s, p, fmt.Errorf("pinned evaluator digest changed")
	}
	for name, d := range s.BaselineDigests {
		if filepath.Base(name) != name {
			return s, p, fmt.Errorf("invalid baseline path")
		}
		b, e := readBoundedFile(filepath.Join(dir, name), 20<<20)
		if e != nil {
			return s, p, e
		}
		if pfDigest(b) != d {
			return s, p, fmt.Errorf("pinned baseline digest changed: %s", name)
		}
	}
	return s, p, nil
}
func newReport(mode string, s pinnedState, p Profile) Report {
	b := make([]byte, 12)
	rand.Read(b)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return Report{SchemaVersion: 1, Mode: mode, Authority: "local-feedback", RunID: hex.EncodeToString(b), StartedAt: now, FinishedAt: now, Current: true, Subject: Subject{RepoRoot: s.Repo, BaselineCommit: s.Baseline, ExcludedCategories: []string{}, ChangedPaths: []string{}, OutOfScopePaths: []string{}}, Policy: Policy{PackageDigest: s.PolicyDigest, ProfileDigest: s.ProfileDigest, BaselineCommit: s.Baseline}, Runtime: Runtime{EvaluatorDigest: s.ConftestDigest, EvaluatorVersion: s.ConftestVersion}, Scope: Scope{Profile: p.ID, Deferred: []string{"artifact verification at trusted build/release", "vulnerability and license assessment"}, Excluded: []string{"backend", "hosted LLM", "E2E, browser and visual tests", "deployment and infrastructure", "branch protection verification", "compliance certification", "test adequacy and business acceptance", "release authorization", "remote refs are not refreshed"}}, Findings: []Finding{}}
}
func finishReport(r Report) Report {
	r.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	FinalizeReport(&r)
	return r
}
func obligations(r *Report, p Profile) {
	f := NewFinding("artifact.verification", "deferred", "trusted_build_required", "Artifact contents and provenance require verification at a trusted build/release stage. Local feedback is not release authorization.")
	f.Owner = p.Owner
	r.Findings = append(r.Findings, f)
	if len(r.Subject.OutOfScopePaths) > 0 {
		f = NewFinding("scope.review", "review_required", "outside_selected_profile", "Changed paths outside the frontend pilot require their own checks and review; contents were not collected.")
		f.Paths = r.Subject.OutOfScopePaths
		f.Owner = p.Owner
		r.Findings = append(r.Findings, f)
	}
}
func Explain(ctx context.Context, o Options) (Report, error) {
	s, p, e := loadState(o.StateDir)
	if e != nil {
		return Report{}, e
	}
	r := newReport("explain", s, p)
	snap, e := CaptureSnapshot(ctx, s.Repo, s.Baseline, o.Base, s.ProfileDigest, s.PolicyDigest, "", p)
	if e != nil {
		return r, e
	}
	r.Subject = snap.Subject
	for _, c := range []struct{ id, msg string }{{"eer.tests", "Run the complete EER Vitest suite with the explicit compatibility patch and Node tests in offline Docker; require fresh positive JUnit evidence and every baseline test file. Changed tests require review."}, {"npm.dependencies", "Evaluate the complete npm v3 lockfile's registry/SHA512 declarations, workspace and bundled ancestry, and preserve existing brace-expansion/js-yaml compatibility overrides. No vulnerability or license verdict."}, {"ci.integrity", "Compare semantic trigger, permissions, protected jobs and gate dependencies against the pinned baseline; intentional changes require review and re-baselining."}} {
		f := NewFinding(c.id, "deferred", "explain_only", c.msg)
		f.Owner = p.Owner
		f.NextActions = []NextAction{{Description: "Evaluate the captured workspace", Command: "preflight", Args: []string{"check", "--state-dir", o.StateDir, "--all"}}}
		r.Findings = append(r.Findings, f)
	}
	obligations(&r, p)
	return finishReport(r), nil
}
func Status(ctx context.Context, o Options, reportPath string) (Report, error) {
	s, p, e := loadState(o.StateDir)
	if e != nil {
		return Report{}, e
	}
	b, e := readBoundedFile(reportPath, 20<<20)
	if e != nil {
		return Report{}, e
	}
	var old Report
	if e = DecodeStrict(b, &old); e != nil {
		return Report{}, e
	}
	if e = ValidateReport(old); e != nil {
		return Report{}, e
	}
	r := newReport("status", s, p)
	base := o.Base
	if base == "" {
		base = old.Subject.ComparisonBase
	}
	snap, e := CaptureSnapshot(ctx, s.Repo, s.Baseline, base, s.ProfileDigest, s.PolicyDigest, "", p)
	if e != nil {
		return r, e
	}
	r.Subject = snap.Subject
	r.Current = old.Current && old.Subject.RepoRoot == s.Repo && old.Subject.SnapshotDigest == snap.Subject.SnapshotDigest && old.Subject.BaselineCommit == snap.Subject.BaselineCommit && old.Subject.Head == snap.Subject.Head && old.Subject.MergeBase == snap.Subject.MergeBase && old.Subject.ComparisonBase == snap.Subject.ComparisonBase && old.Policy == r.Policy && old.Runtime.EvaluatorDigest == s.ConftestDigest && old.Runtime.EvaluatorVersion == s.ConftestVersion
	r.Runtime.RunnerImageID = old.Runtime.RunnerImageID
	r.Runtime.PreparationInputDigest = old.Runtime.PreparationInputDigest
	if old.Runtime.RunnerImageID != "" || old.Runtime.PreparationInputDigest != "" {
		receipt, err := LoadPreparation(o.StateDir)
		if err != nil || receipt.SchemaVersion != 1 || receipt.ImageID != old.Runtime.RunnerImageID || receipt.InputDigest != old.Runtime.PreparationInputDigest {
			r.Current = false
		}
	}
	r.Findings = append(r.Findings, old.Findings...)
	if !r.Current {
		f := NewFinding("evidence.validity", "missing", "stale_report", "Saved report does not cover the current source, revisions, profile or evaluator. Run check again.")
		f.NextActions = []NextAction{{Description: "Re-evaluate current inputs", Command: "preflight", Args: []string{"check", "--state-dir", o.StateDir, "--all"}}}
		r.Findings = append(r.Findings, f)
	}
	r = finishReport(r)
	if !r.Current && r.ExitCode < 3 {
		r.ExitCode = 2
	}
	return r, nil
}
