package preflight

import (
	"bytes"
	"os"
	"testing"
)

func TestProfilePinned(t *testing.T) {
	p, err := LoadProfile("../../profiles/frontend-vitest.json")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "frontend-vitest" || p.TestWorkspace != "@example/frontend" {
		t.Fatalf("profile must use neutral example identifiers: %+v", p)
	}
	if p.NodeVersion != "24.18.0" || p.NPMVersion != "11.17.0" || p.RequiredOverrides["brace-expansion"] != "5.0.9" || p.RequiredOverrides["js-yaml"] != "4.3.1" {
		t.Fatalf("wrong pins: %+v", p)
	}
}
func TestProfileStrict(t *testing.T) {
	b, err := os.ReadFile("../../profiles/frontend-vitest.json")
	if err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string][]byte{
		"unknown":   bytes.Replace(b, []byte(`"schemaVersion": 1`), []byte(`"extra": true, "schemaVersion": 1`), 1),
		"duplicate": bytes.Replace(b, []byte(`"schemaVersion": 1`), []byte(`"schemaVersion": 1, "schemaVersion": 1`), 1),
		"missing":   bytes.Replace(b, []byte(`"schemaVersion": 1,`), nil, 1),
		"runtime":   bytes.Replace(b, []byte(`24.18.0`), []byte(`24.19.0`), 1),
		"traversal": bytes.Replace(b, []byte(`applications/frontend/`), []byte(`../frontend/`), 1),
		"override":  bytes.Replace(b, []byte(`5.0.9`), []byte(`5.0.8`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			path := t.TempDir() + "/profile.json"
			os.WriteFile(path, bad, 0600)
			if _, err := LoadProfile(path); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}
