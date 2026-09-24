package preflight

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

func ValidateReport(r Report) error {
	if r.SchemaVersion != 1 {
		return fmt.Errorf("unsupported report schemaVersion %d", r.SchemaVersion)
	}
	switch r.Mode {
	case "init", "explain", "prepare", "check", "status":
	default:
		return fmt.Errorf("invalid report mode %q", r.Mode)
	}
	if r.Authority != "local-feedback" {
		return fmt.Errorf("report authority must be local-feedback")
	}
	if r.RunID == "" {
		return fmt.Errorf("report runId is required")
	}
	start, err := time.Parse(time.RFC3339, r.StartedAt)
	if err != nil {
		return fmt.Errorf("invalid startedAt: %w", err)
	}
	end, err := time.Parse(time.RFC3339, r.FinishedAt)
	if err != nil {
		return fmt.Errorf("invalid finishedAt: %w", err)
	}
	if end.Before(start) {
		return fmt.Errorf("finishedAt precedes startedAt")
	}
	if len(r.Findings) == 0 {
		return fmt.Errorf("report must contain findings")
	}
	for _, f := range r.Findings {
		if !validStatus(f.Status) {
			return fmt.Errorf("invalid status %q", f.Status)
		}
		if f.ControlID == "" || f.ReasonCode == "" || f.Message == "" {
			return fmt.Errorf("finding controlId, reasonCode and message are required")
		}
	}
	// The recursive contract check also catches nil arrays generated in Go.
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	var shape Report
	if err = DecodeStrict(b, &shape); err != nil {
		return err
	}
	if r.Summary != summarize(r.Findings) {
		return fmt.Errorf("summary does not match findings")
	}
	want := ExitCode(r.Findings)
	if r.Mode == "status" && !r.Current && want < 2 {
		want = 2
	}
	if r.ExitCode != want {
		return fmt.Errorf("exitCode %d does not match findings (%d)", r.ExitCode, want)
	}
	return nil
}

// WriteReport renders exactly one shared report, validating it before writing.
func WriteReport(w io.Writer, r Report, format string) error {
	if format != "text" && format != "json" {
		return fmt.Errorf("unsupported report format %q", format)
	}
	if err := ValidateReport(r); err != nil {
		return err
	}
	var b bytes.Buffer
	if format == "json" {
		enc := json.NewEncoder(&b)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(r); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(&b, "rebaze Preflight — %s\nAuthority: %s\nRun: %s\nStarted: %s\nFinished: %s\nCurrent: %t\n", r.Mode, r.Authority, r.RunID, r.StartedAt, r.FinishedAt, r.Current)
		fmt.Fprintf(&b, "Repository: %s\nBaseline: %s\nComparison base: %s\nHead: %s\nMerge base: %s\nSnapshot: %s\n", r.Subject.RepoRoot, r.Subject.BaselineCommit, r.Subject.ComparisonBase, r.Subject.Head, r.Subject.MergeBase, r.Subject.SnapshotDigest)
		fmt.Fprintf(&b, "Policy: %s\nProfile: %s (%s)\nPolicy baseline: %s\nEvaluator: %s (%s)\nRunner image: %s\nPreparation: %s\n", r.Policy.PackageDigest, r.Scope.Profile, r.Policy.ProfileDigest, r.Policy.BaselineCommit, r.Runtime.EvaluatorDigest, r.Runtime.EvaluatorVersion, r.Runtime.RunnerImageID, r.Runtime.PreparationInputDigest)
		for _, group := range []struct {
			name   string
			values []string
		}{{"Excluded paths", r.Subject.ExcludedCategories}, {"Changed paths", r.Subject.ChangedPaths}, {"Out-of-scope paths", r.Subject.OutOfScopePaths}, {"Deferred assurance", r.Scope.Deferred}, {"Excluded assurance", r.Scope.Excluded}} {
			fmt.Fprintf(&b, "%s: %s\n", group.name, strings.Join(group.values, ", "))
		}
		for _, f := range r.Findings {
			fmt.Fprintf(&b, "\n%s %s [%s]\n  %s\n", f.Status, f.ControlID, f.ReasonCode, f.Message)
			fmt.Fprintf(&b, "  Owner: %s\n", f.Owner)
			for _, p := range f.Paths {
				fmt.Fprintf(&b, "  Path: %s\n", p)
			}
			for _, e := range f.Evidence {
				fmt.Fprintf(&b, "  Evidence: %s %s %s\n", e.Kind, e.Digest, e.Path)
			}
			for _, a := range f.NextActions {
				fmt.Fprintf(&b, "  Next: %s\n  Command: %q", a.Description, a.Command)
				for _, arg := range a.Args {
					fmt.Fprintf(&b, " %q", arg)
				}
				fmt.Fprintln(&b)
			}
		}
		fmt.Fprintf(&b, "\nSummary: pass=%d fail=%d missing=%d error=%d review_required=%d deferred=%d not_applicable=%d\nExit code: %d\nLocal feedback; no release authorization.\n", r.Summary.Pass, r.Summary.Fail, r.Summary.Missing, r.Summary.Error, r.Summary.ReviewRequired, r.Summary.Deferred, r.Summary.NotApplicable, r.ExitCode)
	}
	_, err := w.Write(b.Bytes())
	return err
}
