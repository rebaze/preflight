package main

import (
	"context"
	"flag"
	"fmt"
	pf "github.com/rebaze/preflight/internal/preflight"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const usage = `rebaze Preflight: sourced discovery and bounded local checks, not release authorization.

preflight version
preflight capabilities [--format text|json]
preflight inspect [--repo PATH] [--base REF] [--github] [--pr NUMBER] [--compare FILE] [--output FILE] [--format text|json]
preflight init --repo PATH --baseline SHA --profile FILE --policy-dir DIR --state-dir DIR --conftest FILE
preflight explain --state-dir DIR [--base REF] --format text|json
preflight prepare --state-dir DIR [--base REF] --allow-downloads
preflight check --state-dir DIR [--base REF] --format text|json [--all] [--output FILE]
preflight status --state-dir DIR --report FILE --format text|json
`

var version = "dev"
var commit = "unknown"
var date = "unknown"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, out, diagnostic io.Writer) int {
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Fprintf(out, "preflight %s (commit %s, built %s)\n", version, commit, date)
		return 0
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, usage)
		return 0
	}
	mode := args[0]
	if mode == "capabilities" {
		return runCapabilities(args[1:], out, diagnostic)
	}
	if mode == "inspect" {
		return runInspect(args[1:], out, diagnostic)
	}
	f := flag.NewFlagSet(mode, flag.ContinueOnError)
	f.SetOutput(diagnostic)
	state := f.String("state-dir", "", "private state outside source checkout")
	format := f.String("format", "text", "text or json")
	var base, output, report, repo, baseline, profile, policy, evaluator string
	var all, downloads bool
	switch mode {
	case "init":
		f.StringVar(&repo, "repo", "", "source repo")
		f.StringVar(&baseline, "baseline", "", "explicit trusted commit")
		f.StringVar(&profile, "profile", "", "profile JSON")
		f.StringVar(&policy, "policy-dir", "", "trusted Rego directory")
		f.StringVar(&evaluator, "conftest", "", "external evaluator file")
	case "explain", "prepare", "check":
		f.StringVar(&base, "base", "", "comparison ref only")
		if mode == "check" {
			f.BoolVar(&all, "all", false, "run the full selected frontend suite")
			f.StringVar(&output, "output", "", "save canonical JSON report")
		}
		if mode == "prepare" {
			f.BoolVar(&downloads, "allow-downloads", false, "explicit dependency bootstrap permission")
		}
	case "status":
		f.StringVar(&report, "report", "", "saved report JSON")
	default:
		fmt.Fprint(diagnostic, usage)
		return 3
	}
	if e := f.Parse(args[1:]); e != nil {
		if e == flag.ErrHelp {
			return 0
		}
		return flagError(mode, e.Error(), args, out, diagnostic)
	}
	if f.NArg() != 0 {
		return flagError(mode, "unexpected positional arguments", args, out, diagnostic)
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintln(diagnostic, "format must be text or json")
		return 3
	}
	ctx := context.Background()
	o := pf.Options{StateDir: *state, Base: base, All: all}
	var r pf.Report
	var e error
	switch mode {
	case "init":
		e = pf.Init(ctx, pf.InitOptions{Repo: repo, Baseline: baseline, ProfilePath: profile, PolicyDir: policy, StateDir: *state, ConftestPath: evaluator})
		if e == nil {
			r, e = pf.Explain(ctx, o)
			r.Mode = "init"
		}
	case "explain":
		r, e = pf.Explain(ctx, o)
	case "prepare":
		e = pf.Prepare(ctx, o, downloads)
		if e == nil {
			r, e = pf.Explain(ctx, o)
			r.Mode = "prepare"
			r.Findings = append(r.Findings, pf.NewFinding("eer.tests", "pass", "dependencies_prepared", "Dependencies prepared without lifecycle scripts. Run check --all for offline test evidence."))
		}
	case "check":
		if output != "" {
			e = pf.ValidateOutputPath(*state, output)
		}
		if e == nil {
			r, e = pf.Check(ctx, o)
		}
	case "status":
		r, e = pf.Status(ctx, o, report)
	}
	if e != nil {
		if r.RunID == "" {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			r = pf.Report{Mode: mode, RunID: fmt.Sprintf("error-%d", time.Now().UnixNano()), StartedAt: now, FinishedAt: now}
		}
		r.Current = false
		r.Findings = append(r.Findings, pf.NewFinding("preflight.execution", "error", "configuration_or_execution_error", e.Error()))
	}
	pf.FinalizeReport(&r)
	if output != "" && e == nil {
		if werr := writeOutput(output, r); werr != nil {
			r.Findings = append(r.Findings, pf.NewFinding("preflight.report", "error", "output_write_error", werr.Error()))
			pf.FinalizeReport(&r)
		}
	}
	if e := pf.WriteReport(out, r, *format); e != nil {
		fmt.Fprintln(diagnostic, e)
		return 3
	}
	return r.ExitCode
}
func writeOutput(path string, r pf.Report) error {
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".preflight-report-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = pf.WriteReport(f, r, "json"); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if strings.HasSuffix(path, string(filepath.Separator)) {
		return fmt.Errorf("output requires a file")
	}
	return os.Rename(name, path)
}

// Flag errors keep machine-readable stdout when JSON was requested.
func flagError(mode, message string, args []string, out, diagnostic io.Writer) int {
	format := "text"
	for i, arg := range args {
		if arg == "--format=json" || (arg == "--format" && i+1 < len(args) && args[i+1] == "json") {
			format = "json"
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r := pf.Report{Mode: mode, RunID: fmt.Sprintf("error-%d", time.Now().UnixNano()), StartedAt: now, FinishedAt: now, Findings: []pf.Finding{pf.NewFinding("preflight.cli", "error", "invalid_arguments", message)}}
	pf.FinalizeReport(&r)
	if e := pf.WriteReport(out, r, format); e != nil {
		fmt.Fprintln(diagnostic, e)
	}
	return 3
}
