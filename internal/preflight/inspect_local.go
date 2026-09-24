package preflight

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const inspectPathLimit = 10000
const inspectSourceLimit = 64 << 10
const inspectSourcesLimit = 512 << 10
const inspectRecordsLimit = 1536 << 10

type inspectEntry struct{ oid, mode string }

// InspectLocal reads a bounded projection without Git diff/status, which can
// invoke repository-configured clean filters even when external diffs are off.
func InspectLocal(ctx context.Context, repo, base string) Discovery {
	first := inspectLocalOnce(ctx, repo, base)
	if first.Subject.RepoRoot != "" && first.ExitCode != 3 {
		last := inspectLocalOnce(ctx, repo, base)
		if first.Subject.InputDigest != last.Subject.InputDigest || first.Subject.Head != last.Subject.Head || first.Subject.IndexDigest != last.Subject.IndexDigest || (first.Subject.Current && !last.Subject.Current) {
			first.Subject.Current = false
			first.Diagnostics = append(first.Diagnostics, DiscoveryDiagnostic{Code: "source_changed", Message: "Source identity changed or could not be revalidated during discovery; collect a fresh observation before relying on these facts.", Severity: "warning"})
		}
	}
	FinalizeDiscovery(&first)
	return first
}

func inspectLocalOnce(ctx context.Context, repo, base string) Discovery {
	d := NewDiscovery()
	d.Subject.Current = true
	if repo == "" {
		repo = "."
	}
	abs, err := filepath.Abs(repo)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		inspectDiagnostic(&d, "repository_unavailable", "Cannot open the requested repository directory.", "", true)
		FinalizeDiscovery(&d)
		return d
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		inspectDiagnostic(&d, "repository_unavailable", "Repository path must be an accessible directory.", "", true)
		FinalizeDiscovery(&d)
		return d
	}
	root := abs
	b, gitErr := snapshotGitRead(ctx, abs, "rev-parse", "--show-toplevel")
	if gitErr == nil {
		root = strings.TrimSuffix(string(b), "\n")
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			inspectDiagnostic(&d, "repository_unavailable", "Cannot resolve Git worktree directory.", "", true)
			FinalizeDiscovery(&d)
			return d
		}
	}
	d.Subject.RepoRoot = root
	d.Subject.WorktreeID = inspectHash(root)
	d.Subject.RepositoryID = inspectHash(root)
	d.Subject.BaseRef = base
	index, head, comparison := map[string]inspectEntry{}, map[string]inspectEntry{}, map[string]inspectEntry{}
	untracked := map[string]bool{}
	paths := map[string]bool{}
	objectFormat := "sha1"
	if gitErr != nil {
		d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: "git", Status: "unavailable", Detail: "Git identity unavailable; only bounded local documents and manifests were inspected."})
		inspectWalk(root, paths, &d)
	} else {
		d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: "git", Status: "complete", Detail: "Read-only Git object/index metadata and raw worktree bytes; no hooks, filters, diff drivers or project commands."})
		read := func(args ...string) string {
			v, e := snapshotGitRead(ctx, root, args...)
			if e != nil {
				return ""
			}
			return strings.TrimSpace(string(v))
		}
		if common := read("rev-parse", "--path-format=absolute", "--git-common-dir"); common != "" {
			if p, e := filepath.EvalSymlinks(common); e == nil {
				d.Subject.RepositoryID = inspectHash(p)
			}
		}
		d.Subject.Head = read("rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
		d.Subject.Branch = read("symbolic-ref", "--quiet", "--short", "HEAD")
		if f := read("rev-parse", "--show-object-format"); f == "sha256" {
			objectFormat = f
		}
		if d.Subject.Head == "" {
			inspectDiagnostic(&d, "head_unavailable", "No committed HEAD is available (the repository may be newly initialized).", "", false)
		}
		if d.Subject.BaseRef == "" {
			d.Subject.BaseRef = "HEAD"
		}
		d.Subject.BaseCommit = read("rev-parse", "--verify", "--end-of-options", d.Subject.BaseRef+"^{commit}")
		if d.Subject.BaseCommit == "" {
			inspectDiagnostic(&d, "comparison_unavailable", "Comparison ref cannot be resolved; other local facts remain useful.", "", false)
		} else if d.Subject.Head != "" {
			d.Subject.MergeBase = read("merge-base", d.Subject.BaseCommit, d.Subject.Head)
			if d.Subject.MergeBase == "" {
				inspectDiagnostic(&d, "comparison_unavailable", "Comparison commits have no available merge base.", "", false)
			}
		}
		index = inspectEntries(ctx, root, "", true, &d)
		if d.Subject.Head != "" {
			head = inspectEntries(ctx, root, d.Subject.Head, false, &d)
		}
		if d.Subject.MergeBase != "" {
			comparison = inspectEntries(ctx, root, d.Subject.MergeBase, false, &d)
		}
		for _, entries := range []map[string]inspectEntry{index, head, comparison} {
			for p := range entries {
				paths[p] = true
			}
		}
		b, e := snapshotGitRead(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
		if e != nil {
			inspectDiagnostic(&d, "untracked_unavailable", "Cannot enumerate non-ignored untracked paths.", "", false)
		} else {
			for _, p := range snapshotNUL(b) {
				paths[p] = true
				untracked[p] = true
			}
		}
	}
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	if len(names) > inspectPathLimit {
		names = names[:inspectPathLimit]
		inspectDiagnostic(&d, "path_limit", "Path limit reached; remaining source identities are unknown.", "", false)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		inspectDiagnostic(&d, "repository_unavailable", "Cannot open repository root.", "", true)
		FinalizeDiscovery(&d)
		return d
	}
	defer r.Close()
	var total int64
	sourceBytes := 0
	excluded := 0
	inputHash, indexHash := sha256.New(), sha256.New()
	for _, p := range names {
		if err := ctx.Err(); err != nil {
			inspectDiagnostic(&d, "cancelled", "Discovery stopped before all paths were inspected.", "", false)
			break
		}
		if snapshotValidPath(p) != nil {
			inspectDiagnostic(&d, "unsupported_path", "A repository path cannot be safely represented.", "", false)
			continue
		}
		i, h, c := index[p], head[p], comparison[p]
		for _, v := range []string{p, i.mode, i.oid} {
			snapshotHashField(indexHash, v)
		}
		if inspectExcluded(p) {
			d.Subject.Current = false
			excluded++
			d.Inputs = append(d.Inputs, DiscoveryInput{Path: p, Kind: "excluded"})
			snapshotHashField(inputHash, p)
			snapshotHashField(inputHash, "excluded")
			fi, statErr := snapshotLstat(r, p)
			marker := "present"
			if os.IsNotExist(statErr) {
				marker = "missing"
				if i.oid != "" {
					inspectAddChange(&d, p, "unstaged", i, inspectEntry{})
				}
			} else if statErr != nil || !fi.Mode().IsRegular() {
				marker = "unsupported"
			}
			snapshotHashField(inputHash, marker)
			if untracked[p] || i != h {
				d.Subject.Dirty = true
			}
			continue
		}
		if d.Subject.MergeBase != "" {
			inspectAddChange(&d, p, "committed", c, h)
		}
		if gitErr == nil {
			inspectAddChange(&d, p, "staged", h, i)
		}
		in := DiscoveryInput{Path: p, Kind: "file"}
		fi, e := snapshotLstat(r, p)
		var content []byte
		var rawOID string
		if os.IsNotExist(e) {
			in.Kind = "missing"
		} else if e != nil || !fi.Mode().IsRegular() {
			in.Kind = "unsupported"
			inspectDiagnostic(&d, "unsupported_source", "Symlink, special file, submodule, or inaccessible source was not read.", p, false)
		} else {
			in.Mode = uint32(fi.Mode().Perm())
			if fi.Size() > snapshotFileLimit || total+fi.Size() > snapshotTotalLimit {
				in.Kind = "unsupported"
				inspectDiagnostic(&d, "input_limit", "Source identity exceeds the bounded read limit.", p, false)
			} else {
				content, e = inspectRead(r, p, fi)
				if e != nil {
					in.Kind = "unsupported"
					inspectDiagnostic(&d, "source_unavailable", "Source could not be safely read or changed while being read.", p, false)
				} else {
					total += int64(len(content))
					in.Digest = inspectHash(string(content))
					rawOID = inspectBlobHash(content, objectFormat)
				}
			}
		}
		if in.Kind == "unsupported" {
			d.Subject.Current = false
		}
		d.Inputs = append(d.Inputs, in)
		for _, v := range []string{in.Path, in.Kind, in.Digest, strconv.FormatUint(uint64(in.Mode), 10)} {
			snapshotHashField(inputHash, v)
		}
		if gitErr == nil {
			if untracked[p] {
				d.Changes = append(d.Changes, DiscoveryChange{Path: p, Layer: "untracked", Status: "added", After: in.Digest})
				d.Subject.Dirty = true
			} else if in.Kind != "unsupported" {
				mode := "100644"
				if in.Mode&0111 != 0 {
					mode = "100755"
				}
				if in.Kind == "missing" {
					mode = ""
				}
				inspectAddChange(&d, p, "unstaged", i, inspectEntry{rawOID, mode})
			}
		}
		kind := inspectSourceKind(p)
		if kind != "" && in.Kind == "file" {
			if len(content) > inspectSourceLimit || sourceBytes+len(content) > inspectSourcesLimit {
				inspectDiagnostic(&d, "source_limit", "Document contents exceed the bounded disclosure limit; identity retained.", p, false)
				continue
			}
			if !utf8.Valid(content) || strings.ContainsRune(string(content), 0) {
				inspectDiagnostic(&d, "source_encoding", "Non-text source contents were not included.", p, false)
				continue
			}
			lines := strings.Count(string(content), "\n")
			if len(content) == 0 || content[len(content)-1] != '\n' {
				lines++
			}
			d.Sources = append(d.Sources, DiscoverySource{Path: p, Kind: kind, Digest: in.Digest, Content: string(content), StartLine: 1, EndLine: lines, Mode: in.Mode})
			sourceBytes += len(content)
		}
	}
	d.Subject.IndexDigest = hex.EncodeToString(indexHash.Sum(nil))
	snapshotHashField(inputHash, d.Subject.Head)
	snapshotHashField(inputHash, d.Subject.BaseRef)
	snapshotHashField(inputHash, d.Subject.BaseCommit)
	snapshotHashField(inputHash, d.Subject.IndexDigest)
	snapshotHashField(inputHash, d.Subject.MergeBase)
	d.Subject.InputDigest = hex.EncodeToString(inputHash.Sum(nil))
	if excluded != 0 {
		inspectDiagnostic(&d, "excluded_inputs", fmt.Sprintf("%d private, generated or dependency paths were excluded; their current contents and freshness are unknown.", excluded), "", false)
	}
	if !d.Subject.Current {
		d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: "worktree_cleanliness", Status: "partial", Detail: "Worktree cleanliness is unknown because some input identities could not be collected. dirty=false means no change was established, not a clean checkout."})
	}
	d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: "local_sources", Status: "complete", Detail: "At most 10,000 non-excluded paths; raw file hashes bounded to 20 MiB/file and 256 MiB total. Text allowlist: 64 KiB/file, 512 KiB total; serialized local facts bounded to 1.5 MiB. Git filters and line-ending transformations are not applied; raw differences may need review. Ignored untracked, private, generated and dependency paths are excluded."}, DiscoveryCoverage{Collector: "execution", Status: "not_requested", Detail: "No project code or checks were executed; documented and configured expectations remain unverified."})
	inspectBoundRecords(&d)

	FinalizeDiscovery(&d)
	return d
}

func inspectDiagnostic(d *Discovery, code, message, p string, fatal bool) {
	if len(d.Diagnostics) >= 128 {
		d.Subject.Current = false
		if len(d.Diagnostics) == 128 {
			d.Diagnostics = append(d.Diagnostics, DiscoveryDiagnostic{Code: "diagnostic_limit", Severity: "warning", Message: "Additional local diagnostics were omitted after the bounded limit; coverage remains incomplete."})
			d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: "diagnostic_limit", Status: "partial", Detail: "Additional diagnostics omitted after 128 entries."})
		}
		if fatal {
			d.Diagnostics[len(d.Diagnostics)-1].Severity = "error"
		}
		return
	}
	severity := "warning"
	if fatal {
		severity = "error"
		d.Subject.Current = false
	}
	d.Diagnostics = append(d.Diagnostics, DiscoveryDiagnostic{Code: code, Message: message, Path: p, Severity: severity})
	d.Coverage = append(d.Coverage, DiscoveryCoverage{Collector: code, Status: "partial", Detail: message})
}

func inspectEntries(ctx context.Context, root, revision string, index bool, d *Discovery) map[string]inspectEntry {
	args := []string{"ls-tree", "-rz", "--full-tree", revision}
	if index {
		args = []string{"ls-files", "--stage", "-z"}
	}
	b, err := snapshotGitRead(ctx, root, args...)
	out := map[string]inspectEntry{}
	if err != nil {
		inspectDiagnostic(d, "git_inventory", "Git object/index inventory is unavailable.", "", false)
		return out
	}
	for _, record := range snapshotNUL(b) {
		meta, p, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			inspectDiagnostic(d, "git_inventory", "Git inventory has an unsupported record.", "", false)
			continue
		}
		oid := fields[2]
		if index {
			oid = fields[1]
			if fields[2] != "0" {
				inspectDiagnostic(d, "index_conflict", "Unmerged index entries require review.", p, false)
			}
		}
		out[p] = inspectEntry{oid, fields[0]}
	}
	return out
}

func inspectAddChange(d *Discovery, p, layer string, before, after inspectEntry) {
	if before == after {
		return
	}
	status := "modified"
	if before.oid == "" {
		status = "added"
	}
	if after.oid == "" {
		status = "deleted"
	}
	d.Changes = append(d.Changes, DiscoveryChange{Path: p, Layer: layer, Status: status, Before: before.oid, After: after.oid})
	if layer != "committed" {
		d.Subject.Dirty = true
	}
}

func inspectRead(root *os.Root, p string, before os.FileInfo) ([]byte, error) {
	f, err := root.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return nil, fmt.Errorf("source changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, snapshotFileLimit+1))
	if err != nil {
		return nil, err
	}
	after, err := snapshotLstat(root, p)
	if err != nil || !os.SameFile(before, after) || before.Size() != int64(len(b)) || before.ModTime() != after.ModTime() || before.Mode() != after.Mode() {
		return nil, fmt.Errorf("source changed")
	}
	return b, nil
}

func inspectHash(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func inspectBlobHash(b []byte, format string) string {
	var h hash.Hash = sha1.New()
	if format == "sha256" {
		h = sha256.New()
	}
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func inspectExcluded(p string) bool {
	if snapshotExcluded(p) {
		return true
	}
	for _, part := range strings.Split(strings.ToLower(p), "/") {
		if part == ".npmrc" || part == ".pypirc" || part == ".gitconfig" || part == ".codex" || part == "vendor" || part == "build" || part == "target" || part == ".venv" || strings.Contains(part, "secret") || strings.Contains(part, "private") || strings.Contains(part, "token") {
			return true
		}
	}
	return false
}

func inspectSourceKind(p string) string {
	b := strings.ToLower(path.Base(p))
	switch b {
	case "agents.md", "claude.md", "contributing.md", "contributing", "readme.md", "readme", "development.md", "testing.md":
		return "documentation"
	case "package.json", "go.mod", "go.work", "cargo.toml", "pyproject.toml", "makefile", "justfile", ".gitattributes", ".gitignore", "codeowners":
		return "configuration"
	}
	if strings.HasPrefix(p, ".github/workflows/") && (strings.HasSuffix(b, ".yml") || strings.HasSuffix(b, ".yaml")) && strings.Count(p, "/") == 2 {
		return "workflow"
	}
	if b == "tsconfig.json" || strings.HasPrefix(b, "vitest.config.") || strings.HasPrefix(b, "vite.config.") || strings.HasPrefix(b, "jest.config.") {
		return "configuration"
	}
	return ""
}

func inspectWalk(root string, paths map[string]bool, d *Discovery) {
	count := 0
	err := filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		count++
		if count > inspectPathLimit {
			return fmt.Errorf("path limit")
		}
		if strings.Count(rel, "/") > 8 {
			inspectDiagnostic(d, "depth_limit", "Nested paths beyond eight directory levels were not inspected.", rel, false)
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if inspectExcluded(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && inspectSourceKind(rel) != "" {
			paths[rel] = true
		}
		return nil
	})
	if err != nil {
		inspectDiagnostic(d, "directory_scan", "Directory traversal was incomplete or exceeded its bounded path/depth limits.", "", false)
	}
}

// Bound the serialized observation, independently of raw-byte and path-count
// limits. A long filename is repeated in several records; JSON escaping also
// expands selected text. Keep a useful prefix of each fact class and make the
// omission explicit. The resulting digest must not be treated as full coverage.
func inspectBoundRecords(d *Discovery) {
	metadata, _ := json.MarshalIndent(struct {
		Diagnostics []DiscoveryDiagnostic
		Coverage    []DiscoveryCoverage
	}{d.Diagnostics, d.Coverage}, "", "  ")
	budget := inspectRecordsLimit - len(metadata)
	if budget < 0 {
		budget = 0
	}
	var used int
	var sourcesLimited, changesLimited, inputsLimited bool
	d.Sources, used, sourcesLimited = inspectRecordPrefix(d.Sources, budget/2)
	budget -= used
	d.Changes, used, changesLimited = inspectRecordPrefix(d.Changes, budget/2)
	budget -= used
	d.Inputs, _, inputsLimited = inspectRecordPrefix(d.Inputs, budget)
	if sourcesLimited || changesLimited || inputsLimited {
		d.Subject.Current = false
		inspectDiagnostic(d, "observation_limit", "Serialized local facts exceed the 1.5 MiB budget; useful source, change and input prefixes were retained. Omitted identities and expectations remain unknown.", "", false)
	}
}
func inspectRecordPrefix[T any](records []T, budget int) ([]T, int, bool) {
	used := 0
	for i, v := range records {
		encoded, _ := json.MarshalIndent(v, "    ", "  ")
		cost := len(encoded) + 6
		if used+cost > budget {
			return records[:i], used, true
		}
		used += cost
	}
	return records, used, false
}
