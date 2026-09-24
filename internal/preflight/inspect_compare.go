package preflight

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DiscoveryComparison struct {
	Compatible            bool                          `json:"compatible"`
	Reason                string                        `json:"reason"`
	PreviousObservedAt    string                        `json:"previousObservedAt"`
	CurrentObservedAt     string                        `json:"currentObservedAt"`
	PreviousEvidenceStale bool                          `json:"previousEvidenceStale"`
	InputChanges          []DiscoveryInputChange        `json:"inputChanges"`
	RequirementChanges    []DiscoveryValueChange        `json:"requirementChanges"`
	NewFindings           []DiscoveryFinding            `json:"newFindings"`
	ResolvedFindings      []DiscoveryFinding            `json:"resolvedFindings"`
	Evidence              []DiscoveryEvidenceTransition `json:"evidence"`
	Structural            []DiscoveryCIExplanation      `json:"structural"`
	ResolvedStructural    []DiscoveryCIExplanation      `json:"resolvedStructural"`
	Notes                 []string                      `json:"notes"`
}
type DiscoveryInputChange struct {
	Path         string `json:"path"`
	Change       string `json:"change"`
	BeforeKind   string `json:"beforeKind"`
	AfterKind    string `json:"afterKind"`
	BeforeDigest string `json:"beforeDigest"`
	AfterDigest  string `json:"afterDigest"`
	BeforeMode   uint32 `json:"beforeMode"`
	AfterMode    uint32 `json:"afterMode"`
}
type DiscoveryValueChange struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Change string `json:"change"`
	Before string `json:"before"`
	After  string `json:"after"`
	Source string `json:"source"`
}
type DiscoveryFinding struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Source  string `json:"source"`
}
type DiscoveryEvidenceTransition struct {
	RequirementID        string `json:"requirementId"`
	BeforeStatus         string `json:"beforeStatus"`
	AfterStatus          string `json:"afterStatus"`
	BeforeRevision       string `json:"beforeRevision"`
	AfterRevision        string `json:"afterRevision"`
	BeforeObservedAt     string `json:"beforeObservedAt"`
	AfterObservedAt      string `json:"afterObservedAt"`
	Comparable           bool   `json:"comparable"`
	LastObservedPassing  string `json:"lastObservedPassing"`
	FirstObservedFailing string `json:"firstObservedFailing"`
	Detail               string `json:"detail"`
}

// SaveDiscovery creates one new private observation. It neither overwrites old
// evidence nor initializes/migrates policy state. The parent must already exist.
func SaveDiscovery(name string, d Discovery) error {
	if err := ValidateDiscovery(d); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > DiscoveryMaxBytes {
		return fmt.Errorf("discovery exceeds 16 MiB")
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return fmt.Errorf("observation parent must already exist: %w", err)
	}
	dest := filepath.Join(parent, filepath.Base(abs))
	if d.Subject.RepoRoot == "" {
		return fmt.Errorf("cannot save without an identified source directory")
	}
	repo, err := filepath.EvalSymlinks(d.Subject.RepoRoot)
	if err != nil {
		return fmt.Errorf("cannot resolve source directory: %w", err)
	}
	if inspectWithin(repo, dest) {
		return fmt.Errorf("observation must be outside the inspected checkout")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if common, e := snapshotGitRead(ctx, repo, "rev-parse", "--path-format=absolute", "--git-common-dir"); e == nil {
		commonPath, e := filepath.EvalSymlinks(strings.TrimSpace(string(common)))
		if e != nil {
			return fmt.Errorf("cannot resolve Git metadata boundary")
		}
		if inspectWithin(commonPath, dest) {
			return fmt.Errorf("observation must be outside Git metadata")
		}
	}
	if _, e := snapshotGitRead(ctx, parent, "rev-parse", "--git-dir"); e == nil {
		return fmt.Errorf("private observations must not be saved in a Git checkout or Git metadata")
	}
	r, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := r.OpenFile(filepath.Base(dest), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("observation must be a new file: %w", err)
	}
	written := false
	defer func() {
		f.Close()
		if !written {
			_ = r.Remove(filepath.Base(dest))
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	written = true
	return nil
}

func inspectWithin(root, name string) bool {
	rel, err := filepath.Rel(root, name)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// LoadDiscovery checks syntax and identities, not producer authenticity. A local
// user can fabricate a valid observation; loading does not make it attested.
func LoadDiscovery(name string) (Discovery, error) {
	var d Discovery
	abs, err := filepath.Abs(name)
	if err != nil {
		return d, err
	}
	r, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return d, err
	}
	defer r.Close()
	base := filepath.Base(abs)
	before, err := r.Lstat(base)
	if err != nil {
		return d, err
	}
	if !before.Mode().IsRegular() || before.Size() > DiscoveryMaxBytes {
		return d, fmt.Errorf("observation must be a regular file of at most 16 MiB")
	}
	f, err := r.Open(base)
	if err != nil {
		return d, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return d, fmt.Errorf("observation changed while opening")
	}
	b, err := io.ReadAll(io.LimitReader(f, (DiscoveryMaxBytes)+1))
	if err != nil {
		return d, err
	}
	after, err := r.Lstat(base)
	if err != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() || before.Size() != int64(len(b)) || before.ModTime() != after.ModTime() {
		return d, fmt.Errorf("observation changed while reading")
	}
	return DecodeDiscovery(b)
}

func CompareDiscovery(previous, current Discovery) DiscoveryComparison {
	c := DiscoveryComparison{PreviousObservedAt: previous.ObservedAt, CurrentObservedAt: current.ObservedAt, InputChanges: []DiscoveryInputChange{}, RequirementChanges: []DiscoveryValueChange{}, NewFindings: []DiscoveryFinding{}, ResolvedFindings: []DiscoveryFinding{}, Evidence: []DiscoveryEvidenceTransition{}, Structural: []DiscoveryCIExplanation{}, ResolvedStructural: []DiscoveryCIExplanation{}, Notes: []string{}}
	if err := ValidateDiscovery(previous); err != nil {
		c.Reason = "Previous observation is invalid: " + err.Error()
		return c
	}
	if err := ValidateDiscovery(current); err != nil {
		c.Reason = "Current observation is invalid: " + err.Error()
		return c
	}
	if previous.Subject.RepositoryID == "" || previous.Subject.RepositoryID != current.Subject.RepositoryID {
		c.Reason = "Repository identities are incompatible"
		return c
	}
	if previous.Subject.WorktreeID == "" || previous.Subject.WorktreeID != current.Subject.WorktreeID {
		c.Reason = "Worktree identities are incompatible"
		return c
	}
	if previous.Subject.Branch != current.Subject.Branch {
		c.Reason = "Branch subjects are incompatible"
		return c
	}
	if previous.Subject.BaseRef != current.Subject.BaseRef {
		c.Reason = "Comparison targets differ; collect observations for the same target"
		return c
	}
	beforeTime, _ := time.Parse(time.RFC3339Nano, previous.ObservedAt)
	afterTime, _ := time.Parse(time.RFC3339Nano, current.ObservedAt)
	if afterTime.Before(beforeTime) {
		c.Reason = "Current observation predates the selected previous observation"
		return c
	}
	c.Compatible = true
	c.Reason = "Comparable repository, worktree, branch and comparison target"
	inspectCompareGitHub(previous.GitHub, current.GitHub, &c)
	if !c.Compatible {
		return c
	}
	c.PreviousEvidenceStale = previous.Subject.InputDigest != current.Subject.InputDigest || !previous.Subject.Current || !current.Subject.Current
	if previous.Subject.BaseCommit != current.Subject.BaseCommit || previous.Subject.MergeBase != current.Subject.MergeBase {
		c.Notes = append(c.Notes, "Comparison commit or merge base changed; input differences include that changed context.")
	}
	if previous.ExitCode != 0 || current.ExitCode != 0 {
		c.Notes = append(c.Notes, "At least one observation has incomplete coverage; absence from it is not evidence of resolution.")
	}
	before, after := map[string]DiscoveryInput{}, map[string]DiscoveryInput{}
	for _, v := range previous.Inputs {
		before[v.Path] = v
	}
	for _, v := range current.Inputs {
		after[v.Path] = v
	}
	for _, p := range inspectUnionKeys(before, after) {
		a, aok := before[p]
		b, bok := after[p]
		if a == b {
			continue
		}
		change := "modified"
		if !aok {
			change = "added"
		}
		if !bok {
			change = "removed"
		}
		if (!aok && previous.ExitCode != 0) || (!bok && current.ExitCode != 0) || a.Kind == "unsupported" || a.Kind == "excluded" || b.Kind == "unsupported" || b.Kind == "excluded" {
			change = "unknown"
		}
		c.InputChanges = append(c.InputChanges, DiscoveryInputChange{Path: p, Change: change, BeforeKind: a.Kind, AfterKind: b.Kind, BeforeDigest: a.Digest, AfterDigest: b.Digest, BeforeMode: a.Mode, AfterMode: b.Mode})
	}
	oldFindings, newFindings := inspectLocalFindings(previous), inspectLocalFindings(current)
	for _, id := range inspectUnionKeys(oldFindings, newFindings) {
		a, aok := oldFindings[id]
		b, bok := newFindings[id]
		if bok && (!aok || a.Status != b.Status) {
			c.NewFindings = append(c.NewFindings, b)
		}
		if aok && (!bok || a.Status != b.Status) && current.ExitCode == 0 {
			c.ResolvedFindings = append(c.ResolvedFindings, a)
		}
	}
	inspectCompareClaims(previous, current, &c)
	if inspectWorkflowCoverage(previous) && inspectWorkflowCoverage(current) {
		c.Structural = ExplainStructuralCI(previous.Sources, current.Sources, previous.GitHub)
	} else {
		c.Notes = append(c.Notes, "Structural CI explanation is unavailable because a workflow source or local inventory was not completely captured.")
	}
	c.Notes = append(c.Notes, "Observations record when evidence was seen; they do not establish when a failure originated or authenticate its producer.")
	return c
}

func inspectUnionKeys[V any](a, b map[string]V) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func inspectLocalFindings(d Discovery) map[string]DiscoveryFinding {
	out := map[string]DiscoveryFinding{}
	for _, diagnostic := range d.Diagnostics {
		id := "diagnostic:" + diagnostic.Code + ":" + diagnostic.Path
		out[id] = DiscoveryFinding{ID: id, Kind: "diagnostic", Status: diagnostic.Severity, Summary: diagnostic.Message, Source: diagnostic.Path}
	}
	return out
}
func inspectCompareClaims(previous, current Discovery, c *DiscoveryComparison) {
	before, after := map[string]DiscoveryClaim{}, map[string]DiscoveryClaim{}
	for _, v := range previous.Claims {
		if v.Origin == "configured" || v.Origin == "documented" {
			before[v.ID] = v
		}
	}
	for _, v := range current.Claims {
		if v.Origin == "configured" || v.Origin == "documented" {
			after[v.ID] = v
		}
	}
	for _, id := range inspectUnionKeys(before, after) {
		a, aok := before[id]
		b, bok := after[id]
		a.Verification = ""
		b.Verification = ""
		av, _ := json.Marshal(a)
		bv, _ := json.Marshal(b)
		if bytes.Equal(av, bv) {
			continue
		}
		change := "modified"
		if !aok {
			change = "added"
		}
		if !bok {
			change = "removed"
		}
		if (!aok && previous.ExitCode != 0) || (!bok && current.ExitCode != 0) {
			change = "unknown"
		}
		source := ""
		refs := b.Sources
		if !bok {
			refs = a.Sources
		}
		if len(refs) > 0 {
			source = refs[0].Path
		}
		c.RequirementChanges = append(c.RequirementChanges, DiscoveryValueChange{ID: id, Kind: "claim", Change: change, Before: a.Summary, After: b.Summary, Source: source})
	}
}

func inspectCompareGitHub(previous, current *DiscoveryGitHub, c *DiscoveryComparison) {
	if previous == nil && current == nil {
		return
	}
	if previous == nil || current == nil {
		c.Notes = append(c.Notes, "GitHub was not observed on both invocations; remote requirements, findings and evidence cannot be compared.")
		return
	}
	if previous.Repository != "" && current.Repository != "" && previous.Repository != current.Repository || previous.TargetBranch != "" && current.TargetBranch != "" && previous.TargetBranch != current.TargetBranch || previous.PullRequest != current.PullRequest || previous.HeadRepository != "" && current.HeadRepository != "" && previous.HeadRepository != current.HeadRepository {
		c.Compatible = false
		c.Reason = "GitHub repository, pull request, head repository or target branch changed"
		return
	}
	beforeRequirements, afterRequirements := inspectRequirementValues(previous), inspectRequirementValues(current)
	for _, id := range inspectUnionKeys(beforeRequirements, afterRequirements) {
		a, aok := beforeRequirements[id]
		b, bok := afterRequirements[id]
		if a == b {
			continue
		}
		change := "modified"
		if !aok {
			change = "added"
		}
		if !bok {
			change = "removed"
		}
		if (!aok && !inspectRequirementsComplete(previous)) || (!bok && !inspectRequirementsComplete(current)) {
			change = "unknown"
		}
		kind, source := b.Kind, b.Source
		if !bok {
			kind, source = a.Kind, a.Source
		}
		c.RequirementChanges = append(c.RequirementChanges, DiscoveryValueChange{ID: id, Kind: kind, Change: change, Before: a.After, After: b.After, Source: source})
	}
	beforeFindings, afterFindings := inspectGitHubFindings(previous), inspectGitHubFindings(current)
	for _, id := range inspectUnionKeys(beforeFindings, afterFindings) {
		a, aok := beforeFindings[id]
		b, bok := afterFindings[id]
		if bok && (!aok || a.Status != b.Status) {
			c.NewFindings = append(c.NewFindings, b)
		}
		if aok && (!bok || a.Status != b.Status) && inspectRemoteResolution(id, current) {
			c.ResolvedFindings = append(c.ResolvedFindings, a)
		}
	}
	beforeReq, afterReq := map[string]DiscoveryGitHubRequirement{}, map[string]DiscoveryGitHubRequirement{}
	for _, req := range previous.Requirements {
		beforeReq[req.ID] = req
	}
	for _, req := range current.Requirements {
		afterReq[req.ID] = req
	}
	for _, id := range inspectUnionKeys(beforeReq, afterReq) {
		a, aok := beforeReq[id]
		b, bok := afterReq[id]
		if !aok || !bok {
			continue
		}
		beforeState, beforeProducer := inspectComparableEvidence(previous, a)
		afterState, afterProducer := inspectComparableEvidence(current, b)
		e := DiscoveryEvidenceTransition{RequirementID: id, BeforeStatus: beforeState, AfterStatus: afterState, BeforeRevision: previous.EvidenceRevision, AfterRevision: current.EvidenceRevision, BeforeObservedAt: previous.ObservedAt, AfterObservedAt: current.ObservedAt, Detail: "Remote observations only; cause and failure origin remain unknown. Local edits are not covered by these results."}
		a.ObservedAt = ""
		b.ObservedAt = ""
		oldTime, err1 := time.Parse(time.RFC3339Nano, previous.ObservedAt)
		newTime, err2 := time.Parse(time.RFC3339Nano, current.ObservedAt)
		e.Comparable = a == b && previous.EvidenceSubject == current.EvidenceSubject && previous.EvidenceSubject != "unknown" && beforeProducer != "" && beforeProducer == afterProducer && err1 == nil && err2 == nil && !newTime.Before(oldTime)
		if e.Comparable && beforeState == "passing" && afterState == "failing" {
			e.LastObservedPassing = previous.ObservedAt
			e.FirstObservedFailing = current.ObservedAt
			e.Detail = "Last passing and first failing within these two comparable remote observations; this bounds an observed transition, not the failure's origin or cause. Local edits are not covered."
		}
		if e.Comparable && beforeState == "failing" && afterState == "passing" {
			e.LastObservedPassing = current.ObservedAt
			e.FirstObservedFailing = previous.ObservedAt
			e.Detail = "Failure in the earlier observation and passing in the later comparable remote observation; the cause of either transition is unknown. Local edits are not covered."
		}
		c.Evidence = append(c.Evidence, e)
	}
}

func inspectRequirementsComplete(g *DiscoveryGitHub) bool {
	if !g.completeResource("rulesets") || !g.completeResource("branch_protection") {
		return false
	}
	for _, coverage := range g.Coverage {
		if (strings.HasPrefix(coverage.Resource, "rule:") || strings.HasPrefix(coverage.Resource, "rulesets.") || strings.HasPrefix(coverage.Resource, "branch_protection.")) && coverage.Status != "complete" {
			return false
		}
	}
	return true
}
func inspectRequirementValues(g *DiscoveryGitHub) map[string]DiscoveryValueChange {
	out := map[string]DiscoveryValueChange{}
	for _, req := range g.Requirements {
		req.ObservedAt = ""
		data, _ := json.Marshal(req)
		id := "check:" + req.ID
		out[id] = DiscoveryValueChange{ID: id, Kind: "required_check", After: string(data), Source: req.Source}
	}
	for _, rule := range g.Rules {
		rule.ObservedAt = ""
		data, _ := json.Marshal(rule)
		id := "rule:" + rule.ID
		out[id] = DiscoveryValueChange{ID: id, Kind: "rule", After: string(data), Source: rule.Source}
	}
	return out
}

func inspectGitHubFindings(g *DiscoveryGitHub) map[string]DiscoveryFinding {
	out := map[string]DiscoveryFinding{}
	for _, m := range g.Matches {
		if m.Status != "matched" {
			id := "github.match:" + m.RequirementID
			source := ""
			for _, r := range g.Requirements {
				if r.ID == m.RequirementID {
					source = r.Source
				}
			}
			out[id] = DiscoveryFinding{ID: id, Kind: "remote_check", Status: m.Status, Summary: m.Detail, Source: source}
		}
	}
	for _, coverage := range g.Coverage {
		if coverage.Status != "complete" {
			id := "github.coverage:" + coverage.Resource
			out[id] = DiscoveryFinding{ID: id, Kind: "remote_coverage", Status: coverage.Status, Summary: coverage.Detail, Source: coverage.Source}
		}
	}
	if g.LocalCoverage != "clean_head_only" {
		out["github.local_coverage"] = DiscoveryFinding{ID: "github.local_coverage", Kind: "remote_freshness", Status: g.LocalCoverage, Summary: "Remote results do not establish coverage of the current local input.", Source: g.Repository}
	}
	return out
}

func inspectRemoteResolution(id string, g *DiscoveryGitHub) bool {
	if strings.HasPrefix(id, "github.match:") {
		req := strings.TrimPrefix(id, "github.match:")
		for _, m := range g.Matches {
			if m.RequirementID == req {
				return m.Status == "matched"
			}
		}
		return false
	}
	if strings.HasPrefix(id, "github.coverage:") {
		return g.completeResource(strings.TrimPrefix(id, "github.coverage:"))
	}
	return id == "github.local_coverage" && g.LocalCoverage == "clean_head_only"
}

// Matching names/status labels alone are not comparable evidence. Re-read the
// recorded revision, actual producer, conclusion and coverage before describing
// an observation as passing. Neutral/skipped checks never become test passes.
func inspectComparableEvidence(g *DiscoveryGitHub, req DiscoveryGitHubRequirement) (string, string) {
	if g.EvidenceRevision == "" || !g.completeResource(g.EvidenceSubject+".check_runs") || !g.completeResource(g.EvidenceSubject+".statuses") {
		return "unknown", ""
	}
	var match *DiscoveryGitHubMatch
	for i := range g.Matches {
		if g.Matches[i].RequirementID == req.ID {
			match = &g.Matches[i]
			break
		}
	}
	if match == nil || len(match.ResultIDs) == 0 || match.Revision != g.EvidenceRevision {
		return "unknown", ""
	}
	if match.Status != "matched" && match.Status != "failed" {
		return match.Status, ""
	}
	repository := g.Repository
	state := "passing"
	producers := []string{}
	for _, id := range match.ResultIDs {
		var result *DiscoveryGitHubResult
		for i := range g.Results {
			if g.Results[i].ID == id {
				result = &g.Results[i]
				break
			}
		}
		if result == nil || result.Name != req.Context || result.Revision != g.EvidenceRevision || result.Repository != repository || result.Source == "" || req.Producer == "app" && result.AppID != req.AppID {
			return "unknown", ""
		}
		r := *result
		actual := githubResultState(r, g.ObservedAt)
		if actual == "failed" {
			state = "failing"
		} else if actual != "matched" || r.Kind == "check_run" && r.Conclusion != "success" || r.Kind == "commit_status" && r.State != "success" {
			return "unknown", ""
		}
		if r.Kind == "check_run" && r.AppID <= 0 || r.Kind == "commit_status" && r.Creator == "" {
			return "unknown", ""
		}
		producers = append(producers, fmt.Sprintf("%s:%s:%d:%s", r.Kind, r.Repository, r.AppID, r.Creator))
	}
	if state == "passing" && match.Status != "matched" || state == "failing" && match.Status != "failed" {
		return "unknown", ""
	}
	sort.Strings(producers)
	return state, strings.Join(producers, "|")
}

func inspectWorkflowCoverage(d Discovery) bool {
	for _, v := range d.Coverage {
		if !strings.HasPrefix(v.Collector, "github.") && (v.Status == "partial" || v.Status == "unavailable") {
			return false
		}
	}
	sources := map[string]DiscoverySource{}
	inputs := map[string]DiscoveryInput{}
	for _, input := range d.Inputs {
		inputs[input.Path] = input
	}
	for _, s := range d.Sources {
		sources[s.Path] = s
		if s.Kind == "workflow" {
			input, ok := inputs[s.Path]
			if !ok || input.Kind != "file" || input.Digest != s.Digest || input.Mode != s.Mode {
				return false
			}
		}
	}
	for _, input := range d.Inputs {
		if inspectSourceKind(input.Path) == "workflow" {
			s, ok := sources[input.Path]
			if input.Kind == "missing" {
				continue
			}
			if input.Kind != "file" || !ok || s.Digest != input.Digest {
				return false
			}
		}
	}
	return d.Subject.Current
}

// ResolveStructuralCI resolves earlier removals only from fresh positive source
// evidence. An absent new delta does not mean an earlier removal was restored.
func ResolveStructuralCI(previous []DiscoveryCIExplanation, current Discovery) []DiscoveryCIExplanation {
	out := []DiscoveryCIExplanation{}
	if !inspectWorkflowCoverage(current) {
		return out
	}
	sources := map[string]DiscoverySource{}
	counts := map[string]int{}
	allJobsKnown := true
	for _, s := range current.Sources {
		if s.Kind != "workflow" {
			continue
		}
		sources[s.Path] = s
		shape := inspectCIShape(s.Content)
		if !shape.jobsKnown {
			allJobsKnown = false
		}
		for _, j := range shape.jobs {
			if j.literal {
				counts[j.name]++
			} else {
				allJobsKnown = false
			}
		}
	}
	for _, prior := range previous {
		s, ok := sources[prior.Path]
		if !ok {
			continue
		}
		restored := false
		line := 1
		shape := inspectCIShape(s.Content)
		switch prior.Kind {
		case "workflow_deleted":
			restored = s.Digest == prior.Before
		case "pr_trigger_removed":
			restored = shape.triggerKnown && shape.pr
			line = shape.triggerLine
		case "required_job_deleted":
			if !allJobsKnown || !shape.jobsKnown || current.GitHub == nil {
				continue
			}
			for _, job := range shape.jobs {
				if !job.literal || counts[job.name] != 1 || "job "+job.id+" ("+job.name+")" != prior.Before {
					continue
				}
				for _, req := range current.GitHub.Requirements {
					if req.ID != prior.RequirementID || req.Context != job.name || req.Producer != "app" || req.AppID <= 0 {
						continue
					}
					for _, result := range current.GitHub.Results {
						if result.Kind == "check_run" && result.Name == req.Context && result.AppID == req.AppID && result.AppSlug == "github-actions" {
							restored = true
							line = job.line
						}
					}
				}
			}
		}
		if restored {
			r := prior
			r.Before = prior.After
			r.After = prior.Before
			r.BeforeLine = prior.AfterLine
			r.AfterLine = line
			r.Verification = "verified"
			r.Detail = "Previously removed configuration is positively present again in the current captured source. This is structural restoration, not an executed check or evidence that a remote failure is fixed."
			out = append(out, r)
		}
	}
	return out
}

func ValidateDiscoveryComparison(c *DiscoveryComparison) error {
	if c == nil {
		return nil
	}
	before, err := time.Parse(time.RFC3339Nano, c.PreviousObservedAt)
	if err != nil {
		return fmt.Errorf("invalid comparison previous time")
	}
	after, err := time.Parse(time.RFC3339Nano, c.CurrentObservedAt)
	if err != nil {
		return fmt.Errorf("invalid comparison current time")
	}
	if c.Reason == "" || c.Compatible && after.Before(before) {
		return fmt.Errorf("invalid comparison reason or chronology")
	}
	if c.InputChanges == nil || c.RequirementChanges == nil || c.NewFindings == nil || c.ResolvedFindings == nil || c.Evidence == nil || c.Structural == nil || c.ResolvedStructural == nil || c.Notes == nil {
		return fmt.Errorf("comparison arrays must not be null")
	}
	if !c.Compatible && (len(c.InputChanges)+len(c.RequirementChanges)+len(c.NewFindings)+len(c.ResolvedFindings)+len(c.Evidence)+len(c.Structural)+len(c.ResolvedStructural) != 0) {
		return fmt.Errorf("incompatible observations cannot claim comparisons")
	}
	seen := map[string]bool{}
	for _, v := range c.InputChanges {
		if snapshotValidPath(v.Path) != nil || seen[v.Path] || !discoveryEnum(v.Change, "added", "modified", "removed", "unknown") || !discoveryEnum(v.BeforeKind, "", "file", "missing", "unsupported", "excluded") || !discoveryEnum(v.AfterKind, "", "file", "missing", "unsupported", "excluded") {
			return fmt.Errorf("invalid comparison input change")
		}
		seen[v.Path] = true
	}
	seen = map[string]bool{}
	for _, v := range c.RequirementChanges {
		if v.ID == "" || seen[v.ID] || !discoveryEnum(v.Kind, "claim", "required_check", "rule") || !discoveryEnum(v.Change, "added", "modified", "removed", "unknown") {
			return fmt.Errorf("invalid requirement change")
		}
		seen[v.ID] = true
	}
	for _, findings := range [][]DiscoveryFinding{c.NewFindings, c.ResolvedFindings} {
		seen = map[string]bool{}
		for _, v := range findings {
			if v.ID == "" || seen[v.ID] || v.Summary == "" || !discoveryEnum(v.Kind, "diagnostic", "remote_check", "remote_coverage", "remote_freshness") {
				return fmt.Errorf("invalid compared finding")
			}
			seen[v.ID] = true
			if !discoveryEnum(v.Status, "warning", "error", "matched", "failed", "unknown", "stale", "pending", "missing", "denied", "inaccessible", "limited", "unsupported", "ambiguous", "stale_local_edits", "revision_mismatch", "incomplete_local_identity", "unavailable") {
				return fmt.Errorf("invalid compared finding status")
			}
		}
	}
	seen = map[string]bool{}
	for _, v := range c.Evidence {
		if v.RequirementID == "" || seen[v.RequirementID] || v.Detail == "" || !discoveryEnum(v.BeforeStatus, "passing", "failing", "unknown", "stale", "pending", "missing") || !discoveryEnum(v.AfterStatus, "passing", "failing", "unknown", "stale", "pending", "missing") {
			return fmt.Errorf("invalid evidence transition")
		}
		seen[v.RequirementID] = true
		oldTime, oldErr := time.Parse(time.RFC3339Nano, v.BeforeObservedAt)
		newTime, newErr := time.Parse(time.RFC3339Nano, v.AfterObservedAt)
		if oldErr != nil || newErr != nil || v.Comparable && newTime.Before(oldTime) {
			return fmt.Errorf("invalid evidence observation time")
		}
		if v.Comparable && (v.BeforeRevision == "" || v.AfterRevision == "") {
			return fmt.Errorf("comparable evidence needs both revisions")
		}
		if v.LastObservedPassing != "" || v.FirstObservedFailing != "" {
			if !v.Comparable {
				return fmt.Errorf("incomparable evidence cannot establish observed passing or failing")
			}
			forward := v.BeforeStatus == "passing" && v.AfterStatus == "failing" && v.LastObservedPassing == v.BeforeObservedAt && v.FirstObservedFailing == v.AfterObservedAt
			reverse := v.BeforeStatus == "failing" && v.AfterStatus == "passing" && v.LastObservedPassing == v.AfterObservedAt && v.FirstObservedFailing == v.BeforeObservedAt
			if !forward && !reverse {
				return fmt.Errorf("observed interval requires comparable passing/failing transition")
			}
		}
	}
	for _, values := range [][]DiscoveryCIExplanation{c.Structural, c.ResolvedStructural} {
		for _, v := range values {
			if snapshotValidPath(v.Path) != nil || !discoveryEnum(v.Kind, "workflow_deleted", "pr_trigger_removed", "required_job_deleted", "unsupported_structure") || !discoveryEnum(v.Verification, "verified", "unverified") || v.BeforeLine < 0 || v.AfterLine < 0 || v.Before == "" || v.After == "" || v.Detail == "" {
				return fmt.Errorf("invalid structural explanation")
			}
			if v.Kind == "unsupported_structure" && v.Verification != "unverified" {
				return fmt.Errorf("unsupported structural semantics cannot be verified")
			}
		}
	}
	return nil
}
