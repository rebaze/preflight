package preflight

// GitHub discovery uses GET requests only. The normalized observation deliberately
// records facts and gaps, not a merge-eligibility or release-authorization verdict.
import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const githubPageLimit = 10

type DiscoveryGitHub struct {
	Repository       string                       `json:"repository"`
	ObservedAt       string                       `json:"observed_at"`
	TargetBranch     string                       `json:"target_branch"`
	TargetSource     string                       `json:"target_source"`
	PullRequest      int                          `json:"pull_request"`
	Head             string                       `json:"head"`
	HeadRepository   string                       `json:"head_repository"`
	MergeCandidate   string                       `json:"merge_candidate"`
	EvidenceRevision string                       `json:"evidence_revision"`
	EvidenceSubject  string                       `json:"evidence_subject"`
	LocalCoverage    string                       `json:"local_coverage"`
	Rules            []DiscoveryGitHubRule        `json:"rules"`
	Requirements     []DiscoveryGitHubRequirement `json:"requirements"`
	Results          []DiscoveryGitHubResult      `json:"results"`
	Matches          []DiscoveryGitHubMatch       `json:"matches"`
	Coverage         []DiscoveryGitHubCoverage    `json:"coverage"`
}

type DiscoveryGitHubCoverage struct {
	Resource   string `json:"resource"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
	Source     string `json:"source"`
	ObservedAt string `json:"observed_at"`
}

type DiscoveryGitHubRule struct {
	ID                  string `json:"id"`
	Type                string `json:"type"`
	Source              string `json:"source"`
	SourceType          string `json:"source_type"`
	RulesetID           int64  `json:"ruleset_id"`
	Supported           bool   `json:"supported"`
	ReviewCount         int    `json:"review_count"`
	CodeOwnerReview     bool   `json:"code_owner_review"`
	DismissStaleReviews bool   `json:"dismiss_stale_reviews"`
	LastPushApproval    bool   `json:"last_push_approval"`
	StrictChecks        bool   `json:"strict_checks"`
	ObservedAt          string `json:"observed_at"`
}

type DiscoveryGitHubRequirement struct {
	ID         string `json:"id"`
	RuleID     string `json:"rule_id"`
	Context    string `json:"context"`
	AppID      int64  `json:"app_id"`
	Producer   string `json:"producer"`
	Source     string `json:"source"`
	ObservedAt string `json:"observed_at"`
}

type DiscoveryGitHubResult struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Revision    string `json:"revision"`
	Repository  string `json:"repository"`
	AppID       int64  `json:"app_id"`
	AppSlug     string `json:"app_slug"`
	Creator     string `json:"creator"`
	State       string `json:"state"`
	Conclusion  string `json:"conclusion"`
	CompletedAt string `json:"completed_at"`
	UpdatedAt   string `json:"updated_at"`
	Source      string `json:"source"`
	ObservedAt  string `json:"observed_at"`
}

type DiscoveryGitHubMatch struct {
	RequirementID string   `json:"requirement_id"`
	Revision      string   `json:"revision"`
	Status        string   `json:"status"`
	ResultIDs     []string `json:"result_ids"`
	Detail        string   `json:"detail"`
}

type githubFetch func(context.Context, string) ([]byte, error)
type githubAPIError struct {
	Status int
	Kind   string
}

func (e githubAPIError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("GitHub HTTP %d (%s)", e.Status, e.Kind)
	}
	return "GitHub " + e.Kind
}

// No response body or stderr is returned as an error: either can contain private
// source text, credential-provider output or hostile instruction-like content.
func githubGET(ctx context.Context, endpoint string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "api", "--hostname", "github.com", "--method", "GET", "--include", "-H", "Accept: application/vnd.github+json", "-H", "X-GitHub-Api-Version: 2022-11-28", endpoint)
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GH_PAGER=cat")
	out := &snapshotBoundedBuffer{limit: 2 << 20}
	errout := &snapshotBoundedBuffer{limit: 16 << 10}
	cmd.Stdout = out
	cmd.Stderr = errout
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return nil, githubAPIError{Kind: "timeout"}
	}
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(out.Bytes())), nil)
	if err != nil {
		return nil, githubAPIError{Kind: "unavailable_or_invalid_response"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		kind := "error"
		if response.StatusCode == 401 || response.StatusCode == 403 {
			kind = "denied"
		}
		if response.StatusCode == 404 {
			kind = "inaccessible"
		}
		return nil, githubAPIError{Status: response.StatusCode, Kind: kind}
	}
	if runErr != nil {
		return nil, githubAPIError{Kind: "command_or_output_limit"}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return nil, githubAPIError{Kind: "output_limit"}
	}
	return data, nil
}

var githubSlug = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func githubRepository(remote string) (string, bool) {
	remote = strings.TrimSpace(remote)
	var name string
	if strings.HasPrefix(remote, "git@github.com:") {
		name = strings.TrimPrefix(remote, "git@github.com:")
	} else {
		u, err := url.Parse(remote)
		if err != nil || !strings.EqualFold(u.Host, "github.com") || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "ssh") {
			return "", false
		}
		if u.User != nil {
			if _, set := u.User.Password(); set || u.User.Username() != "git" {
				return "", false
			}
		}
		name = strings.TrimPrefix(u.Path, "/")
	}
	name = strings.TrimSuffix(name, ".git")
	return name, githubSlug.MatchString(name) && !strings.Contains(name, "..")
}

func newDiscoveryGitHub() *DiscoveryGitHub {
	return &DiscoveryGitHub{ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), LocalCoverage: "unknown", EvidenceSubject: "unknown", Rules: []DiscoveryGitHubRule{}, Requirements: []DiscoveryGitHubRequirement{}, Results: []DiscoveryGitHubResult{}, Matches: []DiscoveryGitHubMatch{}, Coverage: []DiscoveryGitHubCoverage{}}
}

// CollectDiscoveryGitHub is an explicit read-only adapter for github.com origin.
// base is comparison context only; a selected open PR's base takes precedence.
func CollectDiscoveryGitHub(ctx context.Context, root, head, branch, base string, dirty bool, pr int) *DiscoveryGitHub {
	g := newDiscoveryGitHub()
	remote, err := snapshotGitRead(ctx, root, "remote", "get-url", "origin")
	if err != nil {
		g.cover("provider", "unavailable", "No readable origin remote", "origin")
		return g
	}
	repository, ok := githubRepository(string(remote))
	if !ok {
		g.cover("provider", "unsupported", "Only an explicit github.com origin is supported; SSH aliases are not resolved", "origin")
		return g
	}
	return collectDiscoveryGitHub(ctx, repository, head, branch, base, dirty, pr, githubGET)
}

func (g *DiscoveryGitHub) cover(resource, status, detail, source string) {
	g.Coverage = append(g.Coverage, DiscoveryGitHubCoverage{Resource: resource, Status: status, Detail: detail, Source: source, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)})
}
func (g *DiscoveryGitHub) failure(resource, source string, err error) {
	status := "error"
	if e, ok := err.(githubAPIError); ok {
		if e.Kind == "denied" || e.Kind == "inaccessible" {
			status = e.Kind
		} else if e.Kind == "timeout" {
			status = "limited"
		}
	}
	g.cover(resource, status, err.Error(), "https://api.github.com/"+source)
}
func githubDecode(data []byte, v any) error {
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return githubAPIError{Kind: "invalid_response"}
	}
	if err := json.Unmarshal(data, v); err != nil {
		return githubAPIError{Kind: "invalid_response"}
	}
	return nil
}
func githubObject(ctx context.Context, fetch githubFetch, path string, v any) error {
	data, err := fetch(ctx, path)
	if err != nil {
		return err
	}
	return githubDecode(data, v)
}

// Pagination is explicit and bounded; useful earlier pages survive later errors.
// API input accepts additive fields; the saved normalized contract is strict.
func githubPages[T any](ctx context.Context, fetch githubFetch, path, key string) ([]T, error) {
	result := []T{}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for page := 1; page <= githubPageLimit; page++ {
		data, err := fetch(ctx, path+sep+"per_page=100&page="+strconv.Itoa(page))
		if err != nil {
			return result, err
		}
		if key != "" {
			var wrapper map[string]json.RawMessage
			if err = githubDecode(data, &wrapper); err != nil {
				return result, err
			}
			var ok bool
			data, ok = wrapper[key]
			if !ok {
				return result, githubAPIError{Kind: "missing_response_field"}
			}
		}
		var values []T
		if err = githubDecode(data, &values); err != nil {
			return result, err
		}
		result = append(result, values...)
		if len(values) < 100 {
			return result, nil
		}
	}
	return result, githubAPIError{Kind: "pagination_limit"}
}

type githubPR struct {
	Number         int         `json:"number"`
	State          string      `json:"state"`
	MergeCommitSHA string      `json:"merge_commit_sha"`
	Head           githubPRRef `json:"head"`
	Base           githubPRRef `json:"base"`
}
type githubPRRef struct {
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repo"`
}
type githubRule struct {
	Type       string          `json:"type"`
	SourceType string          `json:"ruleset_source_type"`
	Source     string          `json:"ruleset_source"`
	ID         int64           `json:"ruleset_id"`
	Parameters json.RawMessage `json:"parameters"`
}
type githubRuleParameters struct {
	Checks []struct {
		Context string `json:"context"`
		AppID   *int64 `json:"integration_id"`
	} `json:"required_status_checks"`
	Reviews    *int `json:"required_approving_review_count"`
	CodeOwners bool `json:"require_code_owner_review"`
	Dismiss    bool `json:"dismiss_stale_reviews_on_push"`
	LastPush   bool `json:"require_last_push_approval"`
	Strict     bool `json:"strict_required_status_checks_policy"`
}

func collectDiscoveryGitHub(ctx context.Context, repository, localHead, branch, base string, dirty bool, prNumber int, fetch githubFetch) *DiscoveryGitHub {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Share a bounded request and byte budget across all independent collectors.
	rawFetch := fetch
	requests, bytesRead := 0, 0
	fetch = func(ctx context.Context, path string) ([]byte, error) {
		if ctx.Err() != nil {
			return nil, githubAPIError{Kind: "timeout"}
		}
		if requests >= 40 || bytesRead >= 8<<20 {
			return nil, githubAPIError{Kind: "budget_limit"}
		}
		requests++
		data, err := rawFetch(ctx, path)
		bytesRead += len(data)
		if bytesRead > 8<<20 {
			return nil, githubAPIError{Kind: "budget_limit"}
		}
		return data, err
	}
	g := newDiscoveryGitHub()
	g.Repository = repository
	g.HeadRepository = repository
	g.Head = localHead
	prefix := "repos/" + repository
	var meta struct {
		DefaultBranch string `json:"default_branch"`
		FullName      string `json:"full_name"`
	}
	if err := githubObject(ctx, fetch, prefix, &meta); err != nil {
		g.failure("repository", prefix, err)
	} else if meta.FullName == "" || meta.DefaultBranch == "" || !strings.EqualFold(meta.FullName, repository) {
		g.failure("repository", prefix, githubAPIError{Kind: "invalid_repository_identity"})
	} else {
		g.cover("repository", "complete", "Repository metadata observed", "https://api.github.com/"+prefix)
		g.TargetBranch = meta.DefaultBranch
		g.TargetSource = "default_branch"
	}
	if base != "" {
		g.TargetBranch = strings.TrimPrefix(strings.TrimPrefix(base, "refs/heads/"), "origin/")
		g.TargetSource = "comparison_base"
	}
	var pr githubPR
	if prNumber > 0 {
		path := prefix + "/pulls/" + strconv.Itoa(prNumber)
		if err := githubObject(ctx, fetch, path, &pr); err != nil {
			g.failure("pull_request", path, err)
		} else if pr.Number != prNumber || pr.State != "open" {
			g.failure("pull_request", path, githubAPIError{Kind: "not_an_open_pull_request"})
			pr = githubPR{}
		} else {
			g.cover("pull_request", "complete", "Selected open pull request observed", "https://api.github.com/"+path)
		}
	} else if branch != "" {
		path := prefix + "/pulls?state=open&head=" + url.QueryEscape(strings.Split(repository, "/")[0]+":"+branch)
		prs, err := githubPages[githubPR](ctx, fetch, path, "")
		if err != nil {
			g.failure("pull_request", path, err)
		} else if len(prs) > 1 {
			g.cover("pull_request", "ambiguous", "Multiple open pull requests; select one with --pr", "https://api.github.com/"+path)
		} else if len(prs) == 1 {
			detailPath := prefix + "/pulls/" + strconv.Itoa(prs[0].Number)
			if err := githubObject(ctx, fetch, detailPath, &pr); err != nil {
				g.failure("pull_request", detailPath, err)
			} else if pr.State != "open" {
				g.failure("pull_request", detailPath, githubAPIError{Kind: "pull_request_changed"})
				pr = githubPR{}
			} else {
				g.cover("pull_request", "complete", "Unique open pull request observed", "https://api.github.com/"+detailPath)
			}
		} else {
			g.cover("pull_request", "complete", "No open pull request for the local branch", "https://api.github.com/"+path)
		}
	} else {
		g.cover("pull_request", "unsupported", "Detached HEAD; use --pr to select a pull request", "local HEAD")
	}
	if pr.Number > 0 {
		if pr.Base.Repo.FullName != repository || pr.Base.Ref == "" || pr.Head.SHA == "" || !githubSlug.MatchString(pr.Head.Repo.FullName) {
			g.cover("pull_request", "error", "Pull request has incompatible or incomplete repository/revision identity", "https://api.github.com/"+prefix+"/pulls/"+strconv.Itoa(pr.Number))
		} else {
			g.PullRequest = pr.Number
			g.TargetBranch = pr.Base.Ref
			g.TargetSource = "pull_request_base"
			g.Head = pr.Head.SHA
			g.HeadRepository = pr.Head.Repo.FullName
			g.MergeCandidate = pr.MergeCommitSHA
		}
	}
	if g.TargetBranch == "" {
		g.cover("requirements", "unknown", "Target branch could not be resolved", prefix)
	} else {
		g.collectRules(ctx, fetch, prefix)
		g.collectProtection(ctx, fetch, prefix)
	}
	if g.Head != "" {
		g.collectResults(ctx, fetch, g.Repository, g.Head, "head")
	}
	if g.MergeCandidate != "" && g.MergeCandidate != g.Head {
		g.collectResults(ctx, fetch, repository, g.MergeCandidate, "merge_candidate")
	}
	g.EvidenceRevision = g.Head
	g.EvidenceSubject = "head"
	if g.MergeCandidate != "" && g.MergeCandidate != g.Head {
		mergeKnown := g.completeResource("merge_candidate.check_runs") && g.completeResource("merge_candidate.statuses")
		mergePresent := false
		for _, r := range g.Results {
			if r.Revision == g.MergeCandidate {
				mergePresent = true
			}
		}
		if mergePresent {
			g.EvidenceRevision = g.MergeCandidate
			g.EvidenceSubject = "merge_candidate"
		} else if !mergeKnown {
			g.EvidenceRevision = ""
			g.EvidenceSubject = "unknown"
		}
	}
	if dirty {
		g.LocalCoverage = "stale_local_edits"
	} else if localHead == "" || localHead != g.Head {
		g.LocalCoverage = "revision_mismatch"
	} else {
		g.LocalCoverage = "clean_head_only"
	}
	// Recheck mutable PR selection after collecting immutable revision results.
	// Preserve captured facts if it moved, while making current applicability unknown.
	if g.PullRequest > 0 {
		path := prefix + "/pulls/" + strconv.Itoa(g.PullRequest)
		var current githubPR
		if err := githubObject(ctx, fetch, path, &current); err != nil {
			g.failure("pull_request_freshness", path, err)
			g.LocalCoverage = "unknown"
		} else if current.State != "open" || current.Head.SHA != g.Head || current.Head.Repo.FullName != g.HeadRepository || current.Base.Ref != g.TargetBranch || current.Base.Repo.FullName != g.Repository || current.MergeCommitSHA != g.MergeCandidate {
			g.cover("pull_request_freshness", "unknown", "Pull request identities changed during observation; retained results cover the recorded revisions only", "https://api.github.com/"+path)
			g.LocalCoverage = "revision_mismatch"
		} else {
			g.cover("pull_request_freshness", "complete", "Pull request identities remained unchanged during collection", "https://api.github.com/"+path)
		}
	}
	g.matchRequirements()
	return g
}
func (g *DiscoveryGitHub) completeResource(resource string) bool {
	for _, c := range g.Coverage {
		if c.Resource == resource {
			return c.Status == "complete"
		}
	}
	return false
}

func (g *DiscoveryGitHub) collectRules(ctx context.Context, fetch githubFetch, prefix string) {
	path := prefix + "/rules/branches/" + url.PathEscape(g.TargetBranch)
	rules, err := githubPages[githubRule](ctx, fetch, path, "")
	if err != nil {
		g.failure("rulesets", path, err)
	} else {
		g.cover("rulesets", "complete", "Effective active rules observed; bypass eligibility is not evaluated", "https://api.github.com/"+path)
	}
	// API ordering is not a requirement identity. Repeated authoritative keys
	// receive content identities and explicit incomplete coverage; keep every
	// distinct declaration so a conflicting duplicate cannot erase a known gate.
	counts := map[string]int{}
	for _, rule := range rules {
		counts[githubRuleIdentity(rule)]++
	}
	seenRules, reportedDuplicates := map[string]bool{}, map[string]bool{}
	for _, rule := range rules {
		if rule.ID <= 0 || rule.Type == "" || rule.Source == "" || rule.SourceType == "" {
			g.cover("rulesets.identity", "error", "Rule identity or source is incomplete", "https://api.github.com/"+path)
			continue
		}
		id := githubRuleIdentity(rule)
		if counts[id] > 1 {
			if !reportedDuplicates[id] {
				g.cover("rulesets.duplicates", "ambiguous", "Repeated rule identity; all distinct declarations retained, effective requirements need review", "https://api.github.com/"+path)
				reportedDuplicates[id] = true
			}
			var canonical any
			_ = json.Unmarshal(rule.Parameters, &canonical)
			encoded, _ := json.Marshal(canonical)
			id += ":" + inspectHash(string(encoded))
		}
		if seenRules[id] {
			continue
		}
		seenRules[id] = true
		r := DiscoveryGitHubRule{ID: id, Type: rule.Type, Source: "https://api.github.com/" + path, SourceType: rule.SourceType + ":" + rule.Source, RulesetID: rule.ID, ObservedAt: g.ObservedAt}
		var params githubRuleParameters
		switch rule.Type {
		case "required_status_checks", "pull_request":
			if err := githubDecode(rule.Parameters, &params); err != nil {
				g.cover("rule:"+id, "error", "Rule parameters are unavailable or malformed", r.Source)
				break
			}
			r.Supported = true
			r.CodeOwnerReview = params.CodeOwners
			r.DismissStaleReviews = params.Dismiss
			r.LastPushApproval = params.LastPush
			r.StrictChecks = params.Strict
			if params.Reviews != nil {
				r.ReviewCount = *params.Reviews
			}
			if rule.Type == "required_status_checks" {
				if params.Checks == nil {
					r.Supported = false
					g.cover("rule:"+id, "error", "Required status check list is missing", r.Source)
				}
				for _, check := range params.Checks {
					g.addRequirement(id, check.Context, check.AppID, r.Source)
				}
			}
			if rule.Type == "pull_request" && params.Reviews == nil {
				r.Supported = false
				g.cover("rule:"+id, "error", "Required review count is missing", r.Source)
			}
		default:
			g.cover("rule:"+id, "unsupported", "Rule is recorded but its semantics are not evaluated: "+rule.Type, r.Source)
		}
		g.Rules = append(g.Rules, r)
	}
}
func (g *DiscoveryGitHub) addRequirement(ruleID, name string, app *int64, source string) {
	if name == "" {
		g.cover("rule:"+ruleID, "error", "Required check context is missing", source)
		return
	}
	appID := int64(-1)
	producer := "any"
	if app != nil && *app > 0 {
		appID = *app
		producer = "app"
	}
	g.Requirements = append(g.Requirements, DiscoveryGitHubRequirement{ID: fmt.Sprintf("%s:check:%s:%d", ruleID, name, appID), RuleID: ruleID, Context: name, AppID: appID, Producer: producer, Source: source, ObservedAt: g.ObservedAt})
}

func (g *DiscoveryGitHub) collectProtection(ctx context.Context, fetch githubFetch, prefix string) {
	branchPath := prefix + "/branches/" + url.PathEscape(g.TargetBranch)
	var branch struct {
		Protected *bool `json:"protected"`
	}
	if err := githubObject(ctx, fetch, branchPath, &branch); err == nil && branch.Protected != nil && !*branch.Protected {
		g.cover("branch_protection", "complete", "Branch metadata explicitly reports no protection", "https://api.github.com/"+branchPath)
		return
	}
	path := branchPath + "/protection"
	var p struct {
		Checks *struct {
			Contexts []string `json:"contexts"`
			Checks   []struct {
				Context string `json:"context"`
				AppID   *int64 `json:"app_id"`
			} `json:"checks"`
			Strict bool `json:"strict"`
		} `json:"required_status_checks"`
		Reviews *struct {
			Count      int  `json:"required_approving_review_count"`
			CodeOwners bool `json:"require_code_owner_reviews"`
			Dismiss    bool `json:"dismiss_stale_reviews"`
			LastPush   bool `json:"require_last_push_approval"`
		} `json:"required_pull_request_reviews"`
	}
	data, err := fetch(ctx, path)
	if err != nil {
		g.failure("branch_protection", path, err)
		return
	}
	var fields map[string]json.RawMessage
	if err = githubDecode(data, &fields); err != nil {
		g.failure("branch_protection", path, err)
		return
	}
	if _, ok := fields["required_status_checks"]; !ok {
		g.failure("branch_protection", path, githubAPIError{Kind: "missing_response_field"})
		return
	}
	if err = githubDecode(data, &p); err != nil {
		g.failure("branch_protection", path, err)
		return
	}
	source := "https://api.github.com/" + path
	g.cover("branch_protection", "complete", "Legacy protection observed; bypass eligibility and review satisfaction are not evaluated", source)
	if p.Checks != nil {
		if p.Checks.Contexts == nil && p.Checks.Checks == nil {
			g.cover("branch_protection.checks", "error", "Legacy required check lists are missing", source)
		}
		id := "branch_protection:required_status_checks"
		g.Rules = append(g.Rules, DiscoveryGitHubRule{ID: id, Type: "required_status_checks", Source: source, SourceType: "legacy_branch_protection", Supported: true, StrictChecks: p.Checks.Strict, ObservedAt: g.ObservedAt})
		names := map[string]bool{}
		for _, check := range p.Checks.Checks {
			g.addRequirement(id, check.Context, check.AppID, source)
			names[check.Context] = true
		}
		for _, name := range p.Checks.Contexts {
			if !names[name] {
				g.addRequirement(id, name, nil, source)
			}
		}
	}
	if p.Reviews != nil {
		g.Rules = append(g.Rules, DiscoveryGitHubRule{ID: "branch_protection:pull_request", Type: "pull_request", Source: source, SourceType: "legacy_branch_protection", Supported: true, ReviewCount: p.Reviews.Count, CodeOwnerReview: p.Reviews.CodeOwners, DismissStaleReviews: p.Reviews.Dismiss, LastPushApproval: p.Reviews.LastPush, ObservedAt: g.ObservedAt})
	}
	// Preserve other protection objects as explicit unsupported requirements.
	known := map[string]bool{"url": true, "required_status_checks": true, "required_pull_request_reviews": true, "enforce_admins": true, "allow_force_pushes": true, "allow_deletions": true, "allow_fork_syncing": true}
	keys := []string{}
	for key, value := range fields {
		if !known[key] && !bytes.Equal(value, []byte("null")) {
			var enabled struct {
				Enabled *bool `json:"enabled"`
			}
			_ = json.Unmarshal(value, &enabled)
			if enabled.Enabled == nil || *enabled.Enabled {
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		g.Rules = append(g.Rules, DiscoveryGitHubRule{ID: "branch_protection:" + key, Type: key, Source: source, SourceType: "legacy_branch_protection", ObservedAt: g.ObservedAt})
		g.cover("rule:branch_protection:"+key, "unsupported", "Protection object is recorded but not evaluated: "+key, source)
	}
}

type githubCheckRun struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Head        string `json:"head_sha"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	CompletedAt string `json:"completed_at"`
	StartedAt   string `json:"started_at"`
	App         struct {
		ID   int64  `json:"id"`
		Slug string `json:"slug"`
	} `json:"app"`
}
type githubStatus struct {
	ID        int64  `json:"id"`
	Name      string `json:"context"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Creator   struct {
		Login string `json:"login"`
	} `json:"creator"`
}

func (g *DiscoveryGitHub) collectResults(ctx context.Context, fetch githubFetch, repository, sha, subject string) {
	prefix := "repos/" + repository + "/commits/" + url.PathEscape(sha)
	path := prefix + "/check-runs?filter=latest"
	checks, err := githubPages[githubCheckRun](ctx, fetch, path, "check_runs")
	if err != nil {
		g.failure(subject+".check_runs", path, err)
	} else {
		g.cover(subject+".check_runs", "complete", "Latest check runs observed for explicit revision; workflow event eligibility is not evaluated", "https://api.github.com/"+path)
	}
	for _, check := range checks {
		r := DiscoveryGitHubResult{ID: fmt.Sprintf("check:%s:%d", repository, check.ID), Kind: "check_run", Name: check.Name, Revision: check.Head, Repository: repository, AppID: check.App.ID, AppSlug: check.App.Slug, State: check.Status, Conclusion: check.Conclusion, CompletedAt: check.CompletedAt, UpdatedAt: check.StartedAt, Source: fmt.Sprintf("https://api.github.com/repos/%s/check-runs/%d", repository, check.ID), ObservedAt: g.ObservedAt}
		if r.Name == "" || r.ID == "" || check.ID <= 0 || r.Revision != sha || r.AppID <= 0 {
			g.cover(subject+".result_identity", "error", "Check result has incomplete or mismatched revision/app identity", r.Source)
		}
		g.Results = append(g.Results, r)
	}
	path = prefix + "/statuses"
	statuses, err := githubPages[githubStatus](ctx, fetch, path, "")
	if err != nil {
		g.failure(subject+".statuses", path, err)
	} else {
		g.cover(subject+".statuses", "complete", "Legacy commit statuses observed; creator is not proof of GitHub App identity", "https://api.github.com/"+path)
	}
	for _, status := range statuses {
		g.Results = append(g.Results, DiscoveryGitHubResult{ID: fmt.Sprintf("status:%s:%d", repository, status.ID), Kind: "commit_status", Name: status.Name, Revision: sha, Repository: repository, AppID: 0, Creator: status.Creator.Login, State: status.State, CompletedAt: status.UpdatedAt, UpdatedAt: status.CreatedAt, Source: "https://api.github.com/" + path, ObservedAt: g.ObservedAt})
	}
}

func (g *DiscoveryGitHub) matchRequirements() {
	for _, req := range g.Requirements {
		match := DiscoveryGitHubMatch{RequirementID: req.ID, Revision: g.EvidenceRevision, Status: "missing", ResultIDs: []string{}, Detail: "No result matches this revision and configured producer"}
		if g.EvidenceRevision == "" {
			match.Status = "unknown"
			match.Detail = "Relevant head versus merge-candidate evidence cannot be resolved"
			g.Matches = append(g.Matches, match)
			continue
		}
		latest := map[string]DiscoveryGitHubResult{}
		unidentifiedProducer := false
		for _, r := range g.Results {
			if r.Repository != g.Repository || r.Revision != g.EvidenceRevision || r.Name != req.Context {
				continue
			}
			if githubResultNumber(r.ID) <= 0 {
				unidentifiedProducer = true
				continue
			}
			if req.Producer == "app" && r.AppID != req.AppID {
				if r.Kind == "commit_status" {
					unidentifiedProducer = true
				}
				continue
			}
			if r.Kind == "check_run" && r.AppID <= 0 {
				unidentifiedProducer = true
				continue
			}
			key := r.Kind + ":" + strconv.FormatInt(r.AppID, 10)
			// GitHub's latest filter selects check runs. Duplicate names from the
			// same app can be separate jobs; never erase one job's failure.
			if r.Kind == "check_run" {
				key += ":" + r.ID
			}
			old, ok := latest[key]
			if !ok || r.UpdatedAt > old.UpdatedAt || r.UpdatedAt == old.UpdatedAt && githubResultNumber(r.ID) > githubResultNumber(old.ID) {
				latest[key] = r
			}
		}
		matchedStates := []string{}
		for _, r := range latest {
			match.ResultIDs = append(match.ResultIDs, r.ID)
			matchedStates = append(matchedStates, githubResultState(r, g.ObservedAt))
		}
		sort.Strings(match.ResultIDs)
		if len(matchedStates) > 0 {
			match.Status = "matched"
			match.Detail = "Revision and producer match; this is remote evidence, not a local execution or merge authorization"
			for _, state := range []string{"failed", "unknown", "stale", "pending"} {
				found := false
				for _, s := range matchedStates {
					if s == state {
						found = true
					}
				}
				if found {
					match.Status = state
					break
				}
			}
		}
		if unidentifiedProducer && match.Status != "failed" {
			match.Status = "unknown"
			match.Detail = "A same-name result has incomplete identity or a legacy status has no verifiable App identity"
		}
		if (!g.completeResource(g.EvidenceSubject+".check_runs") || !g.completeResource(g.EvidenceSubject+".statuses")) && match.Status != "failed" {
			match.Status = "unknown"
			match.Detail = "Result collection is incomplete; retained results cannot establish complete matching evidence"
		}
		g.Matches = append(g.Matches, match)
	}
}
func githubResultState(r DiscoveryGitHubResult, observed string) string {
	state := "unknown"
	if r.Kind == "check_run" {
		if r.State == "completed" {
			switch r.Conclusion {
			case "success", "neutral", "skipped":
				state = "matched"
			case "failure", "cancelled", "timed_out", "action_required", "startup_failure":
				state = "failed"
			}
		} else if r.State == "queued" || r.State == "in_progress" || r.State == "waiting" || r.State == "pending" {
			state = "pending"
		}
	} else {
		switch r.State {
		case "success":
			state = "matched"
		case "failure", "error":
			state = "failed"
		case "pending":
			state = "pending"
		}
	}
	if state == "matched" {
		completed, err := time.Parse(time.RFC3339Nano, r.CompletedAt)
		now, nerr := time.Parse(time.RFC3339Nano, observed)
		if err != nil || nerr != nil || completed.After(now.Add(time.Minute)) {
			return "unknown"
		}
		if now.Sub(completed) > 7*24*time.Hour {
			return "stale"
		}
	}
	return state
}

func githubResultNumber(id string) int64 {
	parts := strings.Split(id, ":")
	number, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	return number
}

// ValidateDiscoveryGitHub validates normalized saved observations. An API response
// may add fields; saved observations may not omit identities or invent states.
func ValidateDiscoveryGitHub(g *DiscoveryGitHub) error {
	if g == nil {
		return nil
	}
	oneOf := func(value string, allowed ...string) bool {
		for _, a := range allowed {
			if value == a {
				return true
			}
		}
		return false
	}
	validTime := func(value string) bool { _, err := time.Parse(time.RFC3339Nano, value); return err == nil }
	fail := func(field string) error { return fmt.Errorf("invalid GitHub discovery %s", field) }
	if !validTime(g.ObservedAt) {
		return fail("observation time")
	}
	if g.Repository != "" && !githubSlug.MatchString(g.Repository) {
		return fail("repository")
	}
	if !oneOf(g.TargetSource, "", "default_branch", "comparison_base", "pull_request_base") {
		return fail("target source")
	}
	if !oneOf(g.EvidenceSubject, "unknown", "head", "merge_candidate") {
		return fail("evidence subject")
	}
	if !oneOf(g.LocalCoverage, "unknown", "stale_local_edits", "revision_mismatch", "clean_head_only", "incomplete_local_identity") {
		return fail("local coverage")
	}
	if g.PullRequest < 0 {
		return fail("pull request")
	}
	if g.Rules == nil || g.Requirements == nil || g.Results == nil || g.Matches == nil || g.Coverage == nil {
		return fail("missing arrays")
	}
	if g.EvidenceSubject == "head" && g.EvidenceRevision != g.Head || g.EvidenceSubject == "merge_candidate" && (g.EvidenceRevision == "" || g.EvidenceRevision != g.MergeCandidate) || g.EvidenceSubject == "unknown" && g.EvidenceRevision != "" {
		return fail("evidence revision")
	}
	rules := map[string]bool{}
	for _, r := range g.Rules {
		if r.ID == "" || rules[r.ID] || r.Type == "" || r.Source == "" || r.ReviewCount < 0 || r.RulesetID < 0 || !validTime(r.ObservedAt) {
			return fail("rule")
		}
		rules[r.ID] = true
	}
	requirements := map[string]DiscoveryGitHubRequirement{}
	for _, r := range g.Requirements {
		if r.ID == "" || r.Context == "" || !rules[r.RuleID] || r.Source == "" || !validTime(r.ObservedAt) {
			return fail("requirement")
		}
		if _, exists := requirements[r.ID]; exists {
			return fail("duplicate requirement")
		}
		if !oneOf(r.Producer, "any", "app") || r.Producer == "app" && r.AppID <= 0 || r.Producer == "any" && r.AppID != -1 {
			return fail("required producer")
		}
		requirements[r.ID] = r
	}
	results := map[string]DiscoveryGitHubResult{}
	for _, r := range g.Results {
		if r.ID == "" || r.Source == "" || !validTime(r.ObservedAt) || !githubSlug.MatchString(r.Repository) || !oneOf(r.Kind, "check_run", "commit_status") {
			return fail("result")
		}
		if _, exists := results[r.ID]; exists {
			return fail("duplicate result")
		}
		// Malformed remote identities remain observable facts with error coverage;
		// they cannot appear in a successful match below.
		results[r.ID] = r
	}
	matched := map[string]bool{}
	for _, m := range g.Matches {
		req, ok := requirements[m.RequirementID]
		if !ok || matched[m.RequirementID] || m.ResultIDs == nil || !oneOf(m.Status, "matched", "missing", "failed", "unknown", "stale", "pending") || m.Revision != g.EvidenceRevision {
			return fail("match")
		}
		matched[m.RequirementID] = true
		if m.Status == "matched" && (len(m.ResultIDs) == 0 || m.Revision == "" || !g.completeResource(g.EvidenceSubject+".check_runs") || !g.completeResource(g.EvidenceSubject+".statuses")) {
			return fail("unsupported successful match")
		}
		seen := map[string]bool{}
		for _, id := range m.ResultIDs {
			r, ok := results[id]
			if !ok || seen[id] || r.Repository != g.Repository || r.Revision != m.Revision || r.Name != req.Context || req.Producer == "app" && r.AppID != req.AppID {
				return fail("match identity")
			}
			seen[id] = true
			if m.Status == "matched" && githubResultState(r, g.ObservedAt) != "matched" {
				return fail("match outcome")
			}
		}
	}
	if len(matched) != len(requirements) {
		return fail("missing requirement matches")
	}
	for _, c := range g.Coverage {
		if c.Resource == "" || c.Source == "" || !validTime(c.ObservedAt) || !oneOf(c.Status, "complete", "unavailable", "unsupported", "error", "denied", "inaccessible", "limited", "ambiguous", "unknown") {
			return fail("coverage")
		}
	}
	// Recompute evidence matching so a supplied annotation cannot turn an absent,
	// contradictory, stale or failed observation into a successful match.
	recomputed := *g
	recomputed.Matches = []DiscoveryGitHubMatch{}
	recomputed.matchRequirements()
	a, _ := json.Marshal(g.Matches)
	b, _ := json.Marshal(recomputed.Matches)
	if !bytes.Equal(a, b) {
		return fail("matches contradict recorded facts")
	}
	return nil
}

func githubRuleIdentity(rule githubRule) string {
	return fmt.Sprintf("ruleset:%s:%s:%d:%s", rule.SourceType, rule.Source, rule.ID, rule.Type)
}
