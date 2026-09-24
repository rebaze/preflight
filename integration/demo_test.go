package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	pf "github.com/rebaze/preflight/internal/preflight"
)

func testEvaluator(t *testing.T) string {
	t.Helper()
	conftest := os.Getenv("PREFLIGHT_CONFTEST")
	if conftest == "" {
		conftest = "conftest"
	}
	conftest, e := exec.LookPath(conftest)
	if e != nil {
		t.Fatalf("required Conftest evaluator unavailable; set PREFLIGHT_CONFTEST or install conftest on PATH: %v", e)
	}
	conftest, e = filepath.Abs(conftest)
	if e != nil {
		t.Fatal(e)
	}
	return conftest
}

func validateDemoImageID(image string) error {
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(image) {
		return fmt.Errorf("PREFLIGHT_DEMO_IMAGE must be an immutable local image ID (sha256 followed by 64 lowercase hex digits); use the runtime image=sha256:... value logged by TestDockerSyntheticReports")
	}
	return nil
}

func demoManifestArgs(front, image string) []string {
	return []string{"run", "--rm", "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--cpus", "2", "--memory", "2g", "--pids-limit", "512", "--tmpfs", "/tmp:rw,nosuid,nodev,size=512m", "--env", "HOME=/tmp/home", "--env", "NPM_CONFIG_GLOBALCONFIG=/dev/null", "--mount", "type=bind,src=" + front + ",dst=/work", "--workdir", "/work", "--entrypoint", "npm", image, "install", "--package-lock-only", "--ignore-scripts", "--no-audit", "--fund=false", "--registry=https://registry.npmjs.org", "--cache=/tmp/npm-cache"}
}

func TestDemoManifestUsesHostIdentity(t *testing.T) {
	args := demoManifestArgs("/synthetic/manifests", "sha256:"+strings.Repeat("a", 64))
	want := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--user" {
			if args[i+1] != want {
				t.Fatalf("manifest container user %q, want host %q", args[i+1], want)
			}
			return
		}
	}
	t.Fatal("manifest container must use the host UID:GID to keep its lockfile writable")
}

func TestDemoImageRequiresImmutableID(t *testing.T) {
	for _, invalid := range []string{"", "node:24.18.0-bookworm-slim", "sha256:abc", "sha256:" + strings.Repeat("g", 64), "node@sha256:" + strings.Repeat("a", 64)} {
		if err := validateDemoImageID(invalid); err == nil {
			t.Errorf("accepted nonlocal-image-ID %q", invalid)
		}
	}
	if err := validateDemoImageID("sha256:" + strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
}

// This opt-in test deliberately retains only synthetic local material so its CLI
// reports can be reviewed after the run. It never opens the real pilot checkout.
func TestSyntheticCLIWorkflow(t *testing.T) {
	if os.Getenv("PREFLIGHT_SYNTHETIC_DEMO") != "1" {
		t.Skip("set PREFLIGHT_SYNTHETIC_DEMO=1 to authorize manifest downloads and the synthetic Docker demo")
	}
	image := os.Getenv("PREFLIGHT_DEMO_IMAGE")
	if image == "" {
		t.Fatal("PREFLIGHT_DEMO_IMAGE is required: first run PREFLIGHT_DOCKER_TESTS=1 go test ./internal/preflight -run '^TestDockerSyntheticReports$' -v -count=1 from the project root, then set PREFLIGHT_DEMO_IMAGE to the logged runtime image=sha256:... value")
	}
	if err := validateDemoImageID(image); err != nil {
		t.Fatal(err)
	}
	conftest := testEvaluator(t)
	if output, err := exec.Command("docker", "image", "inspect", image).CombinedOutput(); err != nil {
		t.Fatalf("PREFLIGHT_DEMO_IMAGE %q is unavailable locally; check the Docker daemon and rerun TestDockerSyntheticReports to obtain an installed runner image: %v\n%s", image, err, output)
	}
	project, e := filepath.Abs("..")
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp("/tmp", "preflight-synthetic-demo-")
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("retained synthetic demo: %s", dir)
	repo := filepath.Join(dir, "repo")
	state := filepath.Join(dir, "state")
	front := filepath.Join(repo, "applications/frontend")
	binary := filepath.Join(dir, "preflight")
	write := func(rel, content string) {
		f := filepath.Join(repo, filepath.FromSlash(rel))
		if e := os.MkdirAll(filepath.Dir(f), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(f, []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
	}
	run := func(cwd, command string, args ...string) []byte {
		t.Helper()
		c := exec.Command(command, args...)
		c.Dir = cwd
		if command == "git" {
			c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_COUNT=0")
		}
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("%s %v: %v\n%s", command, args, e, b)
		}
		return b
	}
	run(project, "go", "build", "-o", binary, "./cmd/preflight")
	write("applications/frontend/package.json", `{"name":"preflight-synthetic","private":true,"version":"1.0.0","packageManager":"npm@11.17.0","workspaces":["apps/*"],"devDependencies":{"vitest":"4.1.10","brace-expansion":"5.0.9","js-yaml":"4.3.1"},"overrides":{"brace-expansion":"5.0.9","js-yaml":"4.3.1"}}`)
	write("applications/frontend/apps/einfache-erechnung/package.json", `{"name":"@clarula/einfache-erechnung-frontend","private":true,"version":"1.0.0","type":"module","scripts":{"test":"vitest run"}}`)
	write("applications/frontend/.node-version", "24.18.0\n")
	write("applications/frontend/.npmrc", "save-exact=true\nfund=false\naudit=false\nregistry=https://registry.npmjs.org\n")
	// At this point the mount contains only manifests and safe npm configuration.
	// Keep the generated lockfile writable by the host on rootful Linux Docker.
	run(project, "docker", demoManifestArgs(front, image)...)
	source := "export const answer = 42;\n"
	write("applications/frontend/apps/einfache-erechnung/answer.js", source)
	write("applications/frontend/apps/einfache-erechnung/test/answer.test.ts", "import { test, expect } from 'vitest';\nimport { answer } from '../answer.js';\ntest('synthetic answer remains correct', () => expect(answer).toBe(42));\n")
	write("applications/frontend/scripts/apply-dependency-compatibility-patches.mjs", "// Synthetic fixture has no application compatibility patch.\n")
	write("applications/frontend/scripts/dependency-compatibility.test.mjs", "import test from 'node:test'; import assert from 'node:assert/strict'; test('synthetic compatibility', () => assert.equal(1, 1));\n")
	ci := `name: Synthetic frontend
on: {pull_request: {branches: [main]}}
permissions: {contents: read}
jobs:
  changes: {runs-on: ubuntu-latest, steps: [{run: "echo synthetic"}]}
  frontend-eer-run: {runs-on: ubuntu-latest, needs: changes, steps: [{run: "npm run test --workspace @clarula/einfache-erechnung-frontend"}]}
  frontend-operator-run: {runs-on: ubuntu-latest, needs: changes, steps: [{run: "echo synthetic"}]}
  build-and-test: {name: "Build & Test", if: "always()", runs-on: ubuntu-latest, needs: [changes, frontend-eer-run, frontend-operator-run], steps: [{run: "echo synthetic"}]}
`
	write(".github/workflows/ci.yml", ci)
	run(repo, "git", "init", "-b", "main")
	run(repo, "git", "-c", "core.hooksPath=/dev/null", "add", ".")
	run(repo, "git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Synthetic Fixture", "-c", "user.email=synthetic@example.invalid", "commit", "-m", "Synthetic baseline")
	baseline := strings.TrimSpace(string(run(repo, "git", "rev-parse", "HEAD")))
	type step struct {
		Name       string   `json:"name"`
		Args       []string `json:"args"`
		Exit       int      `json:"exit"`
		Duration   string   `json:"duration"`
		ReportPath string   `json:"reportPath"`
	}
	steps := []step{}
	cli := func(name string, want int, args ...string) pf.Report {
		t.Helper()
		start := time.Now()
		c := exec.Command(binary, args...)
		c.Dir = dir
		var stderr bytes.Buffer
		c.Stderr = &stderr
		b, e := c.Output()
		code := 0
		if e != nil {
			if x, ok := e.(*exec.ExitError); ok {
				code = x.ExitCode()
			} else {
				t.Fatal(e)
			}
		}
		out := filepath.Join(dir, name+".json")
		os.WriteFile(out, b, 0600)
		os.WriteFile(filepath.Join(dir, name+".stderr"), stderr.Bytes(), 0600)
		steps = append(steps, step{name, args, code, time.Since(start).Round(time.Millisecond).String(), out})
		if code != want {
			t.Fatalf("%s exit %d want %d\n%s\n%s", name, code, want, b, stderr.String())
		}
		var report pf.Report
		if e = json.Unmarshal(b, &report); e != nil {
			t.Fatalf("%s invalid JSON: %v: %s", name, e, b)
		}
		t.Logf("%s: exit=%d current=%v duration=%s", name, code, report.Current, steps[len(steps)-1].Duration)
		return report
	}
	finding := func(r pf.Report, control, status string) {
		t.Helper()
		for _, f := range r.Findings {
			if f.ControlID == control && f.Status == status {
				return
			}
		}
		t.Fatalf("missing %s/%s: %+v", control, status, r.Findings)
	}
	cli("init", 0, "init", "--repo", repo, "--baseline", baseline, "--profile", filepath.Join(project, "profiles/invoicex-frontend.json"), "--policy-dir", filepath.Join(project, "policy"), "--state-dir", state, "--conftest", conftest, "--format", "json")
	cli("explain", 0, "explain", "--state-dir", state, "--format", "json")
	cli("prepare", 0, "prepare", "--state-dir", state, "--allow-downloads", "--format", "json")
	check := func(name string, exit int) pf.Report {
		return cli(name, exit, "check", "--state-dir", state, "--all", "--format", "json", "--output", filepath.Join(state, name+"-report.json"))
	}
	pass := check("baseline-pass", 0)
	finding(pass, "eer.tests", "pass")
	for _, f := range pass.Findings {
		if f.ControlID == "eer.tests" && len(f.Evidence) == 0 {
			t.Fatal("test pass without real report evidence")
		}
	}
	var text bytes.Buffer
	if e = pf.WriteReport(&text, pass, "text"); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "baseline-pass.txt"), text.Bytes(), 0600)
	write("applications/frontend/apps/einfache-erechnung/answer.js", "export const answer = 41;\n")
	finding(check("assertion-failure", 1), "eer.tests", "fail")
	write("applications/frontend/apps/einfache-erechnung/answer.js", source)
	finding(check("restored-pass", 0), "eer.tests", "pass")
	lockPath := filepath.Join(front, "package-lock.json")
	lock, e := os.ReadFile(lockPath)
	if e != nil {
		t.Fatal(e)
	}
	badLock := strings.Replace(string(lock), "https://registry.npmjs.org/", "https://untrusted.example.invalid/", 1)
	if badLock == string(lock) {
		t.Fatal("synthetic lock had no registry URL")
	}
	write("applications/frontend/package-lock.json", badLock)
	bad := check("invalid-lock-source", 2)
	finding(bad, "npm.dependencies", "fail")
	finding(bad, "eer.tests", "missing")
	write("applications/frontend/package-lock.json", string(lock))
	write(".github/workflows/ci.yml", strings.Split(ci, "  build-and-test:")[0])
	finding(check("gate-removed", 1), "ci.integrity", "fail")
	write(".preflight/policy/main.rego", "package main\n# Candidate attempt to suppress all findings\n")
	write("conftest.toml", "policy = '.preflight/policy'\n")
	finding(check("candidate-policy-ignored", 1), "ci.integrity", "fail")
	write(".github/workflows/ci.yml", ci)
	renewed := check("restored-policy-pass", 0)
	finding(renewed, "eer.tests", "pass")
	write("applications/frontend/apps/einfache-erechnung/answer.js", source+"// Further source edit after a passing report.\n")
	stale := cli("stale-status", 2, "status", "--state-dir", state, "--report", filepath.Join(state, "restored-policy-pass-report.json"), "--format", "json")
	if stale.Current {
		t.Fatal("edited snapshot incorrectly current")
	}
	finding(check("renewed-pass", 0), "eer.tests", "pass")
	current := cli("current-status", 0, "status", "--state-dir", state, "--report", filepath.Join(state, "renewed-pass-report.json"), "--format", "json")
	if !current.Current {
		t.Fatal("fresh report unexpectedly stale")
	}
	invalidReport := filepath.Join(dir, "invalid-report.json")
	if e = os.WriteFile(invalidReport, []byte(`{}`), 0600); e != nil {
		t.Fatal(e)
	}
	cli("invalid-report", 3, "status", "--state-dir", state, "--report", invalidReport, "--format", "json")
	transcript, _ := json.MarshalIndent(steps, "", "  ")
	os.WriteFile(filepath.Join(dir, "commands.json"), transcript, 0600)
	summary := fmt.Sprintf("Synthetic CLI workflow completed.\nRoot: %s\nBaseline: %s\nRunner image: %s\n", dir, baseline, pass.Runtime.RunnerImageID)
	os.WriteFile(filepath.Join(dir, "summary.txt"), []byte(summary), 0600)
	t.Log(summary)
}
