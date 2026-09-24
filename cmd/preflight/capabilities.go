package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pf "github.com/rebaze/preflight/internal/preflight"
	"io"
)

type capabilities struct {
	Schema          string   `json:"schema"`
	Version         string   `json:"version"`
	Commit          string   `json:"commit"`
	DiscoverySchema string   `json:"discoverySchema"`
	SkillProtocol   int      `json:"skillProtocol"`
	Features        []string `json:"features"`
}

func runCapabilities(args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	format := f.String("format", "text", "text or json")
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 3
	}
	if f.NArg() != 0 || (*format != "text" && *format != "json") {
		fmt.Fprintln(diagnostic, "capabilities accepts only --format text|json")
		return 3
	}
	c := capabilities{Schema: "preflight.capabilities/v1", Version: version, Commit: commit, DiscoverySchema: pf.DiscoverySchema, SkillProtocol: 1, Features: []string{"inspect", "github", "compare"}}
	if *format == "json" {
		if err := json.NewEncoder(out).Encode(c); err != nil {
			fmt.Fprintln(diagnostic, err)
			return 3
		}
	} else {
		if _, err := fmt.Fprintf(out, "Preflight %s (%s)\nSkill protocol: %d\nDiscovery: %s\nFeatures: inspect, github, compare\n", c.Version, c.Commit, c.SkillProtocol, c.DiscoverySchema); err != nil {
			fmt.Fprintln(diagnostic, err)
			return 3
		}
	}
	return 0
}
