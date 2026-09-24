package main

import (
	"bytes"
	"encoding/json"
	pf "github.com/rebaze/preflight/internal/preflight"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectIndependentCLI(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "CONTRIBUTING.md"), []byte("Preserve response fields.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diag bytes.Buffer
	code := run([]string{"inspect", "--repo", repo, "--format", "json"}, &out, &diag)
	if code != 2 {
		t.Fatalf("want partial non-Git discovery got %d %s %s", code, out.String(), diag.String())
	}
	var d pf.Discovery
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Schema != pf.DiscoverySchema || len(d.Sources) != 1 {
		t.Fatalf("%+v", d)
	}
	for _, args := range [][]string{{"inspect", "--typo", "--format=json"}, {"inspect", "extra", "--format=json"}} {
		out.Reset()
		diag.Reset()
		if run(args, &out, &diag) != 3 {
			t.Fatal(args)
		}
		if _, err := pf.DecodeDiscovery(out.Bytes()); err != nil {
			t.Fatalf("flag errors require discovery JSON: %s %v", out.String(), err)
		}
	}
}
