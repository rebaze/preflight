package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestHelpNoSideEffects(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"--help"}, &out, &err); code != 0 {
		t.Fatal(code)
	}
	if out.Len() == 0 {
		t.Fatal("no help")
	}
}
func TestCLIJSONOnlyStdout(t *testing.T) {
	var out, err bytes.Buffer
	code := run([]string{"check", "--state-dir", "/nonexistent/preflight-fixture", "--format", "json"}, &out, &err)
	if code != 3 {
		t.Fatal(code)
	}
	var v map[string]any
	if e := json.Unmarshal(out.Bytes(), &v); e != nil {
		t.Fatalf("not a JSON report %q %v", out.String(), e)
	}
	if v["authority"] != "local-feedback" {
		t.Fatal(v)
	}
}

func TestCLIJSONFlagError(t *testing.T) {
	var out, err bytes.Buffer
	code := run([]string{"check", "--format", "json", "--unknown"}, &out, &err)
	if code != 3 {
		t.Fatal(code)
	}
	var report map[string]any
	if e := json.Unmarshal(out.Bytes(), &report); e != nil {
		t.Fatalf("flag error must preserve JSON stdout: %v, output %q", e, out.String())
	}
	if report["exitCode"] != float64(3) {
		t.Fatal(report)
	}
}
