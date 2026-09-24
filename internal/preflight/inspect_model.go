package preflight

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const DiscoverySchema = "preflight.discovery/v1"

// Discovery is an observation, not a check report or authorization. Source
// origin and verification are independent; no local test ran during discovery.
type Discovery struct {
	Schema      string                `json:"schema"`
	Authority   string                `json:"authority"`
	ObservedAt  string                `json:"observedAt"`
	Subject     DiscoverySubject      `json:"subject"`
	Inputs      []DiscoveryInput      `json:"inputs"`
	Sources     []DiscoverySource     `json:"sources"`
	Changes     []DiscoveryChange     `json:"changes"`
	Claims      []DiscoveryClaim      `json:"claims"`
	Coverage    []DiscoveryCoverage   `json:"coverage"`
	Diagnostics []DiscoveryDiagnostic `json:"diagnostics"`
	ExitCode    int                   `json:"exitCode"`
}
type DiscoverySubject struct {
	RepoRoot     string `json:"repoRoot"`
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
	Head         string `json:"head"`
	Branch       string `json:"branch"`
	BaseRef      string `json:"baseRef"`
	BaseCommit   string `json:"baseCommit"`
	MergeBase    string `json:"mergeBase"`
	IndexDigest  string `json:"indexDigest"`
	InputDigest  string `json:"inputDigest"`
	Dirty        bool   `json:"dirty"`
	Current      bool   `json:"current"`
}
type DiscoveryInput struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode"`
}
type DiscoverySource struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Digest    string `json:"digest"`
	Content   string `json:"content"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Mode      uint32 `json:"mode"`
}
type DiscoveryChange struct {
	Path   string `json:"path"`
	Layer  string `json:"layer"`
	Status string `json:"status"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type DiscoveryReference struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Line   int    `json:"line"`
}
type DiscoveryClaim struct {
	ID           string               `json:"id"`
	Origin       string               `json:"origin"`
	Verification string               `json:"verification"`
	Summary      string               `json:"summary"`
	Sources      []DiscoveryReference `json:"sources"`
}
type DiscoveryCoverage struct {
	Collector string `json:"collector"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}
type DiscoveryDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

func NewDiscovery() Discovery {
	return Discovery{Schema: DiscoverySchema, Authority: "local-feedback", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Inputs: []DiscoveryInput{}, Sources: []DiscoverySource{}, Changes: []DiscoveryChange{}, Claims: []DiscoveryClaim{}, Coverage: []DiscoveryCoverage{}, Diagnostics: []DiscoveryDiagnostic{}}
}
func discoveryExit(d Discovery) int {
	for _, v := range d.Diagnostics {
		if v.Severity == "error" {
			return 3
		}
	}
	if !d.Subject.Current {
		return 2
	}
	for _, v := range d.Coverage {
		if v.Status == "partial" || v.Status == "unavailable" {
			return 2
		}
	}
	return 0
}
func FinalizeDiscovery(d *Discovery) { d.ExitCode = discoveryExit(*d) }
func DecodeDiscovery(data []byte) (Discovery, error) {
	var d Discovery
	if len(data) > 4<<20 {
		return d, fmt.Errorf("discovery exceeds 4 MiB")
	}
	if err := DecodeStrict(data, &d); err != nil {
		return d, err
	}
	return d, ValidateDiscovery(d)
}
func discoveryEnum(value string, choices ...string) bool {
	for _, v := range choices {
		if value == v {
			return true
		}
	}
	return false
}
func ValidateDiscovery(d Discovery) error {
	if d.Schema != DiscoverySchema || d.Authority != "local-feedback" {
		return fmt.Errorf("unsupported discovery schema or authority")
	}
	if _, err := time.Parse(time.RFC3339Nano, d.ObservedAt); err != nil {
		return fmt.Errorf("invalid observation time: %w", err)
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	var shape Discovery
	if err = DecodeStrict(b, &shape); err != nil {
		return err
	}
	if d.ExitCode != discoveryExit(d) {
		return fmt.Errorf("discovery exit code disagrees with coverage/diagnostics")
	}
	for _, c := range d.Coverage {
		if c.Collector == "" || c.Detail == "" || !discoveryEnum(c.Status, "complete", "partial", "unavailable", "not_requested") {
			return fmt.Errorf("invalid coverage")
		}
	}
	for _, v := range d.Diagnostics {
		if v.Code == "" || v.Message == "" || !discoveryEnum(v.Severity, "warning", "error") {
			return fmt.Errorf("invalid diagnostic")
		}
	}
	seen := map[string]bool{}
	for _, s := range d.Sources {
		if snapshotValidPath(s.Path) != nil || s.Kind == "" || s.Digest == "" || s.StartLine < 1 || s.EndLine < s.StartLine || seen[s.Path] {
			return fmt.Errorf("invalid or duplicate source %q", s.Path)
		}
		if s.Digest != inspectHash(s.Content) {
			return fmt.Errorf("source content identity mismatch: %s", s.Path)
		}
		lines := strings.Count(s.Content, "\n")
		if s.Content == "" || !strings.HasSuffix(s.Content, "\n") {
			lines++
		}
		if s.EndLine != s.StartLine+lines-1 {
			return fmt.Errorf("source line range mismatch: %s", s.Path)
		}
		seen[s.Path] = true
	}
	seen = map[string]bool{}
	for _, v := range d.Inputs {
		if snapshotValidPath(v.Path) != nil || v.Kind == "" || seen[v.Path] {
			return fmt.Errorf("invalid or duplicate input %q", v.Path)
		}
		seen[v.Path] = true
	}
	for _, c := range d.Changes {
		if snapshotValidPath(c.Path) != nil || !discoveryEnum(c.Layer, "committed", "staged", "unstaged", "untracked") || !discoveryEnum(c.Status, "added", "modified", "deleted") {
			return fmt.Errorf("invalid change")
		}
	}
	seen = map[string]bool{}
	for _, c := range d.Claims {
		if c.ID == "" || c.Summary == "" || seen[c.ID] || !discoveryEnum(c.Origin, "documented", "configured", "inferred", "observed") || !discoveryEnum(c.Verification, "unverified", "verified", "unknown", "stale") {
			return fmt.Errorf("invalid claim")
		}
		seen[c.ID] = true
		if len(c.Sources) == 0 && c.Verification == "verified" {
			return fmt.Errorf("verified claim requires a source")
		}
		for _, r := range c.Sources {
			digest, digestErr := hex.DecodeString(r.Digest)
			if r.Path == "" || r.Line < 1 || digestErr != nil || len(digest) != 32 {
				return fmt.Errorf("invalid claim source")
			}
		}
	}
	return nil
}

// WriteDiscovery renders the same facts as JSON, with source line numbers in
// text. It deliberately offers no aggregate project pass/fail verdict.
func WriteDiscovery(w io.Writer, d Discovery, format string) error {
	if err := ValidateDiscovery(d); err != nil {
		return err
	}
	if format == "json" {
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		e.SetEscapeHTML(false)
		return e.Encode(d)
	}
	if format != "text" {
		return fmt.Errorf("unsupported discovery format %q", format)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "Preflight discovery — local feedback\nObserved: %s\nRepository: %s\nHEAD: %s\nComparison: %s (%s); merge base %s\nInput: %s; current=%t dirty=%t\n", d.ObservedAt, d.Subject.RepoRoot, d.Subject.Head, d.Subject.BaseRef, d.Subject.BaseCommit, d.Subject.MergeBase, d.Subject.InputDigest, d.Subject.Current, d.Subject.Dirty)
	for _, v := range d.Coverage {
		fmt.Fprintf(&b, "Coverage %s: %s — %s\n", v.Collector, v.Status, v.Detail)
	}
	for _, v := range d.Diagnostics {
		fmt.Fprintf(&b, "%s [%s] %s: %s\n", v.Severity, v.Code, v.Path, v.Message)
	}
	for _, c := range d.Changes {
		fmt.Fprintf(&b, "Change %s %s %s (%s → %s)\n", c.Layer, c.Status, c.Path, c.Before, c.After)
	}
	for _, c := range d.Claims {
		fmt.Fprintf(&b, "%s / %s: %s\n", c.Origin, c.Verification, c.Summary)
		for _, r := range c.Sources {
			fmt.Fprintf(&b, "  %s:%d [%s]\n", r.Path, r.Line, r.Digest)
		}
	}
	for _, s := range d.Sources {
		fmt.Fprintf(&b, "\nSource %s:%d-%d (%s, %s)\n", s.Path, s.StartLine, s.EndLine, s.Kind, s.Digest)
		for i, line := range strings.Split(strings.TrimSuffix(s.Content, "\n"), "\n") {
			fmt.Fprintf(&b, "  %d: %s\n", s.StartLine+i, line)
		}
	}
	fmt.Fprintf(&b, "\nNo project checks executed. Discovery exit: %d\n", d.ExitCode)
	_, err := w.Write(b.Bytes())
	return err
}
