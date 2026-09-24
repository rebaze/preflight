package preflight

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type SourceFacts struct {
	Parsed      bool   `json:"parsed"`
	Scheme      string `json:"scheme"`
	Host        string `json:"host"`
	Credentials bool   `json:"credentials"`
	Query       bool   `json:"query"`
	Fragment    bool   `json:"fragment"`
}
type BundleAncestor struct {
	Path           string      `json:"path"`
	DeclaresChild  bool        `json:"declaresChild"`
	InBundle       bool        `json:"inBundle"`
	Source         SourceFacts `json:"source"`
	IntegrityValid bool        `json:"integrityValid"`
}
type NPMPackage struct {
	Path            string           `json:"path"`
	Name            string           `json:"name"`
	Version         string           `json:"version"`
	Kind            string           `json:"kind"`
	Resolved        string           `json:"resolved"`
	Integrity       string           `json:"integrity"`
	InBundle        bool             `json:"inBundle"`
	Source          SourceFacts      `json:"source"`
	IntegrityValid  bool             `json:"integrityValid"`
	LinkTargetValid bool             `json:"linkTargetValid"`
	BundleChain     []BundleAncestor `json:"bundleChain"`
	Changed         bool             `json:"changed"`
}
type NPMFacts struct {
	Complete        bool              `json:"complete"`
	Errors          []string          `json:"errors"`
	LockfilePresent bool              `json:"lockfilePresent"`
	LockfileVersion int               `json:"lockfileVersion"`
	Overrides       map[string]string `json:"overrides"`
	Packages        []NPMPackage      `json:"packages"`
}
type lockRecord struct {
	Name               string   `json:"name"`
	Version            string   `json:"version"`
	Resolved           string   `json:"resolved"`
	Integrity          string   `json:"integrity"`
	Link               bool     `json:"link"`
	InBundle           bool     `json:"inBundle"`
	BundleDependencies []string `json:"bundleDependencies"`
}
type npmLock struct {
	LockfileVersion int                        `json:"lockfileVersion"`
	Packages        map[string]json.RawMessage `json:"packages"`
}

func decodeSourceJSON(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	if e := checkJSONValue(d); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return fmt.Errorf("trailing JSON input")
	}
	return json.Unmarshal(b, v)
}
func sourceFacts(raw string) (SourceFacts, string) {
	f := SourceFacts{}
	u, e := url.Parse(raw)
	if e != nil {
		return f, "[invalid URL]"
	}
	f.Parsed = u.IsAbs() && u.Opaque == ""
	f.Scheme = u.Scheme
	f.Host = u.Host
	f.Credentials = u.User != nil
	f.Query = u.RawQuery != "" || u.ForceQuery
	f.Fragment = u.Fragment != "" || strings.Contains(raw, "#")
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return f, u.String()
}
func validSRI(s string) bool {
	if !strings.HasPrefix(s, "sha512-") {
		return false
	}
	b, e := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(s, "sha512-"))
	return e == nil && len(b) == 64 && len(s) == len("sha512-")+88
}
func packageName(p string) string {
	_, suffix, found := strings.Cut(p, "node_modules/")
	if !found {
		return ""
	}
	for {
		_, tail, ok := strings.Cut(suffix, "/node_modules/")
		if !ok {
			break
		}
		suffix = tail
	}
	return suffix
}
func safeRelative(p string) bool {
	return p != "" && !path.IsAbs(p) && !strings.Contains(p, "\\") && path.Clean(p) == p && p != "." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\x00\n\r")
}
func bundleParent(p string) string {
	i := strings.LastIndex(p, "/node_modules/")
	if i < 0 {
		return ""
	}
	return p[:i]
}
func bundleChain(p string, records map[string]lockRecord) []BundleAncestor {
	chain := []BundleAncestor{}
	child := packageName(p)
	for parent := bundleParent(p); parent != ""; parent = bundleParent(parent) {
		r, exists := records[parent]
		if !exists {
			chain = append(chain, BundleAncestor{Path: parent})
			break
		}
		declares := false
		for _, n := range r.BundleDependencies {
			if n == child {
				declares = true
			}
		}
		source, _ := sourceFacts(r.Resolved)
		chain = append(chain, BundleAncestor{Path: parent, DeclaresChild: declares, InBundle: r.InBundle, Source: source, IntegrityValid: validSRI(r.Integrity)})
		if !r.InBundle {
			break
		}
		child = packageName(parent)
	}
	return chain
}

// CollectNPM records complete declarations and normalized facts. The trusted Rego
// rules, not this collector, decide source, integrity, links and override verdicts.
func CollectNPM(snapshotDir string, baselineLock []byte, p Profile) (NPMFacts, error) {
	n := NPMFacts{Errors: []string{}, Overrides: map[string]string{}, Packages: []NPMPackage{}}
	root := filepath.Join(snapshotDir, "applications/frontend")
	b, e := readBoundedFile(filepath.Join(root, "package-lock.json"), 20<<20)
	if os.IsNotExist(e) {
		return n, nil
	}
	if e != nil {
		return n, e
	}
	n.LockfilePresent = true
	var lock npmLock
	if e = decodeSourceJSON(b, &lock); e != nil {
		n.Errors = append(n.Errors, "malformed lockfile JSON")
		return n, fmt.Errorf("malformed npm lockfile JSON: %w", e)
	}
	n.LockfileVersion = lock.LockfileVersion
	if _, present := lock.Packages[""]; !present {
		n.Errors = append(n.Errors, "root package record missing")
		return n, fmt.Errorf("lockfile root package record missing")
	}

	if lock.Packages == nil {
		n.Errors = append(n.Errors, "packages object missing")
		return n, fmt.Errorf("lockfile packages object missing")
	}
	rootBytes, e := readBoundedFile(filepath.Join(root, "package.json"), 1<<20)
	if e != nil {
		n.Errors = append(n.Errors, "root package manifest unavailable")
		return n, e
	}
	var manifest struct {
		Workspaces []string                   `json:"workspaces"`
		Overrides  map[string]json.RawMessage `json:"overrides"`
	}
	if e = decodeSourceJSON(rootBytes, &manifest); e != nil {
		n.Errors = append(n.Errors, "invalid root package manifest")
		return n, e
	}
	for k, v := range manifest.Overrides {
		var value string
		if json.Unmarshal(v, &value) == nil {
			n.Overrides[k] = value
		} else {
			n.Overrides[k] = "[non-string override]"
		}
	}
	workspaceNames := map[string]string{}
	for _, pattern := range manifest.Workspaces {
		if !safeRelative(pattern) {
			n.Errors = append(n.Errors, "unsafe workspace pattern")
			return n, fmt.Errorf("unsafe workspace pattern")
		}
		matches, e := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern), "package.json"))
		if e != nil {
			return n, e
		}
		for _, file := range matches {
			rel, e := filepath.Rel(root, filepath.Dir(file))
			if e != nil {
				return n, e
			}
			bb, e := readBoundedFile(file, 1<<20)
			if e != nil {
				return n, e
			}
			var wm struct {
				Name string `json:"name"`
			}
			if e = decodeSourceJSON(bb, &wm); e != nil {
				return n, e
			}
			if wm.Name == "" {
				return n, fmt.Errorf("workspace manifest name missing")
			}
			workspaceNames[filepath.ToSlash(rel)] = wm.Name
		}
	}
	records := map[string]lockRecord{}
	for name, b := range lock.Packages {
		var r lockRecord
		if e = decodeSourceJSON(b, &r); e != nil {
			n.Errors = append(n.Errors, "invalid package record")
			return n, fmt.Errorf("invalid lock package record at %s: %w", name, e)
		}
		records[name] = r
	}
	var old npmLock
	if len(baselineLock) > 0 {
		if e = decodeSourceJSON(baselineLock, &old); e != nil {
			return n, fmt.Errorf("invalid pinned baseline lock: %w", e)
		}
	}
	keys := make([]string, 0, len(records))
	for k := range records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		r := records[key]
		source, resolved := sourceFacts(r.Resolved)
		record := NPMPackage{Path: key, Name: packageName(key), Version: r.Version, Kind: "unknown", Resolved: resolved, Integrity: r.Integrity, InBundle: r.InBundle, Source: source, IntegrityValid: validSRI(r.Integrity), BundleChain: []BundleAncestor{}}
		if r.Name != "" && record.Name == "" {
			record.Name = r.Name
		}
		oldRaw, had := old.Packages[key]
		var oldValue, curValue any
		json.Unmarshal(oldRaw, &oldValue)
		json.Unmarshal(lock.Packages[key], &curValue)
		oa, _ := json.Marshal(oldValue)
		ca, _ := json.Marshal(curValue)
		record.Changed = !had || !bytes.Equal(oa, ca)
		switch {
		case key == "":
			record.Kind = "root"
		case !safeRelative(key):
			record.Kind = "invalid"
		case r.Link && record.Name != "":
			record.Kind = "link"
			target := r.Resolved
			record.LinkTargetValid = safeRelative(target) && workspaceNames[target] != "" && workspaceNames[target] == record.Name
			record.Resolved = target
			if !safeRelative(target) {
				record.Resolved = "[unsafe workspace target]"
			}
		case workspaceNames[key] != "":
			record.Kind = "workspace"
			record.Name = workspaceNames[key]
		case record.Name != "":
			record.Kind = "external"
			if r.InBundle {
				record.Kind = "bundled"
				record.BundleChain = bundleChain(key, records)
			}
		}
		n.Packages = append(n.Packages, record)
	}
	n.Complete = true
	return n, nil
}
