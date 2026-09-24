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
	repo := f.String("repo", ".", "repository or directory to observe read-only")
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	d := pf.InspectLocal(ctx, *repo, *base)
	pf.FinalizeDiscovery(&d)
	if err := pf.WriteDiscovery(out, d, *format); err != nil {
		fmt.Fprintln(diagnostic, err)
		return 3
	}
	return d.ExitCode
}
