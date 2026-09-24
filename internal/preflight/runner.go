package preflight

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const safeNPMConfig = "save-exact=true\nfund=false\naudit=false\nregistry=https://registry.npmjs.org\n"
const runnerDockerfile = `ARG BASE
FROM ${BASE}
RUN npm install --global npm@11.17.0 --ignore-scripts --no-audit --fund=false --registry=https://registry.npmjs.org && test "$(node --version)" = v24.18.0 && test "$(npm --version)" = 11.17.0
COPY run-tests.sh /usr/local/bin/preflight-run-tests
RUN chmod 0555 /usr/local/bin/preflight-run-tests
WORKDIR /work/applications/frontend
ENTRYPOINT ["/usr/local/bin/preflight-run-tests"]
`
const runnerEntrypoint = `#!/bin/sh
set -eu
export HOME=/tmp/home
mkdir -p "$HOME"
node scripts/apply-dependency-compatibility-patches.mjs
node --test scripts/dependency-compatibility.test.mjs
npm --ignore-scripts run test --workspace @example/frontend -- --reporter=junit --reporter=json --outputFile.junit=/out/vitest.xml --outputFile.json=/out/vitest.json
`
const logLimit = 1 << 20
const reportLimit = 20 << 20

type RunnerOptions struct {
	SnapshotDir, StateDir, RunDir, DependencyDigest, ProfileDigest, PolicyDigest, EvaluatorDigest, NPMConfigDigest string
	RequiredFiles                                                                                                  []string
	Profile                                                                                                        Profile
}
type Preparation struct {
	SchemaVersion    int    `json:"schemaVersion"`
	InputDigest      string `json:"inputDigest"`
	DependencyDigest string `json:"dependencyDigest"`
	ProfileDigest    string `json:"profileDigest"`
	PolicyDigest     string `json:"policyDigest"`
	EvaluatorDigest  string `json:"evaluatorDigest"`
	NPMConfigDigest  string `json:"npmConfigDigest"`
	BaseDigest       string `json:"baseDigest"`
	ImageID          string `json:"imageId"`
	Architecture     string `json:"architecture"`
	DockerfileDigest string `json:"dockerfileDigest"`
	EntrypointDigest string `json:"entrypointDigest"`
	Volume           string `json:"volume"`
	PreparedAt       string `json:"preparedAt"`
}
type TestEvidence struct {
	Status     string   `json:"status"`
	ReasonCode string   `json:"reasonCode"`
	Message    string   `json:"message"`
	ExitCode   int      `json:"exitCode"`
	Tests      int      `json:"tests"`
	Failures   int      `json:"failures"`
	Errors     int      `json:"errors"`
	Skipped    int      `json:"skipped"`
	Files      []string `json:"files"`
	JUnitPath  string   `json:"junitPath"`
	JSONPath   string   `json:"jsonPath"`
	LogPath    string   `json:"logPath"`
}

func runnerHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func runnerID(prefix string) string {
	b := make([]byte, 12)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return "preflight-" + prefix + "-" + hex.EncodeToString(b)
}
func preparationInputDigest(o RunnerOptions) string {
	b, _ := json.Marshal([]string{o.DependencyDigest, o.ProfileDigest, o.PolicyDigest, o.EvaluatorDigest, o.NPMConfigDigest, runnerHash([]byte(runnerDockerfile)), runnerHash([]byte(runnerEntrypoint))})
	return runnerHash(b)
}
func preparationPinnedDigest(o RunnerOptions, p Preparation) string {
	b, _ := json.Marshal([]string{preparationInputDigest(o), p.BaseDigest, p.ImageID, p.Architecture, p.DockerfileDigest, p.EntrypointDigest})
	return runnerHash(b)
}
func validSHAIdentity(s, prefix string) bool {
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	b, e := hex.DecodeString(strings.TrimPrefix(s, prefix))
	return e == nil && len(b) == 32
}
func validatePreparation(p Preparation) error {
	if p.SchemaVersion != 1 || !validSHAIdentity(p.ImageID, "sha256:") || !validSHAIdentity(p.BaseDigest, "node@sha256:") || !validSHAIdentity(p.InputDigest, "") {
		return errors.New("invalid preparation schema or image identity")
	}
	if p.Architecture != "arm64" && p.Architecture != "amd64" {
		return errors.New("unsupported prepared image architecture")
	}
	if p.DockerfileDigest != runnerHash([]byte(runnerDockerfile)) || p.EntrypointDigest != runnerHash([]byte(runnerEntrypoint)) {
		return errors.New("runner Dockerfile or entrypoint identity changed")
	}
	suffix := strings.TrimPrefix(p.Volume, "preflight-dependencies-")
	v, e := hex.DecodeString(suffix)
	if suffix == p.Volume || e != nil || len(v) != 12 {
		return errors.New("invalid tool-owned dependency volume identity")
	}
	if _, e := time.Parse(time.RFC3339, p.PreparedAt); e != nil {
		return errors.New("invalid preparation timestamp")
	}
	return nil
}
func PreparationCompatible(o RunnerOptions, p Preparation) error {
	if e := validatePreparation(p); e != nil {
		return e
	}
	if p.DependencyDigest != o.DependencyDigest || p.ProfileDigest != o.ProfileDigest || p.PolicyDigest != o.PolicyDigest || p.EvaluatorDigest != o.EvaluatorDigest || p.NPMConfigDigest != o.NPMConfigDigest || p.InputDigest != preparationPinnedDigest(o, p) {
		return errors.New("preparation inputs changed; run preflight prepare --state-dir STATE --allow-downloads")
	}
	return nil
}
func LoadPreparation(stateDir string) (Preparation, error) {
	var p Preparation
	b, e := readRunnerFile(filepath.Join(stateDir, "preparation.json"), 1<<20)
	if e != nil {
		return p, e
	}
	if e = DecodeStrict(b, &p); e != nil {
		return p, e
	}
	return p, validatePreparation(p)
}

type boundedOutput struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	left := b.limit - b.buf.Len()
	if left > 0 {
		if left > n {
			left = n
		}
		_, _ = b.buf.Write(p[:left])
	}
	if n > left {
		b.exceeded = true
	}
	return n, nil
}
func dockerCommand(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = stdin
	// The client needs Docker's local connection settings, but no host environment is forwarded into containers.
	out := &boundedOutput{limit: logLimit}
	cmd.Stdout = out
	cmd.Stderr = out
	e := cmd.Run()
	if out.exceeded {
		return out.buf.Bytes(), errors.New("Docker output exceeded 1 MiB limit")
	}
	if e != nil {
		return out.buf.Bytes(), fmt.Errorf("docker %s: %w: %s", args[0], e, strings.TrimSpace(out.buf.String()))
	}
	return out.buf.Bytes(), nil
}
func cleanupContainer(name string) {
	ctx, c := context.WithTimeout(context.Background(), 15*time.Second)
	defer c()
	_, _ = dockerCommand(ctx, nil, "rm", "-f", name)
}
func cleanupVolume(name string) {
	ctx, c := context.WithTimeout(context.Background(), 15*time.Second)
	defer c()
	_, _ = dockerCommand(ctx, nil, "volume", "rm", name)
}
func runOwnedContainer(ctx context.Context, name string, stdin io.Reader) ([]byte, error) {
	defer cleanupContainer(name)
	return dockerCommand(ctx, stdin, "start", "--attach", name)
}
func isolationArgs(name, image string) []string {
	return []string{"create", "--name", name, "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--cpus", "2", "--memory", "2g", "--pids-limit", "512", "--tmpfs", "/tmp:rw,nosuid,nodev,size=512m", "--env", "HOME=/tmp/home", "--env", "NPM_CONFIG_USERCONFIG=/work/applications/frontend/.npmrc", "--env", "NPM_CONFIG_GLOBALCONFIG=/dev/null"}
}
func testContainerArgs(name, image, work, out string) []string {
	a := isolationArgs(name, image)
	return append(a, "--network", "none", "--user", "1000:1000", "--mount", "type=volume,src="+work+",dst=/work", "--mount", "type=volume,src="+out+",dst=/out", image)
}
func prepareContainerArgs(name, image, work string) []string {
	a := isolationArgs(name, image)
	return append(a, "--user", "1000:1000", "--mount", "type=volume,src="+work+",dst=/work", "--entrypoint", "npm", image, "ci", "--ignore-scripts", "--no-audit", "--fund=false", "--registry=https://registry.npmjs.org", "--cache=/tmp/npm-cache")
}

// Only package manifests belonging to the declared npm workspaces enter networked preparation.
func dependencyInputs(snapshot string, p Profile) (map[string][]byte, error) {
	if p.NodeVersion != "24.18.0" || p.NPMVersion != "11.17.0" || p.TestWorkspace != "@example/frontend" {
		return nil, errors.New("unsupported runtime/profile: require Node 24.18.0, npm 11.17.0 and the EER workspace")
	}
	base := filepath.Join(snapshot, "applications/frontend")
	root, e := readRunnerFile(filepath.Join(base, "package.json"), reportLimit)
	if e != nil {
		return nil, e
	}
	var m struct {
		PackageManager string   `json:"packageManager"`
		Workspaces     []string `json:"workspaces"`
	}
	if e = json.Unmarshal(root, &m); e != nil {
		return nil, e
	}
	if m.PackageManager != "npm@11.17.0" {
		return nil, fmt.Errorf("required packageManager npm@11.17.0, found %q", m.PackageManager)
	}
	node, e := readRunnerFile(filepath.Join(base, ".node-version"), 1024)
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(string(node)) != "24.18.0" {
		return nil, errors.New("required .node-version 24.18.0 differs")
	}
	lock, e := readRunnerFile(filepath.Join(base, "package-lock.json"), reportLimit)
	if e != nil {
		return nil, e
	}
	var l struct {
		LockfileVersion int `json:"lockfileVersion"`
	}
	if json.Unmarshal(lock, &l) != nil || l.LockfileVersion != 3 {
		return nil, errors.New("require valid npm lockfile v3")
	}
	inputs := map[string][]byte{"applications/frontend/package.json": root, "applications/frontend/package-lock.json": lock, "applications/frontend/.node-version": node, "applications/frontend/.npmrc": []byte(safeNPMConfig)}
	for _, pattern := range m.Workspaces {
		if strings.Contains(pattern, "\\") || path.IsAbs(pattern) || strings.Contains(pattern, "..") || strings.ContainsAny(pattern, "\x00\n\r") {
			return nil, fmt.Errorf("unsafe workspace pattern %q", pattern)
		}
		matches, e := filepath.Glob(filepath.Join(base, filepath.FromSlash(pattern), "package.json"))
		if e != nil {
			return nil, e
		}
		for _, f := range matches {
			b, e := readRunnerFile(f, reportLimit)
			if e != nil {
				return nil, e
			}
			var manifest map[string]any
			if json.Unmarshal(b, &manifest) != nil {
				return nil, fmt.Errorf("invalid manifest %s", f)
			}
			rel, e := filepath.Rel(snapshot, f)
			if e != nil {
				return nil, e
			}
			inputs[filepath.ToSlash(rel)] = b
		}
	}
	return inputs, nil
}
func DependencyInputDigest(snapshot string, p Profile) (string, error) {
	inputs, e := dependencyInputs(snapshot, p)
	if e != nil {
		return "", e
	}
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(h, "%d:%s:%d:", len(k), k, len(inputs[k]))
		h.Write(inputs[k])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func readRunnerFile(f string, limit int64) ([]byte, error) {
	info, e := os.Lstat(f)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("unsupported non-regular input %s", f)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("input exceeds limit: %s", f)
	}
	return os.ReadFile(f)
}
func writeInputDir(dir string, inputs map[string][]byte) error {
	for k, b := range inputs {
		f := filepath.Join(dir, filepath.FromSlash(k))
		if e := os.MkdirAll(filepath.Dir(f), 0700); e != nil {
			return e
		}
		if e := os.WriteFile(f, b, 0600); e != nil {
			return e
		}
	}
	return nil
}

// setupVolume executes only the fixed image utilities; source code is never evaluated here.
func setupVolume(ctx context.Context, image, work, out, fromVolume, source string) error {
	name := runnerID("copy")
	a := isolationArgs(name, image)
	a = append(a, "--network", "none", "--mount", "type=volume,src="+work+",dst=/work")
	script := ""
	if fromVolume != "" {
		a = append(a, "--mount", "type=volume,src="+fromVolume+",dst=/prepared,readonly")
		script += "cp -R /prepared/. /work/; "
	}
	if source != "" {
		a = append(a, "--mount", "type=bind,src="+source+",dst=/input,readonly")
		script += "cp -R /input/. /work/; "
	}
	if out != "" {
		a = append(a, "--mount", "type=volume,src="+out+",dst=/out")
		script += "chown -R 1000:1000 /out; "
	}
	script += "chmod -R u+rwX /work; chown -R 1000:1000 /work"
	// Fixed utility-only setup needs ownership/read access; repository execution drops every capability.
	for i := 0; i < len(a)-1; i++ {
		if a[i] == "--cap-drop" && a[i+1] == "ALL" {
			a = append(a[:i], a[i+2:]...)
			break
		}
	}
	a = append(a, "--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "DAC_OVERRIDE", "--entrypoint", "/bin/sh", image, "-eu", "-c", script)
	if _, e := dockerCommand(ctx, nil, a...); e != nil {
		return e
	}
	_, e := runOwnedContainer(ctx, name, nil)
	return e
}
func PrepareRuntime(ctx context.Context, o RunnerOptions) (Preparation, error) {
	var p Preparation
	ctx, c := context.WithTimeout(ctx, 600*time.Second)
	defer c()
	inputs, e := dependencyInputs(o.SnapshotDir, o.Profile)
	if e != nil {
		return p, e
	}
	actual, e := DependencyInputDigest(o.SnapshotDir, o.Profile)
	if e != nil {
		return p, e
	}
	if o.DependencyDigest == "" {
		o.DependencyDigest = actual
	} else if o.DependencyDigest != actual {
		return p, errors.New("dependency digest does not match captured inputs")
	}
	prior, loadErr := LoadPreparation(o.StateDir)
	if loadErr == nil {
		if _, e = dockerCommand(ctx, nil, "image", "inspect", prior.ImageID); e != nil {
			return p, fmt.Errorf("pinned prepared runtime unavailable: %w", e)
		}
		if PreparationCompatible(o, prior) == nil {
			if _, e = dockerCommand(ctx, nil, "volume", "inspect", prior.Volume); e != nil {
				return p, fmt.Errorf("pinned preparation volume unavailable; initialize fresh state or explicitly remove only its receipt before preparing again: %w", e)
			}
			return prior, nil
		}
	} else if !os.IsNotExist(loadErr) {
		return p, fmt.Errorf("invalid existing preparation receipt: %w", loadErr)
	}
	if e = os.MkdirAll(o.StateDir, 0700); e != nil {
		return p, e
	}
	tmp, e := os.MkdirTemp(o.StateDir, "prepare-inputs-")
	if e != nil {
		return p, e
	}
	defer os.RemoveAll(tmp)
	manifests := filepath.Join(tmp, "manifests")
	if e = writeInputDir(manifests, inputs); e != nil {
		return p, e
	}
	parts := []string{prior.BaseDigest, prior.Architecture}
	image := prior.ImageID
	if loadErr != nil {
		tag := "node:24.18.0-bookworm-slim"
		if _, e = dockerCommand(ctx, nil, "pull", tag); e != nil {
			return p, fmt.Errorf("required official Node runtime %s unavailable: %w", tag, e)
		}
		b, e := dockerCommand(ctx, nil, "image", "inspect", "--format", "{{index .RepoDigests 0}}|{{.Architecture}}", tag)
		if e != nil {
			return p, e
		}
		parts = strings.Split(strings.TrimSpace(string(b)), "|")
		if len(parts) != 2 || !strings.Contains(parts[0], "@sha256:") {
			return p, errors.New("Docker did not resolve immutable official node image digest")
		}
		build := filepath.Join(tmp, "build")
		if e = os.MkdirAll(build, 0700); e != nil {
			return p, e
		}
		if e = os.WriteFile(filepath.Join(build, "Dockerfile"), []byte(runnerDockerfile), 0600); e != nil {
			return p, e
		}
		if e = os.WriteFile(filepath.Join(build, "run-tests.sh"), []byte(runnerEntrypoint), 0600); e != nil {
			return p, e
		}
		iid := filepath.Join(tmp, "image-id")
		if _, e = dockerCommand(ctx, nil, "build", "--build-arg", "BASE="+parts[0], "--iidfile", iid, build); e != nil {
			return p, fmt.Errorf("pinned Node/npm runner build failed: %w", e)
		}
		id, e := os.ReadFile(iid)
		if e != nil {
			return p, e
		}
		image = strings.TrimSpace(string(id))
		if !strings.HasPrefix(image, "sha256:") {
			return p, errors.New("invalid runner image ID")
		}

	}
	volume := runnerID("dependencies")
	if _, e = dockerCommand(ctx, nil, "volume", "create", volume); e != nil {
		return p, e
	}
	keep := false
	defer func() {
		if !keep {
			cleanupVolume(volume)
		}
	}()
	if e = setupVolume(ctx, image, volume, "", "", manifests); e != nil {
		return p, e
	}
	name := runnerID("prepare")
	if _, e = dockerCommand(ctx, nil, prepareContainerArgs(name, image, volume)...); e != nil {
		return p, e
	}
	logs, e := runOwnedContainer(ctx, name, nil)
	_ = os.WriteFile(filepath.Join(o.StateDir, "prepare.log"), logs, 0600)
	if e != nil {
		return p, e
	}
	p = Preparation{SchemaVersion: 1, InputDigest: "", DependencyDigest: o.DependencyDigest, ProfileDigest: o.ProfileDigest, PolicyDigest: o.PolicyDigest, EvaluatorDigest: o.EvaluatorDigest, NPMConfigDigest: o.NPMConfigDigest, BaseDigest: parts[0], Architecture: parts[1], ImageID: image, DockerfileDigest: runnerHash([]byte(runnerDockerfile)), EntrypointDigest: runnerHash([]byte(runnerEntrypoint)), Volume: volume, PreparedAt: time.Now().UTC().Format(time.RFC3339)}
	p.InputDigest = preparationPinnedDigest(o, p)
	data, _ := json.MarshalIndent(p, "", "  ")
	if e = os.WriteFile(filepath.Join(o.StateDir, "preparation.json.tmp"), data, 0600); e != nil {
		return p, e
	}
	if e = os.Rename(filepath.Join(o.StateDir, "preparation.json.tmp"), filepath.Join(o.StateDir, "preparation.json")); e != nil {
		return p, e
	}
	keep = true
	return p, nil
}

func copyExecutionInput(snapshot, dest string) error {
	root := filepath.Join(snapshot, "applications/frontend")
	return filepath.WalkDir(root, func(f string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(snapshot, f)
		if e != nil {
			return e
		}
		base := d.Name()
		if d.IsDir() {
			switch base {
			case ".git", "node_modules", ".nuxt", ".output", "dist", "coverage", "reports":
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dest, rel), 0700)
		}
		if base == ".npmrc" || base == ".env" || strings.HasPrefix(base, ".env.") {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported execution input %s", rel)
		}
		b, e := readRunnerFile(f, reportLimit)
		if e != nil {
			return e
		}
		mode := os.FileMode(0600)
		if info.Mode()&0111 != 0 {
			mode = 0700
		}
		return os.WriteFile(filepath.Join(dest, rel), b, mode)
	})
}
func RunTests(ctx context.Context, o RunnerOptions, p Preparation) (TestEvidence, error) {
	result := TestEvidence{Status: "error", ReasonCode: "runner_error", Files: []string{}}
	if e := PreparationCompatible(o, p); e != nil {
		return TestEvidence{Status: "missing", ReasonCode: "preparation_required", Message: e.Error(), Files: []string{}}, nil
	}
	if !strings.HasPrefix(p.Volume, "preflight-dependencies-") || !strings.HasPrefix(p.ImageID, "sha256:") {
		return result, errors.New("invalid preparation runtime identity")
	}
	if _, e := dockerCommand(ctx, nil, "image", "inspect", p.ImageID); e != nil {
		return result, e
	}
	if _, e := dockerCommand(ctx, nil, "volume", "inspect", p.Volume); e != nil {
		return TestEvidence{Status: "missing", ReasonCode: "preparation_required", Message: "prepared dependency volume is unavailable", Files: []string{}}, nil
	}
	for _, f := range o.RequiredFiles {
		if path.IsAbs(f) || path.Clean(f) != f || strings.HasPrefix(f, "../") {
			return result, fmt.Errorf("unsafe required test path %q", f)
		}
		if _, e := readRunnerFile(filepath.Join(o.SnapshotDir, filepath.FromSlash(f)), reportLimit); e != nil {
			return TestEvidence{Status: "fail", ReasonCode: "required_test_file_missing", Message: "required baseline test file is absent: " + f, Files: []string{}}, nil
		}
	}
	b, e := readRunnerFile(filepath.Join(o.SnapshotDir, "applications/frontend/apps/web/package.json"), reportLimit)
	if e != nil {
		return result, e
	}
	var app struct {
		Scripts map[string]string `json:"scripts"`
	}
	if e = json.Unmarshal(b, &app); e != nil {
		return result, e
	}
	if app.Scripts["test"] != "vitest run" {
		return TestEvidence{Status: "fail", ReasonCode: "unapproved_test_command", Message: "candidate test script must remain vitest run; modified command was not executed", Files: []string{}}, nil
	}
	if e = os.MkdirAll(o.RunDir, 0700); e != nil {
		return result, e
	}
	source, e := os.MkdirTemp(o.RunDir, "execution-input-")
	if e != nil {
		return result, e
	}
	defer os.RemoveAll(source)
	if e = copyExecutionInput(o.SnapshotDir, source); e != nil {
		return result, e
	}
	if e = os.WriteFile(filepath.Join(source, "applications/frontend/.npmrc"), []byte(safeNPMConfig), 0600); e != nil {
		return result, e
	}
	ctx, c := context.WithTimeout(ctx, 300*time.Second)
	defer c()
	work, out := runnerID("work"), runnerID("output")
	for _, v := range []string{work, out} {
		if _, e = dockerCommand(ctx, nil, "volume", "create", v); e != nil {
			return result, e
		}
		defer cleanupVolume(v)
	}
	if e = setupVolume(ctx, p.ImageID, work, out, p.Volume, source); e != nil {
		return result, e
	}
	name := runnerID("tests")
	if _, e = dockerCommand(ctx, nil, testContainerArgs(name, p.ImageID, work, out)...); e != nil {
		return result, e
	}
	defer cleanupContainer(name)
	logs, runErr := dockerCommand(ctx, nil, "start", "--attach", name)
	logPath := filepath.Join(o.RunDir, "tests.log")
	if e = os.WriteFile(logPath, logs, 0600); e != nil {
		return result, e
	}
	if ctx.Err() != nil {
		return result, fmt.Errorf("isolated test timeout/cancellation; owned container removed: %w", ctx.Err())
	}
	code := 0
	if runErr != nil {
		var x *exec.ExitError
		if errors.As(runErr, &x) {
			code = x.ExitCode()
		} else {
			return result, runErr
		}
	}
	reports, e := collectContainerReports(ctx, name)
	if e != nil {
		return result, e
	}
	result = parseTestEvidence(reports["vitest.xml"], reports["vitest.json"], code, o.RequiredFiles)
	result.LogPath = logPath
	for name, b := range reports {
		f := filepath.Join(o.RunDir, name)
		if e = os.WriteFile(f, b, 0600); e != nil {
			return result, e
		}
		if name == "vitest.xml" {
			result.JUnitPath = f
		} else {
			result.JSONPath = f
		}
	}
	return result, nil
}
func collectContainerReports(ctx context.Context, name string) (map[string][]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", "cp", name+":/out/.", "-")
	out := &boundedOutput{limit: 2*reportLimit + (1 << 20)}
	errout := &boundedOutput{limit: logLimit}
	cmd.Stdout = out
	cmd.Stderr = errout
	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("collect fresh container reports: %w: %s", e, errout.buf.String())
	}
	if out.exceeded || errout.exceeded {
		return nil, errors.New("container reports exceeded bounded output limits")
	}
	reports := map[string][]byte{}
	r := tar.NewReader(bytes.NewReader(out.buf.Bytes()))
	for {
		h, e := r.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		n := strings.TrimPrefix(path.Clean(h.Name), "./")
		n = strings.TrimPrefix(n, "out/")
		if n != "vitest.xml" && n != "vitest.json" {
			return nil, fmt.Errorf("unexpected report file %q", h.Name)
		}
		if h.Typeflag != tar.TypeReg || h.Size > reportLimit || h.Size < 0 {
			return nil, errors.New("unsupported or oversized structured report")
		}
		if _, ok := reports[n]; ok {
			return nil, errors.New("duplicate structured report")
		}
		b, e := io.ReadAll(io.LimitReader(r, reportLimit+1))
		if e != nil {
			return nil, e
		}
		reports[n] = b
	}
	return reports, nil
}
func parseTestEvidence(junit, jsonReport []byte, exit int, required []string) TestEvidence {
	r := TestEvidence{Status: "pass", ReasonCode: "fresh_test_evidence", Message: "fresh JUnit and JSON evidence cover all required files", ExitCode: exit, Files: []string{}}
	bad := func(status, reason, msg string) TestEvidence {
		r.Status = status
		r.ReasonCode = reason
		r.Message = msg
		return r
	}
	if len(junit) == 0 {
		if exit != 0 {
			return bad("fail", "test_process_failed", "approved offline command returned nonzero before producing JUnit")
		}
		return bad("missing", "junit_missing", "test command returned zero without fresh JUnit evidence")
	}
	if len(junit) > reportLimit || len(jsonReport) > reportLimit {
		return bad("error", "report_too_large", "structured report exceeds 20 MiB")
	}
	d := xml.NewDecoder(bytes.NewReader(junit))
	depth := 0
	root := ""
	testDepth := 0
	for {
		tok, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return bad("error", "junit_malformed", "invalid JUnit XML: "+e.Error())
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if depth == 0 && root != "" {
				return bad("error", "junit_malformed", "multiple XML roots")
			}
			depth++
			if root == "" {
				root = v.Name.Local
				if root != "testsuites" && root != "testsuite" {
					return bad("error", "junit_malformed", "expected JUnit testsuite(s) root")
				}
			}
			switch v.Name.Local {
			case "testcase":
				if testDepth != 0 {
					return bad("error", "junit_malformed", "nested testcase")
				}
				r.Tests++
				testDepth = depth
			case "failure":
				if testDepth > 0 {
					r.Failures++
				}
			case "error":
				r.Errors++
			case "skipped":
				if testDepth > 0 {
					r.Skipped++
				}
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(v)) != "" {
				return bad("error", "junit_malformed", "text outside XML root")
			}
		case xml.EndElement:
			if testDepth == depth {
				testDepth = 0
			}
			depth--
		}
	}
	if root == "" || depth != 0 {
		return bad("error", "junit_malformed", "incomplete JUnit")
	}
	if r.Failures > 0 || r.Errors > 0 || r.Skipped > 0 || exit != 0 {
		return bad("fail", "tests_failed", "test process failed or JUnit contains failures, errors or skipped cases")
	}
	if r.Tests == 0 {
		return bad("fail", "zero_tests", "JUnit contains no executed testcases")
	}
	if len(jsonReport) == 0 {
		return bad("missing", "vitest_json_missing", "fresh Vitest JSON is required to establish baseline file coverage")
	}
	var j struct {
		TestResults []struct {
			Name             string `json:"name"`
			Status           string `json:"status"`
			AssertionResults []struct {
				Status string `json:"status"`
			} `json:"assertionResults"`
		} `json:"testResults"`
	}
	dec := json.NewDecoder(bytes.NewReader(jsonReport))
	if e := dec.Decode(&j); e != nil {
		return bad("error", "vitest_json_malformed", e.Error())
	}
	if dec.Decode(new(any)) != io.EOF {
		return bad("error", "vitest_json_malformed", "trailing JSON data")
	}
	count := 0
	seen := map[string]bool{}
	for _, f := range j.TestResults {
		n := strings.TrimPrefix(filepath.ToSlash(f.Name), "/work/")
		if seen[n] {
			return bad("error", "vitest_json_malformed", "duplicate test file")
		}
		seen[n] = true
		if len(f.AssertionResults) == 0 {
			return bad("fail", "zero_file_tests", "reported test file contains no executed assertions: "+n)
		}
		r.Files = append(r.Files, n)
		if f.Status != "" && f.Status != "passed" {
			return bad("fail", "tests_failed", "Vitest JSON test file was not passed")
		}
		for _, a := range f.AssertionResults {
			count++
			if a.Status != "passed" {
				return bad("fail", "tests_failed", "Vitest JSON assertion was not passed")
			}
		}
	}
	sort.Strings(r.Files)
	for _, f := range required {
		if !seen[f] {
			return bad("fail", "required_test_file_missing", "report does not represent required baseline test file: "+f)
		}
	}
	if count != r.Tests {
		return bad("error", "test_report_count_mismatch", "JUnit and Vitest JSON testcase counts disagree")
	}
	return r
}
