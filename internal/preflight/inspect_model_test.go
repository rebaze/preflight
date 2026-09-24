package preflight

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiscoveryStrictContract(t *testing.T) {
	d := NewDiscovery()
	d.Subject.Current = true
	FinalizeDiscovery(&d)
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeDiscovery(b); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"unknown":   strings.Replace(string(b), `"schema":`, `"extra":true,"schema":`, 1),
		"duplicate": strings.Replace(string(b), `"schema":`, `"exitCode":0,"schema":`, 1),
		"missing":   strings.Replace(string(b), `"sources":[],`, "", 1),
		"null":      strings.Replace(string(b), `"sources":[]`, `"sources":null`, 1),
		"version":   strings.Replace(string(b), DiscoverySchema, "preflight.discovery/v9", 1),
		"exit":      strings.Replace(string(b), `"exitCode":0`, `"exitCode":1`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDiscovery([]byte(body)); err == nil {
				t.Fatalf("accepted %s", body)
			}
		})
	}
}
func TestDiscoveryPartialAndErrorsRetainClaims(t *testing.T) {
	d := NewDiscovery()
	d.Subject.Current = true
	d.Claims = append(d.Claims, DiscoveryClaim{ID: "expectation", Origin: "documented", Verification: "unverified", Summary: "Preserve response fields", Sources: []DiscoveryReference{{Path: "CONTRIBUTING.md", Digest: strings.Repeat("a", 64), Line: 4}}})
	d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: "git", Status: "partial", Detail: "missing base"})
	FinalizeDiscovery(&d)
	if d.ExitCode != 2 {
		t.Fatal(d.ExitCode)
	}
	d.Diagnostics = append(d.Diagnostics, DiscoveryDiagnostic{Code: "read_error", Message: "unreadable directory", Severity: "error"})
	FinalizeDiscovery(&d)
	if d.ExitCode != 3 {
		t.Fatal(d.ExitCode)
	}
	for _, format := range []string{"text", "json"} {
		var b bytes.Buffer
		if err := WriteDiscovery(&b, d, format); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "Preserve response fields") || !strings.Contains(b.String(), "read_error") {
			t.Fatal(b.String())
		}
	}
	d.Claims[0].Verification = "pass"
	if ValidateDiscovery(d) == nil {
		t.Fatal("invented verification accepted")
	}
}

func TestDiscoveryRejectsMismatchedSourceIdentity(t *testing.T) {
	d := NewDiscovery()
	d.Subject.Current = true
	d.Sources = append(d.Sources, DiscoverySource{Path: "README.md", Kind: "documentation", Content: "actual\n", Digest: strings.Repeat("a", 64), StartLine: 1, EndLine: 1})
	FinalizeDiscovery(&d)
	if ValidateDiscovery(d) == nil {
		t.Fatal("source content accepted with unrelated digest")
	}
	d.Sources[0].Digest = inspectHash("actual\n")
	d.Sources[0].EndLine = 3
	if ValidateDiscovery(d) == nil {
		t.Fatal("invented line range accepted")
	}
}

func TestDiscoveryRejectsIncompleteClaimReferences(t *testing.T) {
	for _, ref := range []DiscoveryReference{{Path: "README.md", Line: 1}, {Path: "README.md", Digest: strings.Repeat("a", 64), Line: 0}, {Path: "README.md", Digest: "not-a-digest", Line: 1}} {
		d := NewDiscovery()
		d.Subject.Current = true
		d.Claims = []DiscoveryClaim{{ID: "documented", Origin: "documented", Verification: "unverified", Summary: "Expectation", Sources: []DiscoveryReference{ref}}}
		FinalizeDiscovery(&d)
		if ValidateDiscovery(d) == nil {
			t.Fatalf("accepted incomplete citation: %+v", ref)
		}
	}
}
