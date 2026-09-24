package preflight

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveDiscoveryExclusivePrivateAndOutsideCheckout(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	d := InspectLocal(context.Background(), repo, "")
	outside := t.TempDir()
	saved := filepath.Join(outside, "before.json")
	if err := SaveDiscovery(saved, d); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(saved)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("observation must be private")
	}
	loaded, err := LoadDiscovery(saved)
	if err != nil || loaded.Subject.InputDigest != d.Subject.InputDigest {
		t.Fatalf("round trip: %+v %v", loaded, err)
	}
	if err := SaveDiscovery(saved, d); err == nil {
		t.Fatal("previous observation overwritten")
	}
	if err := SaveDiscovery(filepath.Join(repo, "inside.json"), d); err == nil {
		t.Fatal("observation saved inside inspected source")
	}
	alias := filepath.Join(outside, "alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	if err := SaveDiscovery(filepath.Join(alias, "inside.json"), d); err == nil {
		t.Fatal("observation saved through checkout symlink")
	}
	if err := os.Symlink(saved, filepath.Join(outside, "link.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDiscovery(filepath.Join(outside, "link.json")); err == nil {
		t.Fatal("loaded symlink as authenticated observation file")
	}
}

func TestSaveDiscoveryRejectsLinkedGitMetadataAndOtherCheckout(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	linked := filepath.Join(t.TempDir(), "linked")
	snapshotGit(t, repo, "worktree", "add", "--detach", linked, "HEAD")
	d := InspectLocal(context.Background(), linked, "")
	if err := SaveDiscovery(filepath.Join(repo, ".git", "saved.json"), d); err == nil {
		t.Fatal("observation saved in common Git metadata")
	}
	other, _, _ := snapshotFixture(t)
	if err := SaveDiscovery(filepath.Join(other, "saved.json"), d); err == nil {
		t.Fatal("private observation saved in another source checkout")
	}
}

func TestLoadDiscoveryRejectsMalformedAndOversizedEvidence(t *testing.T) {
	for _, body := range []string{`{"schema":"preflight.discovery/v1","schema":"preflight.discovery/v1"}`, strings.Repeat("x", (16<<20)+1)} {
		p := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadDiscovery(p); err == nil {
			t.Fatal("invalid saved observation accepted")
		}
	}
}

func TestCompareDiscoveryInputsRestorationAndCompatibility(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	before := InspectLocal(context.Background(), repo, "")
	snapshotWrite(t, repo, "applications/frontend/base.txt", "changed")
	after := InspectLocal(context.Background(), repo, "")
	c := CompareDiscovery(before, after)
	if !c.Compatible || !c.PreviousEvidenceStale || len(c.InputChanges) != 1 || c.InputChanges[0].Path != "applications/frontend/base.txt" {
		t.Fatalf("lost input edit: %+v", c)
	}
	snapshotWrite(t, repo, "applications/frontend/base.txt", "base")
	restored := InspectLocal(context.Background(), repo, "")
	if got := CompareDiscovery(before, restored); !got.Compatible || got.PreviousEvidenceStale || len(got.InputChanges) != 0 {
		t.Fatalf("restoration did not recover exact input identity: %+v", got)
	}
	restored.Subject.RepositoryID = "other"
	if got := CompareDiscovery(before, restored); got.Compatible || len(got.InputChanges) != 0 {
		t.Fatalf("incompatible repository compared: %+v", got)
	}
	for _, mutate := range []func(*Discovery){func(d *Discovery) { d.Subject.WorktreeID = "other" }, func(d *Discovery) { d.Subject.Branch = "other" }, func(d *Discovery) { d.Subject.BaseRef = "other" }} {
		copy := before
		mutate(&copy)
		if CompareDiscovery(before, copy).Compatible {
			t.Fatal("incompatible subject compared")
		}
	}
}

func TestCompareDiscoveryPartialCollectionDoesNotResolveKnownFinding(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	before := InspectLocal(context.Background(), repo, "")
	before.Diagnostics = append(before.Diagnostics, DiscoveryDiagnostic{Code: "source_limit", Severity: "warning", Path: "AGENTS.md", Message: "Source unavailable"})
	before.Coverage = append(before.Coverage, DiscoveryCoverage{Collector: "local_sources", Status: "partial", Detail: "Source unavailable"})
	FinalizeDiscovery(&before)
	after := InspectLocal(context.Background(), repo, "")
	after.Coverage = append(after.Coverage, DiscoveryCoverage{Collector: "local_sources", Status: "partial", Detail: "Access lost"})
	FinalizeDiscovery(&after)
	if c := CompareDiscovery(before, after); len(c.ResolvedFindings) != 0 {
		t.Fatalf("lost coverage manufactured resolution: %+v", c)
	}
}

func compareGitHubFixture(observed, revision, conclusion string, app int64) *DiscoveryGitHub {
	g := newDiscoveryGitHub()
	g.Repository = "example/service"
	g.HeadRepository = g.Repository
	g.TargetBranch = "main"
	g.Head = revision
	g.EvidenceRevision = revision
	g.EvidenceSubject = "head"
	g.LocalCoverage = "clean_head_only"
	g.ObservedAt = observed
	g.Requirements = []DiscoveryGitHubRequirement{{ID: "required:contract", RuleID: "required", Context: "contract", AppID: 7, Producer: "app", Source: "https://api.github.com/repos/example/service/rules/branches/main", ObservedAt: observed}}
	g.Rules = []DiscoveryGitHubRule{{ID: "required", Type: "required_status_checks", Source: g.Requirements[0].Source, Supported: true, ObservedAt: observed}}
	g.Results = []DiscoveryGitHubResult{{ID: "check:example/service:1", Kind: "check_run", Name: "contract", Revision: revision, Repository: g.Repository, AppID: app, AppSlug: "actions", State: "completed", Conclusion: conclusion, CompletedAt: observed, UpdatedAt: observed, Source: "https://api.github.com/repos/example/service/check-runs/1", ObservedAt: observed}}
	for _, resource := range []string{"rulesets", "branch_protection", "head.check_runs", "head.statuses"} {
		g.Coverage = append(g.Coverage, DiscoveryGitHubCoverage{Resource: resource, Status: "complete", Detail: "Observed", Source: g.Requirements[0].Source, ObservedAt: observed})
	}
	g.matchRequirements()
	return g
}

func TestCompareDiscoveryObservedTransitionsRequireRevisionAndProducer(t *testing.T) {
	previous, current := compareGitHubFixture("2026-09-24T10:00:00Z", "aaa", "success", 7), compareGitHubFixture("2026-09-24T11:00:00Z", "bbb", "failure", 7)
	c := DiscoveryComparison{Compatible: true, Evidence: []DiscoveryEvidenceTransition{}}
	inspectCompareGitHub(previous, current, &c)
	if len(c.Evidence) != 1 || !c.Evidence[0].Comparable || c.Evidence[0].LastObservedPassing != previous.ObservedAt || c.Evidence[0].FirstObservedFailing != current.ObservedAt {
		t.Fatalf("missing supported observation interval: %+v", c)
	}
	for _, mutate := range []func(*DiscoveryGitHub){func(g *DiscoveryGitHub) { g.Results[0].AppID = 8 }, func(g *DiscoveryGitHub) { g.Results[0].Revision = "other" }, func(g *DiscoveryGitHub) { g.EvidenceSubject = "merge_candidate" }, func(g *DiscoveryGitHub) { g.Results[0].Repository = "other/repository" }} {
		g := compareGitHubFixture("2026-09-24T11:00:00Z", "bbb", "failure", 7)
		mutate(g)
		c := DiscoveryComparison{Compatible: true, Evidence: []DiscoveryEvidenceTransition{}}
		inspectCompareGitHub(previous, g, &c)
		for _, e := range c.Evidence {
			if e.LastObservedPassing != "" || e.FirstObservedFailing != "" {
				t.Fatalf("incomparable evidence manufactured interval: %+v", e)
			}
		}
	}
}

func TestCompareDiscoverySkippedCheckIsNotObservedPassing(t *testing.T) {
	previous, current := compareGitHubFixture("2026-09-24T10:00:00Z", "aaa", "skipped", 7), compareGitHubFixture("2026-09-24T11:00:00Z", "bbb", "failure", 7)
	c := DiscoveryComparison{Compatible: true, Evidence: []DiscoveryEvidenceTransition{}}
	inspectCompareGitHub(previous, current, &c)
	for _, e := range c.Evidence {
		if e.LastObservedPassing != "" || e.FirstObservedFailing != "" {
			t.Fatalf("skipped check manufactured passing observation: %+v", e)
		}
	}
}

func TestCompareDiscoveryMissingRemoteAccessPreservesFailureAndRequirements(t *testing.T) {
	previous := compareGitHubFixture("2026-09-24T10:00:00Z", "aaa", "failure", 7)
	current := newDiscoveryGitHub()
	current.Repository = previous.Repository
	current.TargetBranch = "main"
	current.HeadRepository = previous.HeadRepository
	current.Coverage = append(current.Coverage, DiscoveryGitHubCoverage{Resource: "rulesets", Status: "denied", Detail: "Denied", Source: previous.Requirements[0].Source, ObservedAt: current.ObservedAt})
	c := DiscoveryComparison{Compatible: true, Evidence: []DiscoveryEvidenceTransition{}}
	inspectCompareGitHub(previous, current, &c)
	if len(c.ResolvedFindings) != 0 {
		t.Fatalf("failed evidence resolved by lost access: %+v", c)
	}
	for _, change := range c.RequirementChanges {
		if change.Change == "removed" {
			t.Fatalf("lost access became removed requirement: %+v", change)
		}
	}
}

func TestCompareDiscoveryMalformedRuleIsNotRemovedRequirement(t *testing.T) {
	previous := compareGitHubFixture("2026-09-24T10:00:00Z", "aaa", "success", 7)
	current := compareGitHubFixture("2026-09-24T11:00:00Z", "bbb", "success", 7)
	current.Requirements = []DiscoveryGitHubRequirement{}
	current.Coverage = append(current.Coverage, DiscoveryGitHubCoverage{Resource: "rule:required", Status: "error", Detail: "Missing parameters", Source: previous.Requirements[0].Source, ObservedAt: current.ObservedAt})
	c := DiscoveryComparison{Compatible: true, Evidence: []DiscoveryEvidenceTransition{}}
	inspectCompareGitHub(previous, current, &c)
	for _, change := range c.RequirementChanges {
		if change.Kind == "required_check" && change.Change != "unknown" {
			t.Fatalf("unreadable rule became removed requirement: %+v", change)
		}
	}
}

func TestResolveStructuralCIDemandsRestoredSource(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	workflow := "on: [pull_request]\njobs:\n  contract:\n    runs-on: ubuntu-latest\n    steps: []\n"
	snapshotWrite(t, repo, ".github/workflows/ci.yml", workflow)
	before := InspectLocal(context.Background(), repo, "")
	if err := os.Remove(filepath.Join(repo, ".github/workflows/ci.yml")); err != nil {
		t.Fatal(err)
	}
	deleted := InspectLocal(context.Background(), repo, "")
	removals := ExplainStructuralCI(before.Sources, deleted.Sources, nil)
	if len(removals) != 1 {
		t.Fatalf("expected deletion: %+v", removals)
	}
	if got := ResolveStructuralCI(removals, deleted); len(got) != 0 {
		t.Fatalf("continued deletion became restoration: %+v", got)
	}
	snapshotWrite(t, repo, ".github/workflows/ci.yml", workflow)
	restored := InspectLocal(context.Background(), repo, "")
	if got := ResolveStructuralCI(removals, restored); len(got) != 1 || got[0].After != removals[0].Before {
		t.Fatalf("exact restoration not recognized: %+v", got)
	}
	restored.Coverage = append(restored.Coverage, DiscoveryCoverage{Collector: "local_sources", Status: "partial", Detail: "Source capture incomplete"})
	FinalizeDiscovery(&restored)
	if got := ResolveStructuralCI(removals, restored); len(got) != 0 {
		t.Fatalf("partial inventory manufactured restoration: %+v", got)
	}
}

func TestValidateDiscoveryComparisonRejectsInventedObservationIntervals(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	d := InspectLocal(context.Background(), repo, "")
	c := CompareDiscovery(d, d)
	if err := ValidateDiscoveryComparison(&c); err != nil {
		t.Fatalf("valid comparison: %v", err)
	}
	c.Evidence = []DiscoveryEvidenceTransition{{RequirementID: "required", BeforeStatus: "unknown", AfterStatus: "failing", Comparable: false, LastObservedPassing: d.ObservedAt, FirstObservedFailing: d.ObservedAt, Detail: "Invented"}}
	if err := ValidateDiscoveryComparison(&c); err == nil {
		t.Fatal("unsupported opinion manufactured observed passing interval")
	}
	c.Evidence = nil
	if err := ValidateDiscoveryComparison(&c); err == nil {
		t.Fatal("null evidence accepted")
	}
}

func TestResolveStructuralCIDynamicAlternativePreventsUniqueRestoration(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	workflow := "on: pull_request\njobs:\n  contract:\n    runs-on: ubuntu-latest\n"
	snapshotWrite(t, repo, ".github/workflows/ci.yml", workflow)
	d := InspectLocal(context.Background(), repo, "")
	d.GitHub = compareGitHubFixture("2026-09-24T11:00:00Z", "aaa", "success", 7)
	d.GitHub.Results[0].AppSlug = "github-actions"
	prior := []DiscoveryCIExplanation{{Kind: "required_job_deleted", Path: ".github/workflows/ci.yml", Before: "job contract (contract)", After: "absent", RequirementID: "required:contract", Verification: "verified"}}
	if got := ResolveStructuralCI(prior, d); len(got) != 1 {
		t.Fatalf("known unique restoration missing: %+v", got)
	}
	snapshotWrite(t, repo, ".github/workflows/extra.yml", "on: pull_request\njobs:\n  dynamic:\n    name: ${{ vars.JOB_NAME }}\n    runs-on: ubuntu-latest\n")
	withDynamic := InspectLocal(context.Background(), repo, "")
	withDynamic.GitHub = d.GitHub
	if got := ResolveStructuralCI(prior, withDynamic); len(got) != 0 {
		t.Fatalf("ambiguous dynamic mapping manufactured unique restoration: %+v", got)
	}
}

func TestWorkflowCoverageRejectsOrphanedOrExcludedSource(t *testing.T) {
	d := NewDiscovery()
	d.Subject.Current = true
	d.Sources = []DiscoverySource{{Path: ".github/workflows/ci.yml", Kind: "workflow", Content: "on: pull_request\n", Digest: inspectHash("on: pull_request\n"), StartLine: 1, EndLine: 1, Mode: 0600}}
	if inspectWorkflowCoverage(d) {
		t.Fatal("orphaned workflow source counted as complete")
	}
	d.Inputs = []DiscoveryInput{{Path: d.Sources[0].Path, Kind: "excluded", Digest: d.Sources[0].Digest, Mode: 0600}}
	if inspectWorkflowCoverage(d) {
		t.Fatal("excluded workflow source counted as complete")
	}
}
