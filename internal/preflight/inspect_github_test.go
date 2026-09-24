package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const githubTestHead = "1111111111111111111111111111111111111111"
const githubTestMerge = "2222222222222222222222222222222222222222"

type githubFixture map[string]string

func (f githubFixture) fetch(t *testing.T) githubFetch {
	t.Helper()
	return func(_ context.Context, path string) ([]byte, error) {
		value, ok := f[path]
		if !ok {
			t.Errorf("unexpected GET %s", path)
			return nil, githubAPIError{Kind: "fixture_missing"}
		}
		if strings.HasPrefix(value, "HTTP ") {
			var status int
			fmt.Sscanf(value, "HTTP %d", &status)
			kind := "error"
			if status == 403 {
				kind = "denied"
			}
			if status == 404 {
				kind = "inaccessible"
			}
			return nil, githubAPIError{Status: status, Kind: kind}
		}
		return []byte(value), nil
	}
}
func githubBaseFixture() githubFixture {
	return githubFixture{
		"repos/example/project": `{"default_branch":"main","full_name":"example/project"}`,
		"repos/example/project/pulls?state=open&head=example%3Atopic&per_page=100&page=1":                   `[]`,
		"repos/example/project/rules/branches/main?per_page=100&page=1":                                     `[]`,
		"repos/example/project/branches/main":                                                               `{"protected":false}`,
		"repos/example/project/commits/" + githubTestHead + "/check-runs?filter=latest&per_page=100&page=1": `{"check_runs":[]}`,
		"repos/example/project/commits/" + githubTestHead + "/statuses?per_page=100&page=1":                 `[]`,
	}
}
func githubRequirementFixture() githubFixture {
	f := githubBaseFixture()
	f["repos/example/project/rules/branches/main?per_page=100&page=1"] = `[{"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"example/project","ruleset_id":7,"parameters":{"required_status_checks":[{"context":"test","integration_id":42}]}}]`
	return f
}
func githubRun(id, app int, sha, conclusion string, at time.Time) string {
	return fmt.Sprintf(`{"id":%d,"name":"test","head_sha":%q,"status":"completed","conclusion":%q,"started_at":%q,"completed_at":%q,"app":{"id":%d,"slug":"test-app"}}`, id, sha, conclusion, at.Format(time.RFC3339), at.Format(time.RFC3339), app)
}
func githubObserve(t *testing.T, f githubFixture) *DiscoveryGitHub {
	t.Helper()
	return collectDiscoveryGitHub(context.Background(), "example/project", githubTestHead, "topic", "", false, 0, f.fetch(t))
}
func githubMatch(t *testing.T, g *DiscoveryGitHub, status string) {
	t.Helper()
	if len(g.Matches) != 1 || g.Matches[0].Status != status {
		t.Fatalf("matches=%+v, want %s; coverage=%+v", g.Matches, status, g.Coverage)
	}
}

func TestGitHubRepositoryParsing(t *testing.T) {
	for _, remote := range []string{"https://github.com/example/project.git", "git@github.com:example/project.git", "ssh://git@github.com/example/project"} {
		name, ok := githubRepository(remote)
		if !ok || name != "example/project" {
			t.Errorf("%s: %s %v", remote, name, ok)
		}
	}
	for _, remote := range []string{"https://gitlab.com/example/project", "git@alias:example/project", "https://token@github.com/example/project", "https://github.com/example/project?secret=value", "https://github.com/example/../private"} {
		if _, ok := githubRepository(remote); ok {
			t.Errorf("accepted %s", remote)
		}
	}
}
func TestGitHubNoEnforcedChecksIsNotCIExecution(t *testing.T) {
	g := githubObserve(t, githubBaseFixture())
	if len(g.Requirements) != 0 || len(g.Results) != 0 || g.TargetBranch != "main" || g.LocalCoverage != "clean_head_only" {
		t.Fatalf("%+v", g)
	}
	for _, c := range g.Coverage {
		if c.Status != "complete" {
			t.Errorf("%+v", c)
		}
	}
}
func TestGitHubAppIdentityAndFailuresSurviveDeniedCollector(t *testing.T) {
	f := githubRequirementFixture()
	path := "repos/example/project/commits/" + githubTestHead + "/check-runs?filter=latest&per_page=100&page=1"
	f[path] = `{"check_runs":[` + githubRun(1, 17, githubTestHead, "success", time.Now().Add(-time.Hour)) + `,` + githubRun(2, 42, githubTestHead, "failure", time.Now().Add(-time.Hour)) + `]}`
	f["repos/example/project/commits/"+githubTestHead+"/statuses?per_page=100&page=1"] = "HTTP 403"
	g := githubObserve(t, f)
	githubMatch(t, g, "failed")
	if len(g.Matches[0].ResultIDs) != 1 || !strings.HasSuffix(g.Matches[0].ResultIDs[0], ":2") {
		t.Fatal(g.Matches)
	}
}
func TestGitHubWrongAppCannotMatch(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 17, githubTestHead, "success", time.Now().Add(-time.Hour)) + `]}`
	githubMatch(t, githubObserve(t, f), "missing")
}
func TestGitHubStaleAndRevisionMismatch(t *testing.T) {
	for _, test := range []struct {
		name, sha, status string
		age               time.Duration
	}{{"stale", githubTestHead, "stale", 8 * 24 * time.Hour}, {"wrong_sha", githubTestMerge, "missing", time.Hour}, {"current", githubTestHead, "matched", time.Hour}} {
		t.Run(test.name, func(t *testing.T) {
			f := githubRequirementFixture()
			f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 42, test.sha, "success", time.Now().Add(-test.age)) + `]}`
			githubMatch(t, githubObserve(t, f), test.status)
		})
	}
}
func TestGitHubMergeCandidateAndPRBase(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/pulls/5"] = fmt.Sprintf(`{"number":5,"state":"open","merge_commit_sha":%q,"head":{"sha":%q,"ref":"topic","repo":{"full_name":"example/project"}},"base":{"ref":"release","sha":"base","repo":{"full_name":"example/project"}}}`, githubTestMerge, githubTestHead)
	f["repos/example/project/rules/branches/release?per_page=100&page=1"] = f["repos/example/project/rules/branches/main?per_page=100&page=1"]
	f["repos/example/project/branches/release"] = `{"protected":false}`
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 42, githubTestHead, "success", time.Now().Add(-time.Hour)) + `]}`
	f["repos/example/project/commits/"+githubTestMerge+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(2, 42, githubTestMerge, "failure", time.Now().Add(-time.Hour)) + `]}`
	f["repos/example/project/commits/"+githubTestMerge+"/statuses?per_page=100&page=1"] = `[]`
	g := collectDiscoveryGitHub(context.Background(), "example/project", githubTestHead, "topic", "origin/main", true, 5, f.fetch(t))
	githubMatch(t, g, "failed")
	if g.TargetBranch != "release" || g.TargetSource != "pull_request_base" || g.EvidenceRevision != githubTestMerge || g.LocalCoverage != "stale_local_edits" {
		t.Fatalf("%+v", g)
	}
	f["repos/example/project/commits/"+githubTestMerge+"/check-runs?filter=latest&per_page=100&page=1"] = "HTTP 403"
	g = collectDiscoveryGitHub(context.Background(), "example/project", githubTestHead, "topic", "", false, 5, f.fetch(t))
	githubMatch(t, g, "unknown")
}
func TestGitHubLegacyStatusDoesNotProveApp(t *testing.T) {
	f := githubRequirementFixture()
	now := time.Now().Add(-time.Minute).Format(time.RFC3339)
	f["repos/example/project/commits/"+githubTestHead+"/statuses?per_page=100&page=1"] = fmt.Sprintf(`[{"id":1,"context":"test","state":"success","created_at":%q,"updated_at":%q,"creator":{"login":"bot"}}]`, now, now)
	githubMatch(t, githubObserve(t, f), "unknown")
}
func TestGitHubSameNameCheckAndStatusBothMatter(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/rules/branches/main?per_page=100&page=1"] = strings.ReplaceAll(f["repos/example/project/rules/branches/main?per_page=100&page=1"], `,"integration_id":42`, "")
	now := time.Now().Add(-time.Minute)
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(2, 42, githubTestHead, "success", now) + `]}`
	f["repos/example/project/commits/"+githubTestHead+"/statuses?per_page=100&page=1"] = fmt.Sprintf(`[{"id":1,"context":"test","state":"failure","created_at":%q,"updated_at":%q}]`, now.Format(time.RFC3339), now.Format(time.RFC3339))
	githubMatch(t, githubObserve(t, f), "failed")
}
func TestGitHubPaginationPreservesPartialResults(t *testing.T) {
	items := make([]map[string]int, 100)
	for i := range items {
		items[i] = map[string]int{"id": i + 1}
	}
	body, _ := json.Marshal(items)
	calls := 0
	fetch := func(_ context.Context, path string) ([]byte, error) {
		calls++
		if calls == 1 {
			if !strings.HasSuffix(path, "per_page=100&page=1") {
				t.Fatal(path)
			}
			return body, nil
		}
		return nil, githubAPIError{Status: 403, Kind: "denied"}
	}
	result, err := githubPages[map[string]int](context.Background(), fetch, "repos/example/project/items", "")
	if err == nil || len(result) != 100 || calls != 2 {
		t.Fatalf("%d %d %v", len(result), calls, err)
	}
	calls = 0
	fetch = func(_ context.Context, path string) ([]byte, error) {
		calls++
		if calls == 1 {
			return body, nil
		}
		return []byte(`[{"id":101}]`), nil
	}
	result, err = githubPages[map[string]int](context.Background(), fetch, "repos/example/project/items", "")
	if err != nil || len(result) != 101 || calls != 2 {
		t.Fatalf("%d %d %v", len(result), calls, err)
	}
}
func TestGitHubPaginationBound(t *testing.T) {
	items := make([]int, 100)
	body, _ := json.Marshal(items)
	calls := 0
	result, err := githubPages[int](context.Background(), func(context.Context, string) ([]byte, error) { calls++; return body, nil }, "repos/example/project/items", "")
	if err == nil || calls != githubPageLimit || len(result) != 100*githubPageLimit {
		t.Fatalf("%d %d %v", len(result), calls, err)
	}
}
func TestGitHubProtectionMissingAccessNotNoRequirements(t *testing.T) {
	f := githubBaseFixture()
	f["repos/example/project/branches/main"] = `{"protected":true}`
	f["repos/example/project/branches/main/protection"] = "HTTP 404"
	f["repos/example/project/rules/branches/main?per_page=100&page=1"] = "HTTP 403"
	g := githubObserve(t, f)
	if g.completeResource("branch_protection") || g.completeResource("rulesets") {
		t.Fatal(g.Coverage)
	}
	if len(g.Requirements) != 0 {
		t.Fatal(g.Requirements)
	}
}
func TestGitHubLegacyProtectionAndUnsupportedRules(t *testing.T) {
	f := githubBaseFixture()
	f["repos/example/project/branches/main"] = `{"protected":true}`
	f["repos/example/project/branches/main/protection"] = `{"required_status_checks":{"contexts":["test","legacy"],"checks":[{"context":"test","app_id":42}],"strict":true},"required_pull_request_reviews":{"required_approving_review_count":2,"require_code_owner_reviews":true},"required_signatures":{"enabled":true}}`
	f["repos/example/project/rules/branches/main?per_page=100&page=1"] = `[{"ruleset_id":8,"type":"merge_queue","ruleset_source_type":"Organization","ruleset_source":"example","parameters":{}}]`
	g := githubObserve(t, f)
	if len(g.Requirements) != 2 || g.Requirements[0].AppID != 42 || len(g.Rules) != 4 {
		t.Fatalf("%+v", g)
	}
	found := false
	for _, r := range g.Rules {
		if r.Type == "pull_request" && r.ReviewCount == 2 && r.CodeOwnerReview {
			found = true
		}
	}
	if !found {
		t.Fatal(g.Rules)
	}
}
func TestGitHubGHIsReadOnlyAndSanitizesErrors(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PREFLIGHT_TEST_ARGV\"\nprintf 'HTTP/1.1 403 Forbidden\\r\\nContent-Length: 28\\r\\n\\r\\n{\"message\":\"secret-value\"}'\nprintf 'credential-secret' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PREFLIGHT_TEST_ARGV", log)
	_, err := githubGET(context.Background(), "repos/example/project")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	args, _ := os.ReadFile(log)
	if !strings.Contains(string(args), "--method\nGET\n") || !strings.Contains(string(args), "--hostname\ngithub.com\n") {
		t.Fatalf("%s", args)
	}
}

func TestGitHubDuplicateSameAppCannotEraseFailure(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 42, githubTestHead, "failure", time.Now().Add(-time.Hour)) + `,` + githubRun(2, 42, githubTestHead, "success", time.Now().Add(-time.Minute)) + `]}`
	g := githubObserve(t, f)
	githubMatch(t, g, "failed")
	if len(g.Matches[0].ResultIDs) != 2 {
		t.Fatal(g.Matches)
	}
}
func TestGitHubSavedObservationCannotManufactureSuccessfulMatch(t *testing.T) {
	g := githubObserve(t, githubRequirementFixture())
	if err := ValidateDiscoveryGitHub(g); err != nil {
		t.Fatal(err)
	}
	g.Matches[0].Status = "matched"
	if err := ValidateDiscoveryGitHub(g); err == nil {
		t.Fatal("accepted invented successful check")
	}
	f := githubRequirementFixture()
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(1, 42, githubTestHead, "failure", time.Now().Add(-time.Hour)) + `]}`
	g = githubObserve(t, f)
	g.Matches[0].Status = "matched"
	if err := ValidateDiscoveryGitHub(g); err == nil {
		t.Fatal("accepted failed check as successful")
	}
}
func TestGitHubValidateAllOrdinaryFixtures(t *testing.T) {
	for _, f := range []githubFixture{githubBaseFixture(), githubRequirementFixture()} {
		g := githubObserve(t, f)
		if err := ValidateDiscoveryGitHub(g); err != nil {
			t.Fatal(err)
		}
	}
	g := newDiscoveryGitHub()
	g.cover("provider", "unsupported", "Unsupported provider", "origin")
	if err := ValidateDiscoveryGitHub(g); err != nil {
		t.Fatal(err)
	}
}
func TestGitHubMalformedResponsesRemainUnknown(t *testing.T) {
	for _, body := range []string{"null", `{}`, `{"check_runs":null}`} {
		t.Run(body, func(t *testing.T) {
			f := githubRequirementFixture()
			f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = body
			githubMatch(t, githubObserve(t, f), "unknown")
		})
	}
}

func TestGitHubAuthorizedRealRepository(t *testing.T) {
	if os.Getenv("PREFLIGHT_GITHUB_READONLY_TEST") != "1" {
		t.Skip("explicit read-only GitHub smoke test; PREFLIGHT_GITHUB_READONLY_TEST=1")
	}
	// The issue authorizes this repository only. No mutation or CI trigger occurs.
	var current struct {
		SHA string `json:"sha"`
	}
	if err := githubObject(context.Background(), githubGET, "repos/rebaze/preflight/commits/main", &current); err != nil || current.SHA == "" {
		t.Fatalf("resolve authorized main: %v", err)
	}
	started := time.Now()
	g := collectDiscoveryGitHub(context.Background(), "rebaze/preflight", current.SHA, "main", "main", false, 0, githubGET)
	if err := ValidateDiscoveryGitHub(g); err != nil {
		t.Fatal(err)
	}
	if !g.completeResource("repository") || g.TargetBranch != "main" {
		t.Fatalf("GitHub repository metadata unavailable: %+v", g.Coverage)
	}
	t.Logf("read-only github.com/rebaze/preflight: %.3fs; target=%s; rules=%d; requirements=%d; results=%d; local_coverage=%s", time.Since(started).Seconds(), g.TargetBranch, len(g.Rules), len(g.Requirements), len(g.Results), g.LocalCoverage)
	for _, c := range g.Coverage {
		t.Logf("%s: %s (%s)", c.Resource, c.Status, c.Detail)
	}
}

func TestGitHubMissingResultIdentityCannotMatch(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/commits/"+githubTestHead+"/check-runs?filter=latest&per_page=100&page=1"] = `{"check_runs":[` + githubRun(0, 42, githubTestHead, "success", time.Now().Add(-time.Hour)) + `]}`
	g := githubObserve(t, f)
	githubMatch(t, g, "unknown")
	if err := ValidateDiscoveryGitHub(g); err != nil {
		t.Fatal(err)
	}
}
func TestGitHubMissingRuleIdentityIsPartial(t *testing.T) {
	f := githubRequirementFixture()
	f["repos/example/project/rules/branches/main?per_page=100&page=1"] = `[{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"test"}]}}]`
	g := githubObserve(t, f)
	if len(g.Requirements) != 0 {
		t.Fatal(g.Requirements)
	}
	found := false
	for _, c := range g.Coverage {
		if c.Resource == "rulesets.identity" && c.Status == "error" {
			found = true
		}
	}
	if !found {
		t.Fatal(g.Coverage)
	}
	if err := ValidateDiscoveryGitHub(g); err != nil {
		t.Fatal(err)
	}
}
