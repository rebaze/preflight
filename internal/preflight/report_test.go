package preflight

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func contractReport() Report {
	r := Report{SchemaVersion: 1, Mode: "check", Authority: "local-feedback", RunID: "test-run", StartedAt: "2026-09-23T00:00:00Z", FinishedAt: "2026-09-23T00:00:01Z", Current: true, Subject: Subject{RepoRoot: "/synthetic/repo", BaselineCommit: "baseline", Head: "head", MergeBase: "base", SnapshotDigest: "sha256:snapshot"}, Policy: Policy{PackageDigest: "sha256:policy", ProfileDigest: "sha256:profile", BaselineCommit: "baseline"}, Scope: Scope{Profile: "invoicex-frontend"}, Findings: []Finding{NewFinding("eer.tests", "pass", "tests_passed", "Required tests passed.")}}
	FinalizeReport(&r)
	return r
}
func TestReportJSONOnly(t *testing.T) {
	var out bytes.Buffer
	r := contractReport()
	if err := WriteReport(&out, r, "json"); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&out)
	var got Report
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("extra stdout: %v", err)
	}
	if got.Findings[0].Message != r.Findings[0].Message {
		t.Fatal("findings changed")
	}
}
func TestReportInvalidStatus(t *testing.T) {
	r := contractReport()
	r.Findings[0].Status = "green"
	var out bytes.Buffer
	if err := WriteReport(&out, r, "json"); err == nil {
		t.Fatal("invalid status accepted")
	}
	if out.Len() != 0 {
		t.Fatal("partial report written")
	}
}
func TestReportMandatoryFields(t *testing.T) {
	b, _ := json.Marshal(contractReport())
	var fields map[string]json.RawMessage
	json.Unmarshal(b, &fields)
	for key := range fields {
		t.Run(key, func(t *testing.T) {
			var copy map[string]json.RawMessage
			json.Unmarshal(b, &copy)
			delete(copy, key)
			bad, _ := json.Marshal(copy)
			var r Report
			if err := DecodeStrict(bad, &r); err == nil {
				t.Fatalf("missing %s accepted", key)
			}
		})
	}
}
func TestReportNestedMandatoryFields(t *testing.T) {
	b, _ := json.Marshal(contractReport())
	bad := bytes.Replace(b, []byte(`"paths":[],`), nil, 1)
	var r Report
	if bytes.Equal(b, bad) {
		t.Fatal("fixture paths absent")
	}
	if err := DecodeStrict(bad, &r); err == nil {
		t.Fatal("missing nested paths accepted")
	}
}
func TestReportStrictJSON(t *testing.T) {
	b, _ := json.Marshal(contractReport())
	for name, bad := range map[string][]byte{"unknown": bytes.Replace(b, []byte(`"schemaVersion":1`), []byte(`"extra":1,"schemaVersion":1`), 1), "duplicate": bytes.Replace(b, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":1,"schemaVersion":1`), 1), "trailing": append(append([]byte{}, b...), []byte(` {}`)...), "null": []byte(`null`)} {
		t.Run(name, func(t *testing.T) {
			var r Report
			if err := DecodeStrict(bad, &r); err == nil {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
}
func TestReportTextContainsAllFindingData(t *testing.T) {
	r := contractReport()
	r.Findings[0].Paths = []string{"test/example.test.ts"}
	r.Findings[0].Evidence = []Evidence{{Kind: "junit", Digest: "sha256:evidence", Path: "/local/result.xml"}}
	r.Findings[0].Owner = "frontend team"
	r.Findings[0].NextActions = []NextAction{{Description: "Run again", Command: "preflight", Args: []string{"check", "--all"}}}
	var text bytes.Buffer
	if err := WriteReport(&text, r, "text"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"eer.tests", "pass", "tests_passed", "Required tests passed.", "test/example.test.ts", "junit", "sha256:evidence", "/local/result.xml", "frontend team", "Run again", "preflight", "--all"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestReportNullArrayItemRejected(t *testing.T) {
	r := contractReport()
	r.Findings[0].Paths = []string{"example.ts"}
	b, _ := json.Marshal(r)
	b = bytes.Replace(b, []byte(`["example.ts"]`), []byte(`[null]`), 1)
	var got Report
	if err := DecodeStrict(b, &got); err == nil {
		t.Fatal("null array item accepted as an empty string")
	}
}
