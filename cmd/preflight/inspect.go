package main

import (
	"context"
	"flag"
	"fmt"
	pf "github.com/rebaze/preflight/internal/preflight"
	"io"
	"time"
)

func runInspect(args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("inspect", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	output := f.String("output", "", "save a new private observation outside all checkouts")
	compare := f.String("compare", "", "compare a prior private observation")
	repo := f.String("repo", ".", "repository or directory to observe read-only")
	github := f.Bool("github", false, "explicit read-only GitHub inspection using existing gh authentication")
	pr := f.Int("pr", 0, "select an open pull request (requires --github)")
	base := f.String("base", "", "comparison ref; never selects trusted policy")
	format := f.String("format", "text", "text or json")
	fail := func(message string) int {
		d := pf.NewDiscovery()
		d.Diagnostics = append(d.Diagnostics, pf.DiscoveryDiagnostic{Code: "invalid_arguments", Severity: "error", Message: message})
		pf.FinalizeDiscovery(&d)
		selected := "text"
		for i, a := range args {
			if a == "--format=json" || (a == "--format" && i+1 < len(args) && args[i+1] == "json") {
				selected = "json"
			}
		}
		if err := pf.WriteDiscovery(out, d, selected); err != nil {
			fmt.Fprintln(diagnostic, err)
		}
		return 3
	}
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return fail(err.Error())
	}
	if f.NArg() != 0 {
		return fail("unexpected positional arguments")
	}
	if *format != "text" && *format != "json" {
		return fail("format must be text or json")
	}
	if *pr < 0 || (*pr > 0 && !*github) {
		return fail("--pr requires --github and a positive pull request number")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	d := pf.InspectLocal(ctx, *repo, *base)
	if *github && d.Subject.RepoRoot != "" {
		pf.InspectGitHub(ctx, &d, *base, *pr)
	}
	if *compare != "" {
		previous, err := pf.LoadDiscovery(*compare)
		if err != nil {
			d.Diagnostics = append(d.Diagnostics, pf.DiscoveryDiagnostic{Code: "comparison_read_error", Severity: "error", Message: err.Error()})
		} else {
			comparison := pf.CompareDiscovery(previous, d)
			if previous.Comparison != nil && comparison.Compatible {
				comparison.ResolvedStructural = pf.ResolveStructuralCI(previous.Comparison.Structural, d)
				resolved := map[string]bool{}
				for _, s := range comparison.ResolvedStructural {
					resolved[s.Path+":"+s.Kind+":"+s.RequirementID] = true
				}
				seen := map[string]bool{}
				for _, s := range comparison.Structural {
					seen[s.Path+":"+s.Kind+":"+s.RequirementID] = true
				}
				for _, s := range previous.Comparison.Structural {
					key := s.Path + ":" + s.Kind + ":" + s.RequirementID
					if !resolved[key] && !seen[key] {
						s.Verification = "unverified"
						s.Detail = "Previous structural finding remains unresolved; current observation does not establish restoration."
						comparison.Structural = append(comparison.Structural, s)
					}
				}
			}
			d.Comparison = &comparison
			if !comparison.Compatible {
				d.Coverage = append(d.Coverage, pf.DiscoveryCoverage{Collector: "comparison", Status: "partial", Detail: comparison.Reason})
			}
		}
	}
	pf.FinalizeDiscovery(&d)
	if *output != "" {
		if err := pf.SaveDiscovery(*output, d); err != nil {
			d.Diagnostics = append(d.Diagnostics, pf.DiscoveryDiagnostic{Code: "observation_write_error", Severity: "error", Message: err.Error()})
			pf.FinalizeDiscovery(&d)
		}
	}
	if err := pf.WriteDiscovery(out, d, *format); err != nil {
		fmt.Fprintln(diagnostic, err)
		return 3
	}
	return d.ExitCode
}
