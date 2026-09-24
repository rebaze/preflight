package preflight

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func resolveTestEvaluator(explicit string) (string, string, error) {
	selected := explicit
	if selected == "" {
		selected = "conftest"
	}
	binary, err := exec.LookPath(selected)
	if err != nil {
		return "", "", fmt.Errorf("required Conftest evaluator %q unavailable; set PREFLIGHT_CONFTEST or install conftest on PATH: %w", selected, err)
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return "", "", err
	}
	body, err := os.ReadFile(binary)
	if err != nil {
		return "", "", fmt.Errorf("read selected Conftest evaluator %q: %w", binary, err)
	}
	h := sha256.Sum256(body)
	return binary, hex.EncodeToString(h[:]), nil
}

func testEvaluator(t *testing.T) (string, string) {
	t.Helper()
	binary, digest, err := resolveTestEvaluator(os.Getenv("PREFLIGHT_CONFTEST"))
	if err != nil {
		t.Fatal(err)
	}
	return binary, digest
}

func TestResolveTestEvaluator(t *testing.T) {
	dir := t.TempDir()
	pathBinary := filepath.Join(dir, "conftest")
	explicitBinary := filepath.Join(dir, "selected-conftest")
	for path, body := range map[string]string{pathBinary: "#!/bin/sh\nexit 0\n", explicitBinary: "#!/bin/sh\nexit 1\n"} {
		if err := os.WriteFile(path, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	for _, tc := range []struct{ name, explicit, want string }{
		{"PATH fallback", "", pathBinary},
		{"explicit selection takes precedence", explicitBinary, explicitBinary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, digest, err := resolveTestEvaluator(tc.explicit)
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if path != tc.want || digest != pfDigest(body) {
				t.Fatalf("selected %q (%s), want %q (%s)", path, digest, tc.want, pfDigest(body))
			}
		})
	}
	t.Run("missing explicit selection does not fall back", func(t *testing.T) {
		if _, _, err := resolveTestEvaluator(filepath.Join(dir, "missing")); err == nil {
			t.Fatal("missing explicit evaluator silently accepted")
		}
	})
	t.Run("missing PATH prerequisite fails", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, _, err := resolveTestEvaluator(""); err == nil {
			t.Fatal("missing PATH evaluator silently accepted")
		}
	})
}

func npmFixture(t *testing.T) (string, map[string]any, Profile) {
	t.Helper()
	dir := t.TempDir()
	base := filepath.Join(dir, "applications/frontend")
	os.MkdirAll(filepath.Join(base, "apps/web"), 0700)
	p, e := LoadProfile("../../profiles/frontend-vitest.json")
	if e != nil {
		t.Fatal(e)
	}
	root := map[string]any{"workspaces": []string{"apps/*"}, "overrides": p.RequiredOverrides}
	b, _ := json.Marshal(root)
	os.WriteFile(filepath.Join(base, "package.json"), b, 0600)
	os.WriteFile(filepath.Join(base, "apps/web/package.json"), []byte(`{"name":"@example/frontend"}`), 0600)
	sri := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	lock := map[string]any{"lockfileVersion": 3, "packages": map[string]any{"": map[string]any{}, "apps/web": map[string]any{}, "node_modules/@example/frontend": map[string]any{"link": true, "resolved": "apps/web"}, "node_modules/@parcel/watcher-wasm": map[string]any{"version": "2.6.0", "resolved": "https://registry.npmjs.org/@parcel/watcher-wasm/-/watcher-wasm-2.6.0.tgz", "integrity": sri, "bundleDependencies": []string{"napi-wasm"}}, "node_modules/@parcel/watcher-wasm/node_modules/napi-wasm": map[string]any{"version": "1.1.0", "inBundle": true}, "node_modules/brace-expansion": map[string]any{"version": "5.0.9", "resolved": "https://registry.npmjs.org/brace-expansion/-/brace-expansion-5.0.9.tgz", "integrity": sri}, "node_modules/js-yaml": map[string]any{"version": "4.3.1", "resolved": "https://registry.npmjs.org/js-yaml/-/js-yaml-4.3.1.tgz", "integrity": sri}}}
	return dir, lock, p
}
func npmCollectFixture(t *testing.T, dir string, l map[string]any, p Profile) NPMFacts {
	t.Helper()
	b, _ := json.Marshal(l)
	if e := os.WriteFile(filepath.Join(dir, "applications/frontend/package-lock.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	n, e := CollectNPM(dir, b, p)
	if e != nil {
		t.Fatal(e)
	}
	return n
}
func npmEvaluateFixture(t *testing.T, n NPMFacts, p Profile) []Finding {
	t.Helper()
	return evaluateFixture(t, Facts{SchemaVersion: 1, Profile: p, NPM: n, Tests: TestEvidence{Status: "not_applicable", ReasonCode: "no_scoped_change", Message: "No scoped change", Files: []string{}}, Scope: PolicyScope{ChangedTestPaths: []string{}}}, baselineWorkflow, baselineWorkflow)
}

const baselineWorkflow = "on: {pull_request: {branches: [main]}}\njobs:\n  changes: {runs-on: ubuntu-latest}\n  frontend-eer-run: {runs-on: ubuntu-latest}\n  frontend-operator-run: {runs-on: ubuntu-latest}\n  build-and-test: {name: 'Build & Test', if: 'always()', needs: [changes, frontend-eer-run, frontend-operator-run], runs-on: ubuntu-latest}\n"

func evaluateFixture(t *testing.T, f Facts, baseline, candidate string) []Finding {
	t.Helper()
	binary, digest := testEvaluator(t)
	policy, e := filepath.Abs("../../policy")
	if e != nil {
		t.Fatal(e)
	}
	findings, e := Evaluate(context.Background(), EvalOptions{ConftestPath: binary, Digest: digest, PolicyDir: policy, WorkDir: t.TempDir(), Facts: f, BaselineCI: []byte(baseline), CandidateCI: []byte(candidate)})
	if e != nil {
		t.Fatalf("selected Conftest evaluator %q could not evaluate fixture policy (check evaluator compatibility): %v", binary, e)
	}
	return findings
}
func hasViolation(fs []Finding, control string) bool {
	for _, f := range fs {
		if f.ControlID == control && f.Status == "fail" {
			return true
		}
	}
	return false
}
func TestBundledDependencyRequiresParent(t *testing.T) {
	dir, l, p := npmFixture(t)
	if hasViolation(npmEvaluateFixture(t, npmCollectFixture(t, dir, l, p), p), "npm.dependencies") {
		t.Fatal("valid bundled inheritance rejected")
	}
	packages := l["packages"].(map[string]any)
	delete(packages["node_modules/@parcel/watcher-wasm"].(map[string]any), "bundleDependencies")
	if !hasViolation(npmEvaluateFixture(t, npmCollectFixture(t, dir, l, p), p), "npm.dependencies") {
		t.Fatal("arbitrary inBundle exemption accepted")
	}
}
func TestWorkspaceLinkCannotEscape(t *testing.T) {
	dir, l, p := npmFixture(t)
	l["packages"].(map[string]any)["node_modules/@example/frontend"].(map[string]any)["resolved"] = "../../outside"
	if !hasViolation(npmEvaluateFixture(t, npmCollectFixture(t, dir, l, p), p), "npm.dependencies") {
		t.Fatal("escaping link accepted")
	}
}
func TestOverrideChangeFails(t *testing.T) {
	dir, l, p := npmFixture(t)
	l["packages"].(map[string]any)["node_modules/brace-expansion"].(map[string]any)["version"] = "5.0.8"
	if !hasViolation(npmEvaluateFixture(t, npmCollectFixture(t, dir, l, p), p), "npm.dependencies") {
		t.Fatal("wrong override resolution accepted")
	}
}
func TestUntrustedRegistryFails(t *testing.T) {
	dir, l, p := npmFixture(t)
	l["packages"].(map[string]any)["node_modules/js-yaml"].(map[string]any)["resolved"] = "https://example.invalid/pkg.tgz"
	if !hasViolation(npmEvaluateFixture(t, npmCollectFixture(t, dir, l, p), p), "npm.dependencies") {
		t.Fatal("untrusted registry accepted")
	}
}

func TestNPMMissingRootRecordErrors(t *testing.T) {
	dir, l, p := npmFixture(t)
	delete(l["packages"].(map[string]any), "")
	b, _ := json.Marshal(l)
	os.WriteFile(filepath.Join(dir, "applications/frontend/package-lock.json"), b, 0600)
	n, e := CollectNPM(dir, nil, p)
	if e == nil || n.Complete {
		t.Fatal("lockfile without root record accepted")
	}
}
func TestNPMResolvedCredentialsRedacted(t *testing.T) {
	dir, l, p := npmFixture(t)
	l["packages"].(map[string]any)["node_modules/js-yaml"].(map[string]any)["resolved"] = "https://user:secret-token@registry.npmjs.org/pkg.tgz?token=hidden-token#private-fragment"
	n := npmCollectFixture(t, dir, l, p)
	b, _ := json.Marshal(n)
	for _, secret := range []string{"secret-token", "hidden-token", "private-fragment"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("raw sensitive URL part retained: %s", secret)
		}
	}
	if !hasViolation(npmEvaluateFixture(t, n, p), "npm.dependencies") {
		t.Fatal("credential-bearing URL accepted")
	}
}
func TestNPMRootOverrideChanged(t *testing.T) {
	dir, l, p := npmFixture(t)
	os.WriteFile(filepath.Join(dir, "applications/frontend/package.json"), []byte(`{"workspaces":["apps/*"],"overrides":{"brace-expansion":"5.0.8","js-yaml":"4.3.1"}}`), 0600)
	if !hasViolation(npmEvaluateFixture(t, npmCollectFixture(t, dir, l, p), p), "npm.dependencies") {
		t.Fatal("changed root override accepted")
	}
}
func TestNPMInvalidSRIAndBundleParentSource(t *testing.T) {
	for _, test := range []struct{ name, key, field, value string }{{"sri", "node_modules/js-yaml", "integrity", "sha512-short"}, {"parent source", "node_modules/@parcel/watcher-wasm", "resolved", "http://registry.npmjs.org/pkg.tgz"}} {
		t.Run(test.name, func(t *testing.T) {
			dir, l, p := npmFixture(t)
			l["packages"].(map[string]any)[test.key].(map[string]any)[test.field] = test.value
			if !hasViolation(npmEvaluateFixture(t, npmCollectFixture(t, dir, l, p), p), "npm.dependencies") {
				t.Fatal("invalid integrity/source accepted")
			}
		})
	}
}
