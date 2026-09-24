package preflight

import (
	"fmt"
	"io"
)

func writeDiscoveryComparisonText(w io.Writer, c *DiscoveryComparison) {
	fmt.Fprintf(w, "\nComparison: compatible=%t — %s\nPrevious %s; current %s; previous evidence stale=%t\n", c.Compatible, c.Reason, c.PreviousObservedAt, c.CurrentObservedAt, c.PreviousEvidenceStale)
	for _, v := range c.InputChanges {
		fmt.Fprintf(w, "Input %s %s: %s %s mode=%d → %s %s mode=%d\n", v.Change, v.Path, v.BeforeKind, v.BeforeDigest, v.BeforeMode, v.AfterKind, v.AfterDigest, v.AfterMode)
	}
	for _, v := range c.RequirementChanges {
		fmt.Fprintf(w, "Requirement %s %s %s: %s → %s (%s)\n", v.Kind, v.ID, v.Change, v.Before, v.After, v.Source)
	}
	for _, v := range c.NewFindings {
		fmt.Fprintf(w, "New %s [%s]: %s (%s)\n", v.ID, v.Status, v.Summary, v.Source)
	}
	for _, v := range c.ResolvedFindings {
		fmt.Fprintf(w, "Resolved %s [%s]: %s (%s)\n", v.ID, v.Status, v.Summary, v.Source)
	}
	for _, v := range c.Evidence {
		fmt.Fprintf(w, "Evidence %s: %s at %s → %s at %s; comparable=%t\n  Last observed passing: %s; first observed failing: %s\n  %s\n", v.RequirementID, v.BeforeStatus, v.BeforeRevision, v.AfterStatus, v.AfterRevision, v.Comparable, v.LastObservedPassing, v.FirstObservedFailing, v.Detail)
	}
	for _, v := range c.Structural {
		fmt.Fprintf(w, "Structural %s [%s] %s:%d → %s:%d: %s → %s (requirement %s)\n  %s\n", v.Kind, v.Verification, v.Path, v.BeforeLine, v.Path, v.AfterLine, v.Before, v.After, v.RequirementID, v.Detail)
	}
	for _, v := range c.ResolvedStructural {
		fmt.Fprintf(w, "Restored %s [%s] %s:%d: %s → %s\n", v.Kind, v.Verification, v.Path, v.AfterLine, v.Before, v.After)
	}
	for _, v := range c.Notes {
		fmt.Fprintf(w, "Comparison note: %s\n", v)
	}
}
