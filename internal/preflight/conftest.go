package preflight

import (
	"bytes"
	"context"
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

type PolicyScope struct {
	BaselineCIMissing  bool     `json:"baselineCIMissing"`
	CandidateCIMissing bool     `json:"candidateCIMissing"`
	ChangedTestPaths   []string `json:"changedTestPaths"`
}
type Facts struct {
	SchemaVersion int          `json:"schemaVersion"`
	Profile       Profile      `json:"profile"`
	Subject       Subject      `json:"subject"`
	Tests         TestEvidence `json:"tests"`
	NPM           NPMFacts     `json:"npm"`
	Scope         PolicyScope  `json:"scope"`
}
type EvalOptions struct {
	ConftestPath, Digest, PolicyDir, WorkDir string
	Facts                                    Facts
	BaselineCI, CandidateCI                  []byte
}
type evaluatorMessage struct {
	Message  string `json:"msg"`
	Metadata struct {
		Metadata struct {
			ControlID  string   `json:"controlId"`
			ReasonCode string   `json:"reasonCode"`
			Paths      []string `json:"paths"`
		} `json:"metadata"`
		Query string `json:"query"`
	} `json:"metadata"`
}
type evaluatorResult struct {
	Filename   string             `json:"filename"`
	Namespace  string             `json:"namespace"`
	Successes  int                `json:"successes"`
	Failures   []evaluatorMessage `json:"failures"`
	Warnings   []evaluatorMessage `json:"warnings"`
	Exceptions []json.RawMessage  `json:"exceptions"`
}
type evaluatorOutput struct {
	b        bytes.Buffer
	limit    int
	exceeded bool
}

func (b *evaluatorOutput) Write(p []byte) (int, error) {
	n := len(p)
	space := b.limit - b.b.Len()
	if space > 0 {
		take := n
		if take > space {
			take = space
		}
		b.b.Write(p[:take])
	}
	if n > space {
		b.exceeded = true
	}
	return n, nil
}
func evaluatorEnv(home string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "CONFTEST_") || strings.HasPrefix(upper, "OPA") || upper == "HOME" || strings.HasPrefix(upper, "XDG_") {
			continue
		} // Only OS execution needs are retained; secrets and user configs stay out.
		if name == "PATH" || name == "SYSTEMROOT" || name == "TMPDIR" {
			env = append(env, entry)
		}
	}
	return append(env, "HOME="+home, "XDG_CONFIG_HOME="+home, "XDG_CACHE_HOME="+home, "LC_ALL=C")
}

// Evaluate pins the executable, writes bounded inputs under tool-owned state,
// and invokes only explicit policy/format flags from a private working directory.
func Evaluate(ctx context.Context, o EvalOptions) ([]Finding, error) {
	findings, err := evaluatePolicy(ctx, o, true)
	// Observations already established by collectors and the isolated runner
	// remain true when an independent policy input or the evaluator is broken.
	// A known failure and incomplete collection can coexist for one control.
	addKnown := func(control, status, reason, message string) {
		for _, f := range findings {
			if f.ControlID == control && f.Status == status && f.ReasonCode == reason {
				return
			}
		}
		f := NewFinding(control, status, reason, message)
		f.Owner = o.Facts.Profile.Owner
		findings = append(findings, f)
	}
	switch o.Facts.Tests.Status {
	case "fail", "missing", "error":
		addKnown("eer.tests", o.Facts.Tests.Status, o.Facts.Tests.ReasonCode, o.Facts.Tests.Message)
	}
	if !o.Facts.NPM.Complete {
		if o.Facts.NPM.LockfilePresent || len(o.Facts.NPM.Errors) > 0 {
			addKnown("npm.dependencies", "error", "npm_collection_incomplete", "The npm collector could not produce complete source facts.")
		} else {
			addKnown("npm.dependencies", "missing", "lockfile_missing", "The complete npm lockfile is required.")
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.ControlID != b.ControlID {
			return a.ControlID < b.ControlID
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		if a.ReasonCode != b.ReasonCode {
			return a.ReasonCode < b.ReasonCode
		}
		return strings.Join(a.Paths, "\x00") < strings.Join(b.Paths, "\x00")
	})
	return findings, err
}

// A successful aggregate count does not prove that the expected controls exist.
// Exercise the same pinned package with tiny synthetic violating inputs before
// trusting its real-input success counts. Probe findings stay in private probe
// files and never become candidate findings. No repository code is executed.
func probePolicyControls(ctx context.Context, o EvalOptions) error {
	probe := o
	probe.WorkDir = filepath.Join(o.WorkDir, "policy-contract-probe")
	probe.BaselineCI = []byte("{}\n")
	probe.CandidateCI = []byte("{}\n")
	probe.Facts = Facts{SchemaVersion: 1, Profile: o.Facts.Profile,
		Tests: TestEvidence{Status: "fail", ReasonCode: "preflight_contract_probe", Message: "Synthetic expected-control probe", Files: []string{}},
		NPM:   NPMFacts{Complete: true, LockfilePresent: true, LockfileVersion: 2, Errors: []string{}, Overrides: o.Facts.Profile.RequiredOverrides, Packages: []NPMPackage{}},
		Scope: PolicyScope{CandidateCIMissing: true, ChangedTestPaths: []string{}},
	}
	findings, err := evaluatePolicy(ctx, probe, false)
	if err != nil {
		return fmt.Errorf("expected policy controls could not be verified: %w", err)
	}
	for control, reason := range map[string]string{"npm.dependencies": "unsupported_lockfile_version", "ci.integrity": "workflow_missing", "eer.tests": "preflight_contract_probe"} {
		found := false
		for _, f := range findings {
			if f.ControlID == control && f.Status == "fail" && f.ReasonCode == reason {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("expected policy control %s did not evaluate its required contract probe", control)
		}
	}
	return nil
}

func evaluatePolicy(ctx context.Context, o EvalOptions, verifyControls bool) ([]Finding, error) {
	findings := []Finding{}
	binary, e := readBoundedFile(o.ConftestPath, 256<<20)
	if e != nil {
		return findings, fmt.Errorf("evaluator prerequisite: %w", e)
	}
	hash := sha256.Sum256(binary)
	if hex.EncodeToString(hash[:]) != o.Digest {
		return findings, fmt.Errorf("pinned evaluator digest changed")
	}
	if o.WorkDir == "" {
		return findings, fmt.Errorf("tool-owned evaluator work directory required")
	}
	if verifyControls {
		if err := probePolicyControls(ctx, o); err != nil {
			return findings, err
		}
	}

	if e = os.MkdirAll(o.WorkDir, 0700); e != nil {
		return findings, e
	}
	for _, name := range []string{"facts.json", "baseline-ci.yaml", "candidate-ci.yaml", "home"} {
		if info, err := os.Lstat(filepath.Join(o.WorkDir, name)); err == nil && (info.Mode()&os.ModeSymlink != 0) {
			return findings, fmt.Errorf("refuse symlink evaluator input %s", name)
		}
	}
	home := filepath.Join(o.WorkDir, "home")
	if e = os.MkdirAll(home, 0700); e != nil {
		return findings, e
	}
	// Empty workflow bytes are represented explicitly; missing does not become a parser success.
	if len(o.BaselineCI) == 0 {
		o.Facts.Scope.BaselineCIMissing = true
		o.BaselineCI = []byte("{}\n")
	}
	if len(o.CandidateCI) == 0 {
		o.Facts.Scope.CandidateCIMissing = true
		o.CandidateCI = []byte("{}\n")
	}
	if o.Facts.Scope.ChangedTestPaths == nil {
		o.Facts.Scope.ChangedTestPaths = []string{}
	}
	facts, e := json.MarshalIndent(o.Facts, "", "  ")
	if e != nil {
		return findings, e
	}
	for name, data := range map[string][]byte{"facts.json": facts, "baseline-ci.yaml": o.BaselineCI, "candidate-ci.yaml": o.CandidateCI} {
		if len(data) > 20<<20 {
			return findings, fmt.Errorf("%s exceeds 20 MiB", name)
		}
		if e = os.WriteFile(filepath.Join(o.WorkDir, name), data, 0600); e != nil {
			return findings, e
		}
	}
	evaluator, e := filepath.Abs(o.ConftestPath)
	if e != nil {
		return findings, e
	}
	policy, e := filepath.Abs(o.PolicyDir)
	if e != nil {
		return findings, e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, evaluator, "test", "--combine", "--parser", "yaml", "--policy", policy, "--namespace", "main", "--output", "json", "facts.json", "baseline-ci.yaml", "candidate-ci.yaml")
	cmd.Dir = o.WorkDir
	cmd.Env = evaluatorEnv(home)
	stdout := &evaluatorOutput{limit: 20 << 20}
	stderr := &evaluatorOutput{limit: 1 << 20}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()
	if e = os.WriteFile(filepath.Join(o.WorkDir, "conftest-output.json"), stdout.b.Bytes(), 0600); e != nil {
		return findings, e
	}
	if e = os.WriteFile(filepath.Join(o.WorkDir, "conftest-stderr.log"), stderr.b.Bytes(), 0600); e != nil {
		return findings, e
	}
	if stdout.exceeded || stderr.exceeded {
		return findings, fmt.Errorf("evaluator output exceeded bounded limit")
	}
	var output []evaluatorResult
	d := json.NewDecoder(bytes.NewReader(stdout.b.Bytes()))
	d.DisallowUnknownFields()
	if e = d.Decode(&output); e != nil {
		return findings, fmt.Errorf("malformed evaluator JSON: %w; stderr: %s", e, strings.TrimSpace(stderr.b.String()))
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return findings, fmt.Errorf("trailing evaluator JSON")
	}
	evaluated := 0
	violations := 0
	for _, r := range output {
		if r.Namespace != "main" || r.Filename != "Combined" {
			return findings, fmt.Errorf("unexpected evaluator result identity %q/%q", r.Namespace, r.Filename)
		}
		if r.Successes < 0 || len(r.Exceptions) > 0 {
			return findings, fmt.Errorf("invalid evaluator counts or unsupported exceptions")
		}
		evaluated += r.Successes + len(r.Failures) + len(r.Warnings)
		for _, group := range []struct {
			status string
			items  []evaluatorMessage
		}{{"fail", r.Failures}, {"review_required", r.Warnings}} {
			for _, item := range group.items {
				m := item.Metadata.Metadata
				if m.ControlID != "eer.tests" && m.ControlID != "npm.dependencies" && m.ControlID != "ci.integrity" {
					return findings, fmt.Errorf("unknown evaluator control %q", m.ControlID)
				}
				if item.Message == "" || m.ReasonCode == "" || m.Paths == nil {
					return findings, fmt.Errorf("incomplete evaluator finding")
				}
				f := NewFinding(m.ControlID, group.status, m.ReasonCode, item.Message)
				f.Paths = m.Paths
				f.Owner = o.Facts.Profile.Owner
				findings = append(findings, f)
				if group.status == "fail" {
					violations++
				}
			}
		}
	}
	if len(output) != 1 || evaluated < 3 {
		return findings, fmt.Errorf("zero or incomplete expected policy rule evaluation: %d results, %d rules", len(output), evaluated)
	}
	if runErr != nil {
		exit, ok := runErr.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 || violations == 0 {
			return findings, fmt.Errorf("evaluator execution failure: %w; stderr: %s", runErr, strings.TrimSpace(stderr.b.String()))
		}
	} else if violations > 0 {
		return findings, fmt.Errorf("evaluator returned success despite violations")
	}
	add := func(control, status, reason, msg string) {
		f := NewFinding(control, status, reason, msg)
		f.Owner = o.Facts.Profile.Owner
		findings = append(findings, f)
	}
	failed := map[string]bool{}
	for _, f := range findings {
		if f.Status == "fail" {
			failed[f.ControlID] = true
		}
	}
	if !failed["npm.dependencies"] && o.Facts.NPM.Complete && o.Facts.NPM.LockfilePresent {
		add("npm.dependencies", "pass", "dependency_declarations_accepted", "Complete npm lockfile declarations and existing compatibility overrides satisfy the pinned bounded policy; vulnerability and license assessment are excluded.")
	}
	if !failed["ci.integrity"] {
		add("ci.integrity", "pass", "protected_workflow_unchanged", "Protected semantic workflow objects and all baseline gate dependencies are preserved.")
	}
	if !failed["eer.tests"] {
		t := o.Facts.Tests
		switch t.Status {
		case "pass":
			add("eer.tests", "pass", "tests_passed", t.Message)
		case "missing", "error", "not_applicable", "deferred":
			add("eer.tests", t.Status, t.ReasonCode, t.Message)
		default:
			return findings, fmt.Errorf("unsupported test collector status %q", t.Status)
		}
	}
	// Changed dependency records are explanatory paths on the bounded dependency finding.
	changed := []string{}
	for _, p := range o.Facts.NPM.Packages {
		if p.Changed {
			changed = append(changed, p.Path)
		}
	}
	for i := range findings {
		if findings[i].ControlID == "npm.dependencies" && findings[i].Status == "pass" {
			findings[i].Paths = changed
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.ControlID != b.ControlID {
			return a.ControlID < b.ControlID
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		if a.ReasonCode != b.ReasonCode {
			return a.ReasonCode < b.ReasonCode
		}
		return strings.Join(a.Paths, "\x00") < strings.Join(b.Paths, "\x00")
	})
	return findings, nil
}
