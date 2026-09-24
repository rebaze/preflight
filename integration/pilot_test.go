package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	pf "github.com/rebaze/preflight/internal/preflight"
	"strings"
	"testing"
)

// This runs the public CLI against synthetic Git data. Actual Docker evidence
// and the read-only real pilot are separately recorded in docs/pilot-results.md.
func TestPilotCLIWorkflow(t *testing.T) {
	evaluator := testEvaluator(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	state := filepath.Join(root, "state")
	bin := filepath.Join(root, "preflight")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/preflight")
	cmd.Dir = ".."
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build %v %s", e, b)
	}
	put := func(p, s string) {
		t.Helper()
		full := filepath.Join(repo, p)
		if e := os.MkdirAll(filepath.Dir(full), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(full, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	put("applications/frontend/.node-version", "24.18.0\n")
	put("applications/frontend/package.json", `{"packageManager":"npm@11.17.0","workspaces":["apps/*"],"overrides":{"brace-expansion":"5.0.9","js-yaml":"4.3.1"}}`)
	put("applications/frontend/apps/einfache-erechnung/package.json", `{"name":"@clarula/einfache-erechnung-frontend","scripts":{"test":"vitest run"}}`)
	put("applications/frontend/package-lock.json", `{"lockfileVersion":3,"packages":{"":{},"apps/einfache-erechnung":{"name":"@clarula/einfache-erechnung-frontend"},"node_modules/@clarula/einfache-erechnung-frontend":{"link":true,"resolved":"apps/einfache-erechnung"}}}`)
	testPath := "applications/frontend/apps/einfache-erechnung/test/synthetic.test.ts"
	put(testPath, "// synthetic baseline test\n")
	put(".github/workflows/ci.yml", "on: [push, pull_request]\njobs:\n  changes: {runs-on: ubuntu-latest}\n  frontend-eer-run: {needs: changes}\n  frontend-operator-run: {needs: changes}\n  build-and-test: {needs: [changes, frontend-eer-run, frontend-operator-run], if: 'always()'}\n")
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", repo, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v %s", e, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-b", "main")
	git("config", "user.name", "Synthetic fixture")
	git("config", "user.email", "fixture@example.invalid")
	git("add", ".")
	git("commit", "-m", "Synthetic baseline")
	invoke := func(want int, args ...string) pf.Report {
		t.Helper()
		c := exec.Command(bin, append(args, "--format", "json")...)
		var out, err bytes.Buffer
		c.Stdout = &out
		c.Stderr = &err
		runErr := c.Run()
		code := 0
		if runErr != nil {
			if x, ok := runErr.(*exec.ExitError); ok {
				code = x.ExitCode()
			} else {
				t.Fatal(runErr)
			}
		}
		if code != want {
			t.Fatalf("%v: exit %d want %d stdout=%s stderr=%s", args, code, want, out.String(), err.String())
		}
		var r pf.Report
		if e := pf.DecodeStrict(out.Bytes(), &r); e != nil {
			t.Fatalf("invalid CLI JSON: %v %s", e, out.String())
		}
		if e := pf.ValidateReport(r); e != nil {
			t.Fatal(e)
		}
		return r
	}
	invoke(0, "init", "--repo", repo, "--baseline", git("rev-parse", "HEAD"), "--profile", "../profiles/invoicex-frontend.json", "--policy-dir", "../policy", "--state-dir", state, "--conftest", evaluator)
	invoke(0, "explain", "--state-dir", state)
	reportPath := filepath.Join(state, "report.json")
	invoke(0, "check", "--state-dir", state, "--output", reportPath)
	missing := invoke(2, "check", "--state-dir", state, "--all")
	invoke(0, "status", "--state-dir", state, "--report", reportPath)
	os.Remove(filepath.Join(repo, testPath))
	invoke(1, "check", "--state-dir", state)
	invoke(2, "status", "--state-dir", state, "--report", reportPath)
	put(testPath, "// synthetic baseline test\n")
	invoke(0, "check", "--state-dir", state)
	original, e := os.ReadFile(filepath.Join(state, "profile.json"))
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(state, "profile.json"), []byte("{}"), 0600)
	invoke(3, "check", "--state-dir", state)
	os.WriteFile(filepath.Join(state, "profile.json"), original, 0600)
	if os.Getenv("PREFLIGHT_WRITE_EXAMPLES") == "1" {
		missing.Subject.RepoRoot = "/synthetic/pilot"
		missing.Subject.BaselineCommit = "synthetic-baseline"
		missing.Subject.Head = "synthetic-baseline"
		missing.Subject.ComparisonBase = "synthetic-baseline"
		missing.Subject.MergeBase = "synthetic-baseline"
		missing.Policy.BaselineCommit = "synthetic-baseline"
		for i := range missing.Findings {
			f := &missing.Findings[i]
			f.Message = strings.ReplaceAll(f.Message, state, "/private/preflight-state")
			for j := range f.NextActions {
				for k, v := range f.NextActions[j].Args {
					f.NextActions[j].Args[k] = strings.ReplaceAll(v, state, "/private/preflight-state")
				}
			}
		}
		pf.FinalizeReport(&missing)
		for _, format := range []string{"json", "text"} {
			var b bytes.Buffer
			if e := pf.WriteReport(&b, missing, format); e != nil {
				t.Fatal(e)
			}
			ext := format
			if ext == "text" {
				ext = "txt"
			}
			if e := os.WriteFile("../docs/examples/report."+ext, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	// Verify one model serializes all findings; no separate agent verdict exists.
	b, _ := json.Marshal(missing)
	var decoded pf.Report
	if e := pf.DecodeStrict(b, &decoded); e != nil {
		t.Fatal(e)
	}
	if len(decoded.Findings) != len(missing.Findings) {
		t.Fatal("findings lost")
	}
}
