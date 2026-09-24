package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCapabilitiesIndependentContract(t *testing.T) {
	var out, diag bytes.Buffer
	if code := run([]string{"capabilities", "--format", "json"}, &out, &diag); code != 0 {
		t.Fatalf("%d %s", code, diag.String())
	}
	var c struct {
		Schema, Version, Commit, DiscoverySchema string
		SkillProtocol                            int
		Features                                 []string
	}
	if err := json.Unmarshal(out.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if c.Schema != "preflight.capabilities/v1" || c.DiscoverySchema != "preflight.discovery/v1" || c.SkillProtocol != 1 || len(c.Features) != 3 || c.Version != version || c.Commit != commit {
		t.Fatalf("%+v", c)
	}
	for _, args := range [][]string{{"capabilities", "--format", "xml"}, {"capabilities", "unexpected"}, {"capabilities", "--state-dir", "unused"}} {
		out.Reset()
		diag.Reset()
		if run(args, &out, &diag) != 3 {
			t.Fatal(args)
		}
	}
}
