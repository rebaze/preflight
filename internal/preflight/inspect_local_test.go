package preflight

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectLocalTracksLayersAndPreservesCheckout(t *testing.T) {
	repo, head, _ := snapshotFixture(t)
	snapshotWrite(t, repo, "CONTRIBUTING.md", "Preserve response fields.\nRun contract tests.\n")
	snapshotGit(t, repo, "add", "CONTRIBUTING.md")
	snapshotWrite(t, repo, "applications/frontend/base.txt", "changed")
	snapshotWrite(t, repo, "packages/api/package.json", `{"scripts":{"test":"touch NEVER"}}`)
	before := snapshotGit(t, repo, "status", "--porcelain=v1", "--untracked-files=all")
	d := InspectLocal(context.Background(), repo, "main")
	if d.ExitCode != 0 || d.Subject.Head != head || !d.Subject.Dirty || !d.Subject.Current {
		t.Fatalf("unexpected discovery: %+v", d)
	}
	for _, want := range []struct{ path, layer string }{{"CONTRIBUTING.md", "staged"}, {"applications/frontend/base.txt", "unstaged"}, {"packages/api/package.json", "untracked"}} {
		found := false
		for _, c := range d.Changes {
			if c.Path == want.path && c.Layer == want.layer {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s %s: %+v", want.layer, want.path, d.Changes)
		}
	}
	found := false
	for _, s := range d.Sources {
		if s.Path == "CONTRIBUTING.md" {
			found = s.StartLine == 1 && s.EndLine == 2 && s.Digest != "" && strings.Contains(s.Content, "Preserve response fields")
		}
	}
	if !found {
		t.Fatalf("missing sourced instruction: %+v", d.Sources)
	}
	if after := snapshotGit(t, repo, "status", "--porcelain=v1", "--untracked-files=all"); before != after {
		t.Fatalf("checkout changed: %q -> %q", before, after)
	}
	if _, err := os.Stat(filepath.Join(repo, "NEVER")); !os.IsNotExist(err) {
		t.Fatal("project script executed")
	}
}

func TestInspectLocalNeverExecutesGitConfiguredCode(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	marker := filepath.Join(t.TempDir(), "executed")
	script := "touch '" + marker + "'; cat"
	snapshotGit(t, repo, "config", "filter.hostile.clean", script)
	snapshotGit(t, repo, "config", "filter.hostile.smudge", script)
	snapshotGit(t, repo, "config", "diff.external", script)
	snapshotGit(t, repo, "config", "core.fsmonitor", script)
	snapshotGit(t, repo, "config", "core.hooksPath", filepath.Join(repo, "hooks"))
	snapshotWrite(t, repo, "hooks/post-index-change", "#!/bin/sh\n"+script+"\n")
	if err := os.Chmod(filepath.Join(repo, "hooks/post-index-change"), 0700); err != nil {
		t.Fatal(err)
	}
	snapshotWrite(t, repo, ".gitattributes", "*.txt filter=hostile diff=hostile\n")
	snapshotWrite(t, repo, "applications/frontend/base.txt", "changed")
	d := InspectLocal(context.Background(), repo, "")
	if d.Subject.Head == "" {
		t.Fatalf("missing useful facts: %+v", d)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("Git or repository code executed")
	}
}

func TestInspectLocalExcludesPrivateDataAndSymlinks(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	snapshotWrite(t, repo, ".env", "PRIVATE_SECRET")
	snapshotWrite(t, repo, "node_modules/example/package.json", "PRIVATE_SECRET")
	snapshotWrite(t, repo, ".npmrc", "PRIVATE_SECRET")
	snapshotWrite(t, repo, "secret-notes/AGENTS.md", "PRIVATE_SECRET")
	outside := t.TempDir()
	snapshotWrite(t, outside, "AGENTS.md", "PRIVATE_SECRET")
	if err := os.Symlink(filepath.Join(outside, "AGENTS.md"), filepath.Join(repo, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	d := InspectLocal(context.Background(), repo, "")
	if d.ExitCode != 2 {
		t.Fatalf("symlink must leave partial coverage: %+v", d)
	}
	for _, s := range d.Sources {
		if strings.Contains(s.Content, "PRIVATE_SECRET") {
			t.Fatalf("leaked private content: %s", s.Path)
		}
	}
}

func TestInspectLocalEmptyMissingBaseAndLinkedWorktree(t *testing.T) {
	empty := t.TempDir()
	snapshotWrite(t, empty, "AGENTS.md", "Do not remove response fields.\n")
	d := InspectLocal(context.Background(), empty, "")
	if d.ExitCode != 2 || len(d.Sources) != 1 {
		t.Fatalf("non-Git must retain useful partial sources: %+v", d)
	}
	repo, _, _ := snapshotFixture(t)
	d = InspectLocal(context.Background(), repo, "missing")
	if d.ExitCode != 2 || d.Subject.Head == "" {
		t.Fatalf("missing base lost facts: %+v", d)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	snapshotGit(t, repo, "worktree", "add", "--detach", linked, "HEAD")
	a, b := InspectLocal(context.Background(), repo, ""), InspectLocal(context.Background(), linked, "")
	if a.Subject.RepositoryID != b.Subject.RepositoryID || a.Subject.WorktreeID == b.Subject.WorktreeID || b.ExitCode != 0 {
		t.Fatalf("linked worktree identities: %+v %+v", a.Subject, b.Subject)
	}
}

func TestInspectLocalSourceLimitsAndRawInputFreshness(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	snapshotWrite(t, repo, "AGENTS.md", strings.Repeat("x", 65537))
	a := InspectLocal(context.Background(), repo, "")
	if a.ExitCode != 2 {
		t.Fatalf("oversized source must leave partial result: %+v", a)
	}
	for _, s := range a.Sources {
		if s.Path == "AGENTS.md" {
			t.Fatal("oversized source was ingested")
		}
	}
	snapshotWrite(t, repo, "applications/frontend/base.txt", "changed")
	b := InspectLocal(context.Background(), repo, "")
	if a.Subject.InputDigest == b.Subject.InputDigest {
		t.Fatal("source edit did not invalidate input identity")
	}
}

func TestInspectLocalExcludedInputLeavesExplicitGap(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	snapshotWrite(t, repo, ".env", "secret")
	d := InspectLocal(context.Background(), repo, "")
	if d.ExitCode != 2 {
		t.Fatalf("excluded unknown bytes cannot claim complete freshness: %+v", d)
	}
	for _, input := range d.Inputs {
		if input.Path == ".env" && input.Kind != "excluded" {
			t.Fatalf("private bytes hashed or disclosed: %+v", input)
		}
	}
}

func TestInspectLocalCommittedChangesDeletionAndIndexPreservation(t *testing.T) {
	repo, baseline, _ := snapshotFixture(t)
	snapshotWrite(t, repo, "go.mod", "module example.invalid/service\n\ngo 1.27.1\n")
	snapshotGit(t, repo, "add", "go.mod")
	snapshotGit(t, repo, "commit", "-m", "add module")
	if err := os.Remove(filepath.Join(repo, "applications/frontend/base.txt")); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(repo, ".git", "index")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	d := InspectLocal(context.Background(), repo, baseline)
	if d.ExitCode != 0 {
		t.Fatalf("unexpected partial: %+v", d)
	}
	foundCommit, foundDelete := false, false
	for _, c := range d.Changes {
		if c.Path == "go.mod" && c.Layer == "committed" && c.Status == "added" {
			foundCommit = true
		}
		if c.Path == "applications/frontend/base.txt" && c.Layer == "unstaged" && c.Status == "deleted" {
			foundDelete = true
		}
	}
	if !foundCommit || !foundDelete {
		t.Fatalf("lost change layers: %+v", d.Changes)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("discovery mutated Git index")
	}
	if err := ValidateDiscovery(d); err != nil {
		t.Fatalf("collector produced invalid model: %v", err)
	}
}

func TestInspectLocalSHA256RepositoryAndIgnoredSources(t *testing.T) {
	repo := t.TempDir()
	snapshotGit(t, repo, "init", "--object-format=sha256", "-b", "main")
	snapshotGit(t, repo, "config", "user.name", "Synthetic")
	snapshotGit(t, repo, "config", "user.email", "synthetic@example.invalid")
	snapshotWrite(t, repo, "go.mod", "module example.invalid/service\n")
	snapshotWrite(t, repo, ".gitignore", "ignored/\n")
	snapshotGit(t, repo, "add", ".")
	snapshotGit(t, repo, "commit", "-m", "baseline")
	snapshotWrite(t, repo, "ignored/AGENTS.md", "PRIVATE_IGNORED")
	d := InspectLocal(context.Background(), repo, "")
	if d.Subject.Dirty || d.ExitCode != 0 {
		t.Fatalf("SHA256 clean input mismatched: %+v", d)
	}
	for _, s := range d.Sources {
		if strings.Contains(s.Content, "PRIVATE_IGNORED") {
			t.Fatal("ignored source included")
		}
	}
}

func TestInspectExcludedTrackedWorktreeIsUnknown(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	snapshotWrite(t, repo, "private.txt", "do not disclose")
	snapshotGit(t, repo, "add", "private.txt")
	snapshotGit(t, repo, "commit", "-m", "private fixture")
	snapshotWrite(t, repo, "private.txt", "changed private content")
	d := InspectLocal(context.Background(), repo, "")
	if d.Subject.Current {
		t.Fatal("excluded tracked bytes cannot establish current worktree identity")
	}
	found := false
	for _, c := range d.Coverage {
		if c.Collector == "worktree_cleanliness" && c.Status == "partial" {
			found = true
		}
	}
	if !found {
		t.Fatal("worktree cleanliness uncertainty not explicit")
	}
	if err := os.Remove(filepath.Join(repo, "private.txt")); err != nil {
		t.Fatal(err)
	}
	d = InspectLocal(context.Background(), repo, "")
	if !d.Subject.Dirty {
		t.Fatal("known deletion of excluded tracked file lost")
	}
	for _, s := range d.Sources {
		if s.Path == "private.txt" {
			t.Fatal("private contents disclosed")
		}
	}
}
func TestInspectUnsupportedTrackedWorktreeIsUnknown(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	p := filepath.Join(repo, "applications/frontend/base.txt")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", p); err != nil {
		t.Fatal(err)
	}
	d := InspectLocal(context.Background(), repo, "")
	if d.Subject.Current {
		t.Fatal("unsupported source cannot establish current worktree identity")
	}
	found := false
	for _, c := range d.Coverage {
		if c.Collector == "worktree_cleanliness" && c.Status == "partial" {
			found = true
		}
	}
	if !found {
		t.Fatal("unsupported tracked state not explicit")
	}
}

func TestInspectLocalComparisonTipChangesIdentity(t *testing.T) {
	repo, baseline, _ := snapshotFixture(t)
	snapshotGit(t, repo, "switch", "-c", "topic")
	snapshotWrite(t, repo, "topic.txt", "topic")
	snapshotGit(t, repo, "add", "topic.txt")
	snapshotGit(t, repo, "commit", "-m", "topic")
	before := InspectLocal(context.Background(), repo, "main")
	// Advance the other branch with the baseline tree, without checking it out.
	// Checkout can change raw permission bits and accidentally mask this bug.
	tree := snapshotGit(t, repo, "rev-parse", baseline+"^{tree}")
	next := snapshotGit(t, repo, "commit-tree", strings.TrimSpace(tree), "-p", baseline, "-m", "advance comparison")
	snapshotGit(t, repo, "update-ref", "refs/heads/main", strings.TrimSpace(next))
	after := InspectLocal(context.Background(), repo, "main")
	if before.Subject.MergeBase != after.Subject.MergeBase || before.Subject.BaseCommit == after.Subject.BaseCommit || before.Subject.Head != after.Subject.Head || before.Subject.IndexDigest != after.Subject.IndexDigest {
		t.Fatal("invalid fixture")
	}
	if before.Subject.InputDigest == after.Subject.InputDigest {
		t.Fatal("comparison tip change invisible to freshness")
	}
}
