package preflight

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunnerHasNoSourceWriteMount(t *testing.T) {
	args := testContainerArgs("pf-test-own", "sha256:fixed", "pf-work", "pf-out")
	s := strings.Join(args, " ")
	if strings.Contains(s, "/Users/") || strings.Contains(s, "docker.sock") || !strings.Contains(s, "--read-only") || !strings.Contains(s, "--user 1000:1000") {
		t.Fatal(s)
	}
}
func TestRunnerExecutionHasNoNetwork(t *testing.T) {
	s := strings.Join(testContainerArgs("pf-test-own", "image", "work", "out"), " ")
	for _, v := range []string{"--network none", "--cap-drop ALL", "--security-opt no-new-privileges", "--cpus 2", "--memory 2g", "--pids-limit 512"} {
		if !strings.Contains(s, v) {
			t.Fatalf("missing %s in %s", v, s)
		}
	}
}
func TestPrepareDisablesLifecycleScripts(t *testing.T) {
	s := strings.Join(prepareContainerArgs("pf-prep", "image", "volume"), " ")
	for _, v := range []string{"--ignore-scripts", "--no-audit", "--fund=false", "--registry=https://registry.npmjs.org"} {
		if !strings.Contains(s, v) {
			t.Fatal(s)
		}
	}
	if strings.Contains(s, "--network none") {
		t.Fatal("explicit preparation must fetch dependencies")
	}
}
func TestMissingJUnitNotPass(t *testing.T) {
	e := parseTestEvidence(nil, nil, 0, []string{"a.test.ts"})
	if e.Status != "missing" {
		t.Fatalf("%+v", e)
	}
}
func TestEmptyJUnitNotPass(t *testing.T) {
	e := parseTestEvidence([]byte(`<testsuites/>`), []byte(`{"testResults":[]}`), 0, nil)
	if e.Status != "fail" {
		t.Fatalf("%+v", e)
	}
}
func TestFailedJUnitIsFailure(t *testing.T) {
	e := parseTestEvidence([]byte(`<testsuites><testsuite><testcase name="x"><failure>bad</failure></testcase></testsuite></testsuites>`), []byte(`{"testResults":[]}`), 1, nil)
	if e.Status != "fail" || e.Failures != 1 {
		t.Fatalf("%+v", e)
	}
}
func TestNestedJUnitNotDoubleCounted(t *testing.T) {
	e := parseTestEvidence([]byte(`<testsuites tests="1"><testsuite tests="1"><testsuite tests="1"><testcase name="x"/></testsuite></testsuite></testsuites>`), []byte(`{"testResults":[{"name":"/work/applications/frontend/apps/einfache-erechnung/test/a.test.ts","assertionResults":[{"status":"passed"}]}]}`), 0, []string{"applications/frontend/apps/einfache-erechnung/test/a.test.ts"})
	if e.Status != "pass" || e.Tests != 1 {
		t.Fatalf("%+v", e)
	}
}
func TestRequiredFileMissing(t *testing.T) {
	e := parseTestEvidence([]byte(`<testsuite><testcase/></testsuite>`), []byte(`{"testResults":[]}`), 0, []string{"test/gone.test.ts"})
	if e.Status != "fail" || e.ReasonCode != "required_test_file_missing" {
		t.Fatalf("%+v", e)
	}
}
func TestPreparedInputsChanged(t *testing.T) {
	o := RunnerOptions{DependencyDigest: "one", ProfileDigest: "p", PolicyDigest: "q", EvaluatorDigest: "e"}
	p := Preparation{SchemaVersion: 1, InputDigest: preparationInputDigest(o)}
	o.DependencyDigest = "two"
	if PreparationCompatible(o, p) == nil {
		t.Fatal("stale preparation accepted")
	}
}
func TestNoHostFallback(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := dockerCommand(context.Background(), nil, "version")
	if err == nil {
		t.Fatal("Docker absence accepted")
	}
}
func TestTimeoutCleansOwnContainer(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + log + "\"\nif [ \"$1\" = start ]; then exec /bin/sleep 5; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := runOwnedContainer(ctx, "pf-test-owned", nil)
	if err == nil {
		t.Fatal("expected timeout")
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	if !strings.Contains(s, "rm -f pf-test-owned") || strings.Contains(s, "prune") {
		t.Fatal(s)
	}
}
func TestMalformedJUnitIsError(t *testing.T) {
	e := parseTestEvidence([]byte(`<testsuite>`), nil, 0, nil)
	if e.Status != "error" {
		t.Fatalf("%+v", e)
	}
}

func TestDockerSyntheticReports(t *testing.T) {
	if os.Getenv("PREFLIGHT_DOCKER_TESTS") != "1" {
		t.Skip("set PREFLIGHT_DOCKER_TESTS=1 for explicit Docker preparation and synthetic execution")
	}
	snapshot := t.TempDir()
	front := filepath.Join(snapshot, "applications/frontend")
	if e := os.MkdirAll(front, 0700); e != nil {
		t.Fatal(e)
	}
	for n, b := range map[string]string{"package.json": `{"name":"synthetic-runner","version":"1.0.0","packageManager":"npm@11.17.0","workspaces":[]}`, "package-lock.json": `{"name":"synthetic-runner","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"synthetic-runner","version":"1.0.0"}}}`, ".node-version": "24.18.0\n"} {
		if e := os.WriteFile(filepath.Join(front, n), []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
	}
	o := RunnerOptions{SnapshotDir: snapshot, StateDir: t.TempDir(), Profile: Profile{NodeVersion: "24.18.0", NPMVersion: "11.17.0", TestWorkspace: "@clarula/einfache-erechnung-frontend"}}
	p, e := PrepareRuntime(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	defer cleanupVolume(p.Volume)
	t.Logf("runtime image=%s base=%s architecture=%s", p.ImageID, p.BaseDigest, p.Architecture)
	for _, tc := range []struct{ name, junit, json, status string }{{"pass", `<testsuite><testcase name="one"/></testsuite>`, `{"testResults":[{"name":"/work/test/a.test.ts","assertionResults":[{"status":"passed"}]}]}`, "pass"}, {"fail", `<testsuite><testcase name="one"><failure>synthetic</failure></testcase></testsuite>`, `{"testResults":[]}`, "fail"}, {"missing", "", "", "missing"}} {
		t.Run(tc.name, func(t *testing.T) {
			work, out := runnerID("work"), runnerID("output")
			for _, v := range []string{work, out} {
				if _, e := dockerCommand(context.Background(), nil, "volume", "create", v); e != nil {
					t.Fatal(e)
				}
				defer cleanupVolume(v)
			}
			if e := setupVolume(context.Background(), p.ImageID, work, out, p.Volume, ""); e != nil {
				t.Fatal(e)
			}
			name := runnerID("synthetic")
			args := testContainerArgs(name, p.ImageID, work, out)
			args = append(args[:len(args)-1], "--entrypoint", "node", p.ImageID, "-e")
			script := ""
			if tc.junit != "" {
				j, _ := json.Marshal(tc.junit)
				v, _ := json.Marshal(tc.json)
				script = "require('fs').writeFileSync('/out/vitest.xml'," + string(j) + ");require('fs').writeFileSync('/out/vitest.json'," + string(v) + ");"
			}
			args = append(args, script)
			if _, e := dockerCommand(context.Background(), nil, args...); e != nil {
				t.Fatal(e)
			}
			defer cleanupContainer(name)
			if _, e := dockerCommand(context.Background(), nil, "start", "--attach", name); e != nil {
				t.Fatal(e)
			}
			reports, e := collectContainerReports(context.Background(), name)
			if e != nil {
				t.Fatal(e)
			}
			got := parseTestEvidence(reports["vitest.xml"], reports["vitest.json"], 0, []string{"test/a.test.ts"})
			if got.Status != tc.status {
				t.Fatalf("got %+v want %s", got, tc.status)
			}
			t.Logf("actual evidence status=%s reason=%s", got.Status, got.ReasonCode)
		})
	}
}

func TestJUnitMultipleRootsRejected(t *testing.T) {
	e := parseTestEvidence([]byte(`<testsuite><testcase/></testsuite><testsuite><testcase/></testsuite>`), []byte(`{"testResults":[{"name":"a","assertionResults":[{"status":"passed"},{"status":"passed"}]}]}`), 0, nil)
	if e.Status != "error" {
		t.Fatalf("multiple XML roots accepted: %+v", e)
	}
}
func TestPreparedManifestDigestChanges(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "applications/frontend")
	os.MkdirAll(filepath.Join(base, "apps/a"), 0700)
	for n, v := range map[string]string{"package.json": `{"packageManager":"npm@11.17.0","workspaces":["apps/*"]}`, "package-lock.json": `{"lockfileVersion":3}`, ".node-version": "24.18.0", "apps/a/package.json": `{"name":"a"}`} {
		if e := os.WriteFile(filepath.Join(base, n), []byte(v), 0600); e != nil {
			t.Fatal(e)
		}
	}
	p := Profile{NodeVersion: "24.18.0", NPMVersion: "11.17.0", TestWorkspace: "@clarula/einfache-erechnung-frontend"}
	a, e := DependencyInputDigest(dir, p)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(base, "apps/a/package.json"), []byte(`{"name":"b"}`), 0600)
	b, e := DependencyInputDigest(dir, p)
	if e != nil {
		t.Fatal(e)
	}
	if a == b {
		t.Fatal("workspace manifest edit did not invalidate preparation")
	}
}
func TestRuntimeFilesMatchCompiledRunner(t *testing.T) {
	for n, want := range map[string]string{"Dockerfile": runnerDockerfile, "run-tests.sh": runnerEntrypoint} {
		b, e := os.ReadFile(filepath.Join("../../runtime", n))
		if os.IsNotExist(e) {
			b, e = os.ReadFile(filepath.Join("runtime", n))
		}
		if e != nil {
			t.Fatal(e)
		}
		if string(b) != want {
			t.Fatalf("runtime/%s differs from compiled pinned content", n)
		}
	}
}

func TestPreparationRejectsMissingAndDuplicateFields(t *testing.T) {
	for _, v := range []string{`{"schemaVersion":1}`, `{"schemaVersion":1,"schemaVersion":1}`} {
		d := t.TempDir()
		os.WriteFile(filepath.Join(d, "preparation.json"), []byte(v), 0600)
		if _, e := LoadPreparation(d); e == nil {
			t.Fatal("invalid receipt accepted")
		}
	}
}
func TestPreparationRuntimeIdentityRequired(t *testing.T) {
	o := RunnerOptions{DependencyDigest: "d"}
	p := Preparation{SchemaVersion: 1, InputDigest: preparationInputDigest(o)}
	if PreparationCompatible(o, p) == nil {
		t.Fatal("missing runtime identity accepted")
	}
}

func TestRequiredFileMustExecuteTests(t *testing.T) {
	e := parseTestEvidence([]byte(`<testsuite><testcase/></testsuite>`), []byte(`{"testResults":[{"name":"a.test.ts","assertionResults":[]},{"name":"b.test.ts","assertionResults":[{"status":"passed"}]}]}`), 0, []string{"a.test.ts"})
	if e.Status != "fail" {
		t.Fatalf("baseline file with zero test execution accepted: %+v", e)
	}
}

func TestPrepareChangedInputsKeepsPinnedRuntime(t *testing.T) {
	snapshot := t.TempDir()
	front := filepath.Join(snapshot, "applications/frontend")
	os.MkdirAll(front, 0700)
	for n, v := range map[string]string{"package.json": `{"packageManager":"npm@11.17.0","workspaces":[]}`, "package-lock.json": `{"lockfileVersion":3}`, ".node-version": "24.18.0"} {
		os.WriteFile(filepath.Join(front, n), []byte(v), 0600)
	}
	o := RunnerOptions{SnapshotDir: snapshot, StateDir: t.TempDir(), DependencyDigest: "old-input", Profile: Profile{NodeVersion: "24.18.0", NPMVersion: "11.17.0", TestWorkspace: "@clarula/einfache-erechnung-frontend"}}
	prior := Preparation{SchemaVersion: 1, DependencyDigest: o.DependencyDigest, BaseDigest: "node@sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("b", 64), Architecture: "arm64", DockerfileDigest: runnerHash([]byte(runnerDockerfile)), EntrypointDigest: runnerHash([]byte(runnerEntrypoint)), Volume: "preflight-dependencies-" + strings.Repeat("c", 24), PreparedAt: time.Now().UTC().Format(time.RFC3339)}
	prior.InputDigest = preparationPinnedDigest(o, prior)
	b, _ := json.Marshal(prior)
	os.WriteFile(filepath.Join(o.StateDir, "preparation.json"), b, 0600)
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nif [ \"$1\" = pull ] || [ \"$1\" = build ]; then echo unexpected-runtime-update >&2; exit 99; fi\nexit 0\n"), 0700)
	t.Setenv("PATH", bin)
	o.DependencyDigest = ""
	got, e := PrepareRuntime(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if got.ImageID != prior.ImageID || got.BaseDigest != prior.BaseDigest || got.Architecture != prior.Architecture || got.InputDigest == prior.InputDigest {
		t.Fatalf("pin or input identity incorrect: %+v", got)
	}
}
