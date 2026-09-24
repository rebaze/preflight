package preflight

import (
	"context"
	"fmt"
	"io"
)

func InspectGitHub(ctx context.Context, d *Discovery, base string, pr int) {
	g := CollectDiscoveryGitHub(ctx, d.Subject.RepoRoot, d.Subject.Head, d.Subject.Branch, base, d.Subject.Dirty, pr)
	if !d.Subject.Current || d.ExitCode != 0 {
		g.LocalCoverage = "incomplete_local_identity"
	}
	d.GitHub = g
	for _, c := range g.Coverage {
		status := "partial"
		if c.Status == "complete" {
			status = "complete"
		}
		d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: "github." + c.Resource, Status: status, Detail: c.Detail})
	}
	// Remote observation can take seconds; facts must not claim to cover edits
	// that happened while reading GitHub. No source content is executed.
	now := InspectLocal(ctx, d.Subject.RepoRoot, base)
	if now.Subject.InputDigest != d.Subject.InputDigest || !now.Subject.Current {
		d.Subject.Current = false
		g.LocalCoverage = "stale_local_edits"
		inspectDiagnostic(d, "source_changed", "Local input changed while reading GitHub; remote results do not cover the current worktree.", "", false)
	}
	FinalizeDiscovery(d)
}
func writeDiscoveryGitHubText(w io.Writer, g *DiscoveryGitHub) {
	fmt.Fprintf(w, "\nGitHub %s — target %s (%s), PR %d\nHead %s; merge candidate %s\nRemote evidence %s (%s); local coverage: %s\n", g.Repository, g.TargetBranch, g.TargetSource, g.PullRequest, g.Head, g.MergeCandidate, g.EvidenceRevision, g.EvidenceSubject, g.LocalCoverage)
	for _, r := range g.Rules {
		fmt.Fprintf(w, "Rule %s [%s] supported=%t reviews=%d codeOwners=%t dismissStale=%t lastPushApproval=%t strictChecks=%t\n  %s observed %s\n", r.ID, r.Type, r.Supported, r.ReviewCount, r.CodeOwnerReview, r.DismissStaleReviews, r.LastPushApproval, r.StrictChecks, r.Source, r.ObservedAt)
	}
	for _, r := range g.Requirements {
		fmt.Fprintf(w, "Required check %s (producer %s, app %d) [%s]\n  %s observed %s\n", r.Context, r.Producer, r.AppID, r.ID, r.Source, r.ObservedAt)
	}
	for _, r := range g.Results {
		fmt.Fprintf(w, "Result %s %s: %s/%s at %s app=%d (%s) creator=%s\n  %s observed %s\n", r.Kind, r.Name, r.State, r.Conclusion, r.Revision, r.AppID, r.AppSlug, r.Creator, r.Source, r.ObservedAt)
	}
	for _, r := range g.Matches {
		fmt.Fprintf(w, "Evidence %s: %s at %s — %s (results %v)\n", r.RequirementID, r.Status, r.Revision, r.Detail, r.ResultIDs)
	}
	for _, c := range g.Coverage {
		fmt.Fprintf(w, "GitHub coverage %s: %s — %s\n  %s observed %s\n", c.Resource, c.Status, c.Detail, c.Source, c.ObservedAt)
	}
}
