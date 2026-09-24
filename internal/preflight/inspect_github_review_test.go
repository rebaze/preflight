package preflight

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestGitHubReviewForkHeadUsesTargetRepository(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/pulls/5"] = fmt.Sprintf(`{"number":5,"state":"open","merge_commit_sha":%q,"head":{"sha":%q,"ref":"topic","repo":{"full_name":"contributor/fork"}},"base":{"ref":"main","repo":{"full_name":"example/project"}}}`, githubTestMerge, githubTestHead)
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 42, githubTestHead, "success", time.Now().Add(-time.Minute)) + `]}`
	f["repos/example/project/commits/"+githubTestMerge+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[]}`
	f["repos/example/project/commits/"+githubTestMerge+"/statuses?per_page=100&page=1"] = `[]`
	g := collectDiscoveryGitHub(context.Background(), "example/project", githubTestHead, "topic", "", false, 5, f.fetch(t))
	githubMatch(t, g, "matched")
	if g.HeadRepository != "contributor/fork" || g.Results[0].Repository != g.Repository {
		t.Fatalf("repository identity: %+v", g)
	}
	for _, subject := range []string{"head", "merge_candidate"} {
		t.Run(subject, func(t *testing.T) {
			copy := *g
			copy.EvidenceSubject = subject
			copy.EvidenceRevision = githubTestHead
			copy.MergeCandidate = githubTestHead
			copy.Results = append([]DiscoveryGitHubResult{}, g.Results...)
			copy.Results[0].Repository = "unrelated/project"
			if ValidateDiscoveryGitHub(&copy) == nil {
				t.Fatal("validator accepted unrelated repository match")
			}
			copy.Matches = nil
			copy.matchRequirements()
			githubMatch(t, &copy, "missing")
		})
	}
}

func TestGitHubReviewForgedLegacyAppCannotMatch(t *testing.T) {
	f := githubRequirementFixture()
	now := time.Now().Add(-time.Minute).Format(time.RFC3339)
	f["repos/example/project/commits/"+githubTestHead+"/statuses?per_page=100&page=1"] = fmt.Sprintf(`[{"id":1,"context":"test","state":"success","created_at":%q,"updated_at":%q}]`, now, now)
	g := githubObserve(t, f)
	g.Results[0].AppID = 42
	g.Matches = nil
	g.matchRequirements()
	githubMatch(t, g, "unknown")
	if ValidateDiscoveryGitHub(g) == nil {
		t.Fatal("accepted forged legacy app identity")
	}
}

func TestGitHubReviewLegacyUpdatedAtSelectsLatestState(t *testing.T) {
	f := githubBaseFixture()
	f["repos/example/project/rules/branches/main?per_page=100&page=1"] = `[{"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"example/project","ruleset_id":7,"parameters":{"required_status_checks":[{"context":"test"}]}}]`
	now := time.Now()
	a, b, c := now.Add(-3*time.Hour).Format(time.RFC3339), now.Add(-2*time.Hour).Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339)
	f["repos/example/project/commits/"+githubTestHead+"/statuses?per_page=100&page=1"] = fmt.Sprintf(`[{"id":1,"context":"test","state":"failure","created_at":%q,"updated_at":%q},{"id":2,"context":"test","state":"success","created_at":%q,"updated_at":%q}]`, a, c, b, b)
	g := githubObserve(t, f)
	githubMatch(t, g, "failed")
	if g.Results[0].UpdatedAt != c {
		t.Fatal("lost updated_at")
	}
}

func TestGitHubReviewStaleConclusion(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 42, githubTestHead, "stale", time.Now().Add(-time.Minute)) + `]}`
	githubMatch(t, githubObserve(t, f), "stale")
}

func TestGitHubReviewMergeSelectionBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, merge, checks, statuses, want string }{
		{"empty_complete", githubTestMerge, `{"check_runs":[]}`, `[]`, "head"},
		{"unavailable_revision", "", ``, ``, "unknown"},
		{"denied", githubTestMerge, `HTTP 403`, `[]`, "unknown"},
		{"present_partial", githubTestMerge, `{"check_runs":[` + githubRun(2, 42, githubTestMerge, "failure", time.Now().Add(-time.Minute)) + `]}`, `HTTP 403`, "merge_candidate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := githubRequirementFixture()
			f["repos/example/project/pulls/5"] = fmt.Sprintf(`{"number":5,"state":"open","merge_commit_sha":%q,"head":{"sha":%q,"ref":"topic","repo":{"full_name":"example/project"}},"base":{"ref":"main","repo":{"full_name":"example/project"}}}`, tc.merge, githubTestHead)
			f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 42, githubTestHead, "success", time.Now().Add(-time.Minute)) + `]}`
			f["repos/example/project/commits/"+githubTestMerge+"/check-runs?filter=latest&per_page=100&page=1"] = tc.checks
			f["repos/example/project/commits/"+githubTestMerge+"/statuses?per_page=100&page=1"] = tc.statuses
			g := collectDiscoveryGitHub(context.Background(), "example/project", githubTestHead, "topic", "", false, 5, f.fetch(t))
			if g.EvidenceSubject != tc.want {
				t.Fatalf("subject=%s want %s", g.EvidenceSubject, tc.want)
			}
			if tc.want == "head" {
				githubMatch(t, g, "matched")
			}
			if tc.want == "unknown" {
				githubMatch(t, g, "unknown")
			}
			if tc.want == "merge_candidate" {
				githubMatch(t, g, "failed")
			}
		})
	}
}

func TestGitHubReviewFreshnessDiagnosticBound(t *testing.T) {
	repo, _, _ := snapshotFixture(t)
	d := InspectLocal(context.Background(), repo, "")
	for i := 0; i < 140; i++ {
		inspectDiagnostic(&d, "synthetic", "bounded diagnostic", "", false)
	}
	before := len(d.Diagnostics)
	d.Subject.InputDigest = "changed"
	InspectGitHub(context.Background(), &d, "", 0)
	if len(d.Diagnostics) != before {
		t.Fatalf("diagnostics grew beyond cap: %d -> %d", before, len(d.Diagnostics))
	}
	if d.Subject.Current || d.GitHub.LocalCoverage != "stale_local_edits" || d.ExitCode == 0 {
		t.Fatal("freshness failure lost")
	}
}
