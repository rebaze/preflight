package preflight

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func snapshotGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func snapshotWrite(t *testing.T, dir, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, path), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func snapshotFixture(t *testing.T) (string, string, Profile) {
	t.Helper()
	dir := t.TempDir()
	snapshotGit(t, dir, "init", "-b", "main")
	snapshotGit(t, dir, "config", "user.email", "synthetic@example.invalid")
	snapshotGit(t, dir, "config", "user.name", "Synthetic")
	snapshotWrite(t, dir, "applications/frontend/base.txt", "base")
	snapshotWrite(t, dir, ".github/workflows/ci.yml", "jobs: {}\n")
	snapshotWrite(t, dir, ".gitignore", "applications/frontend/ignored.txt\n")
	snapshotGit(t, dir, "add", ".")
	snapshotGit(t, dir, "commit", "-m", "synthetic baseline")
	return dir, snapshotGit(t, dir, "rev-parse", "HEAD"), Profile{SourcePrefixes: []string{"applications/frontend/"}, SourceFiles: []string{".github/workflows/ci.yml"}}
}

func takeSnapshot(t *testing.T, repo, baseline, base, dest string, p Profile) Snapshot {
	t.Helper()
	s, err := CaptureSnapshot(context.Background(), repo, baseline, base, "profile", "policy", dest, p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCommittedBranchChangeIncluded(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	snapshotGit(t, repo, "checkout", "-b", "feature")
	path := "applications/frontend/feature with spaces.txt"
	snapshotWrite(t, repo, path, "feature")
	snapshotGit(t, repo, "add", ".")
	snapshotGit(t, repo, "commit", "-m", "feature")
	if got := snapshotGit(t, repo, "status", "--porcelain"); got != "" {
		t.Fatalf("dirty fixture: %s", got)
	}
	s := takeSnapshot(t, repo, baseline, "main", t.TempDir(), p)
	if !slices.Contains(s.Subject.ChangedPaths, path) {
		t.Fatalf("committed change omitted: %+v", s.Subject)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir, path))
	if err != nil || string(b) != "feature" {
		t.Fatalf("capture %q %v", b, err)
	}
}

func TestWorktreeOverridesIndex(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	path := "applications/frontend/base.txt"
	snapshotWrite(t, repo, path, "index")
	snapshotGit(t, repo, "add", path)
	snapshotWrite(t, repo, path, "worktree")
	s := takeSnapshot(t, repo, baseline, "", t.TempDir(), p)
	b, err := os.ReadFile(filepath.Join(s.Dir, path))
	if err != nil || string(b) != "worktree" {
		t.Fatalf("got %q %v", b, err)
	}
}

func TestRelevantUntrackedIncluded(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	snapshotWrite(t, repo, "applications/frontend/new\nfile.ts", "new")
	snapshotWrite(t, repo, "applications/frontend/ignored.txt", "ignored")
	snapshotWrite(t, repo, "backend/changes.txt", "not selected")
	s := takeSnapshot(t, repo, baseline, "", t.TempDir(), p)
	if _, err := os.Stat(filepath.Join(s.Dir, "applications/frontend/new\nfile.ts")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "applications/frontend/ignored.txt")); !os.IsNotExist(err) {
		t.Fatal("ignored input copied")
	}
	if !slices.Contains(s.Subject.OutOfScopePaths, "backend/changes.txt") {
		t.Fatal("scope change omitted")
	}
}

func TestSecretExcluded(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	for _, path := range []string{".env", ".env.local", "private.pem", "credentials.json", "id_rsa", "node_modules/fake.txt", "coverage/file.txt"} {
		full := "applications/frontend/" + path
		// Dangling links make accidental secret reads/copies fail, while exclusions
		// can be decided from path names without accessing their contents.
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, full)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/does-not-exist", filepath.Join(repo, full)); err != nil {
			t.Fatal(err)
		}
	}
	s := takeSnapshot(t, repo, baseline, "", t.TempDir(), p)
	for _, f := range s.Files {
		if strings.Contains(f.Path, ".env") || strings.Contains(f.Path, "private.pem") {
			t.Fatalf("secret included: %s", f.Path)
		}
	}
}

func TestSymlinkRejected(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "parent"}[parent], func(t *testing.T) {
			repo, baseline, p := snapshotFixture(t)
			if parent {
				if err := os.RemoveAll(filepath.Join(repo, "applications/frontend")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), filepath.Join(repo, "applications/frontend")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Symlink("base.txt", filepath.Join(repo, "applications/frontend/link.txt")); err != nil {
					t.Fatal(err)
				}
			}
			_, err := CaptureSnapshot(context.Background(), repo, baseline, "", "p", "r", t.TempDir(), p)
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("want symlink error, got %v", err)
			}
		})
	}
}

func TestSnapshotMissingAndModesAffectIdentity(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	a := takeSnapshot(t, repo, baseline, "", "", p)
	if err := os.Chmod(filepath.Join(repo, "applications/frontend/base.txt"), 0700); err != nil {
		t.Fatal(err)
	}
	b := takeSnapshot(t, repo, baseline, "", "", p)
	if a.Subject.SnapshotDigest == b.Subject.SnapshotDigest {
		t.Fatal("mode change omitted")
	}
	if err := os.Remove(filepath.Join(repo, "applications/frontend/base.txt")); err != nil {
		t.Fatal(err)
	}
	c := takeSnapshot(t, repo, baseline, "", "", p)
	if c.Subject.SnapshotDigest == b.Subject.SnapshotDigest {
		t.Fatal("deletion omitted")
	}
	found := false
	for _, f := range c.Files {
		if f.Path == "applications/frontend/base.txt" && f.Kind == "missing" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing marker omitted")
	}
}

func TestSnapshotNPMConfigNeverCopied(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	path := "applications/frontend/.npmrc"
	snapshotWrite(t, repo, path, "save-exact=true\n//registry.npmjs.org/:_authToken=synthetic-secret\n")
	a := takeSnapshot(t, repo, baseline, "", t.TempDir(), p)
	if _, err := os.Stat(filepath.Join(a.Dir, path)); !os.IsNotExist(err) {
		t.Fatal("source npmrc copied")
	}
	if len(a.UnsupportedNPMKeys) != 1 || strings.Contains(strings.Join(a.UnsupportedNPMKeys, ""), "synthetic-secret") {
		t.Fatalf("unsafe review: %v", a.UnsupportedNPMKeys)
	}
	snapshotWrite(t, repo, path, "save-exact=true\n//registry.npmjs.org/:_authToken=other-secret\n")
	b := takeSnapshot(t, repo, baseline, "", "", p)
	if a.Subject.SnapshotDigest == b.Subject.SnapshotDigest {
		t.Fatal("npmrc change omitted")
	}
}

func TestSnapshotBaselineFileRead(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	files, err := BaselineFiles(context.Background(), repo, baseline, p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, ".github/workflows/ci.yml") {
		t.Fatal(files)
	}
	b, err := ReadBaselineFile(context.Background(), repo, baseline, ".github/workflows/ci.yml")
	if err != nil || string(b) != "jobs: {}\n" {
		t.Fatalf("%q %v", b, err)
	}
}

func TestSnapshotDeletedCommittedAdditionHasMissingMarker(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	name := "applications/frontend/added.ts"
	snapshotWrite(t, repo, name, "feature")
	snapshotGit(t, repo, "add", name)
	snapshotGit(t, repo, "commit", "-m", "add feature")
	snapshotGit(t, repo, "rm", name)
	s := takeSnapshot(t, repo, baseline, "", "", p)
	for _, f := range s.Files {
		if f.Path == name && f.Kind == "missing" {
			return
		}
	}
	t.Fatal("staged deletion of committed addition lacks missing marker")
}

func TestSnapshotIgnoresInheritedGitRepository(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	other := t.TempDir()
	t.Setenv("GIT_DIR", other)
	t.Setenv("GIT_WORK_TREE", other)
	s := takeSnapshot(t, repo, baseline, "", "", p)
	expected, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if s.Subject.RepoRoot != expected {
		t.Fatalf("Git environment changed target: %s", s.Subject.RepoRoot)
	}
}

func TestSnapshotMalformedNPMConfigDoesNotExposeValue(t *testing.T) {
	keys := snapshotNPMReview([]byte("synthetic-secret-without-key\n"))
	if len(keys) != 1 || strings.Contains(keys[0], "synthetic-secret") {
		t.Fatalf("malformed config exposes bytes: %v", keys)
	}
}

func TestSnapshotDestinationCannotAliasSource(t *testing.T) {
	repo, baseline, p := snapshotFixture(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	_, err := CaptureSnapshot(context.Background(), repo, baseline, "", "p", "r", filepath.Join(alias, "new-state"), p)
	if err == nil {
		t.Fatal("snapshot wrote through symlink into source")
	}
	if _, err := os.Stat(filepath.Join(repo, "new-state")); !os.IsNotExist(err) {
		t.Fatal("source was changed")
	}
}
