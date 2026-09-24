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

func TestInspectSaveCompareAndPreservePrevious(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	guide := filepath.Join(repo, "CONTRIBUTING.md")
	if err := os.WriteFile(guide, []byte("Preserve response fields.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := filepath.Join(outside, "before.json")
	var out, diag bytes.Buffer
	if code := run([]string{"inspect", "--repo", repo, "--output", before, "--format", "json"}, &out, &diag); code != 2 {
		t.Fatalf("%d %s %s", code, out.String(), diag.String())
	}
	original, err := os.ReadFile(before)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guide, []byte("Keep response fields and add tests.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diag.Reset()
	code := run([]string{"inspect", "--repo", repo, "--compare", before, "--format", "json"}, &out, &diag)
	d, err := pf.DecodeDiscovery(out.Bytes())
	if err != nil {
		t.Fatalf("%v %s %s", err, out.String(), diag.String())
	}
	if code != 2 || d.Comparison == nil || !d.Comparison.Compatible || !d.Comparison.PreviousEvidenceStale || len(d.Comparison.InputChanges) != 1 {
		t.Fatalf("%+v", d)
	}
	out.Reset()
	diag.Reset()
	if run([]string{"inspect", "--repo", repo, "--output", before, "--format", "json"}, &out, &diag) != 3 {
		t.Fatal("overwrote observation")
	}
	got, err := os.ReadFile(before)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("previous observation changed")
	}
	out.Reset()
	diag.Reset()
	if run([]string{"inspect", "--repo", repo, "--output", filepath.Join(repo, "observation.json"), "--format", "json"}, &out, &diag) != 3 {
		t.Fatal("saved private evidence in checkout")
	}
}
