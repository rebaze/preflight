package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Public CLI, no Conftest/Docker/project dependencies. Tripwires show source
// declarations cannot become host commands, in differently laid-out repos.
func TestInspectCLIIndependentLayouts(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "preflight")
	cmd := exec.Command("go", "build", "-o", bin, "../cmd/preflight")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	for _, layout := range []string{"go", "javascript", "empty"} {
		t.Run(layout, func(t *testing.T) {
			repo := t.TempDir()
			git := func(args ...string) {
				t.Helper()
				c := exec.Command("git", append([]string{"-C", repo}, args...)...)
				c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
				if b, e := c.CombinedOutput(); e != nil {
					t.Fatalf("git: %v %s", e, b)
				}
			}
			git("init", "-b", "main")
			git("config", "user.name", "Synthetic")
			git("config", "user.email", "synthetic@example.invalid")
			files := map[string]string{}
			if layout != "empty" {
				files["CONTRIBUTING.md"] = "# Changes\n\nPreserve existing response fields.\n"
			}
			if layout == "go" {
				files["go.mod"] = "module example.invalid/service\n\ngo 1.27.1\n"
				files["service.go"] = "package service\n//go:generate touch SCRIPT_EXECUTED\n"
			}
			if layout == "javascript" {
				files["clients/browser/package.json"] = `{"scripts":{"test":"touch SCRIPT_EXECUTED","postinstall":"touch SCRIPT_EXECUTED"}}`
				files["clients/browser/index.js"] = "export const label = 'kept';\n"
			}
			for p, s := range files {
				p = filepath.Join(repo, p)
				if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(p, []byte(s), 0600); e != nil {
					t.Fatal(e)
				}
			}
			git("add", ".")
			git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "fixture")
			c := exec.Command(bin, "inspect", "--format", "json")
			c.Dir = repo
			b, e := c.Output()
			if e != nil {
				t.Fatalf("inspect: %v %s", e, b)
			}
			var d struct {
				Schema  string
				Sources []struct{ Path, Content string }
				Subject struct{ Dirty, Current bool }
			}
			if e = json.Unmarshal(b, &d); e != nil {
				t.Fatal(e)
			}
			if d.Schema != "preflight.discovery/v1" || !d.Subject.Current || d.Subject.Dirty {
				t.Fatalf("%s", b)
			}
			if layout != "empty" {
				found := false
				for _, s := range d.Sources {
					if s.Path == "CONTRIBUTING.md" {
						found = true
					}
				}
				if !found {
					t.Fatal("no relevant sourced expectation")
				}
			}
			for p, want := range files {
				got, e := os.ReadFile(filepath.Join(repo, p))
				if e != nil || string(got) != want {
					t.Fatal("source changed")
				}
			}
			if _, e := os.Stat(filepath.Join(repo, "SCRIPT_EXECUTED")); !os.IsNotExist(e) {
				t.Fatal("script executed")
			}
		})
	}
}
