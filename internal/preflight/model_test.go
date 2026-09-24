package preflight

import "testing"

func TestExitCodePreservesErrorPrecedence(t *testing.T) {
	f := []Finding{{Status: "fail"}, {Status: "error"}, {Status: "review_required"}}
	if got := ExitCode(f); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
}
func TestExitCode(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []string
		want     int
	}{
		{"pass", []string{"pass", "deferred", "review_required", "not_applicable"}, 0},
		{"fail", []string{"fail", "pass"}, 1}, {"missing", []string{"fail", "missing"}, 2},
		{"error", []string{"missing", "error", "fail"}, 3}, {"invalid", []string{"unknown"}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fs []Finding
			for _, s := range tc.statuses {
				fs = append(fs, Finding{Status: s})
			}
			if got := ExitCode(fs); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}
