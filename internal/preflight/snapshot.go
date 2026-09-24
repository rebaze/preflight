package preflight

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const snapshotFileLimit = 20 << 20
const snapshotTotalLimit = 256 << 20

type SnapshotFile struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode"`
}

type Snapshot struct {
	Subject            Subject
	Dir                string
	Files              []SnapshotFile
	UnsupportedNPMKeys []string
}

// CaptureSnapshot copies only the approved projection. An empty dest computes
// identity without materializing source. Source .npmrc contents are never copied.
func CaptureSnapshot(ctx context.Context, repo, baseline, base, profileDigest, policyDigest, dest string, p Profile) (Snapshot, error) {
	s, err := captureSnapshotOnce(ctx, repo, baseline, base, profileDigest, policyDigest, dest, p)
	if err != nil {
		return s, err
	}
	if dest != "" {
		verify, e := captureSnapshotOnce(ctx, repo, baseline, base, profileDigest, policyDigest, "", p)
		if e != nil {
			return s, e
		}
		if verify.Subject.SnapshotDigest != s.Subject.SnapshotDigest {
			return s, fmt.Errorf("workspace_changed_during_run: source changed during snapshot capture")
		}
	}
	return s, nil
}

func captureSnapshotOnce(ctx context.Context, repo, baseline, base, profileDigest, policyDigest, dest string, p Profile) (Snapshot, error) {
	s := Snapshot{Dir: dest, Files: []SnapshotFile{}, UnsupportedNPMKeys: []string{}}
	rootBytes, err := snapshotGitRead(ctx, repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return s, err
	}
	root := strings.TrimSuffix(string(rootBytes), "\n")
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return s, err
	}
	if dest != "" {
		abs, e := snapshotCanonicalPath(dest)
		if e != nil {
			return s, e
		}
		rel, e := filepath.Rel(root, abs)
		if e != nil {
			return s, e
		}
		if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return s, fmt.Errorf("snapshot destination must be outside source repository")
		}
		if err = os.MkdirAll(dest, 0700); err != nil {
			return s, err
		}
		entries, e := os.ReadDir(dest)
		if e != nil {
			return s, e
		}
		if len(entries) > 0 {
			return s, fmt.Errorf("snapshot destination must be empty")
		}
	}
	resolve := func(ref string) (string, error) {
		b, e := snapshotGitRead(ctx, root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
		return strings.TrimSpace(string(b)), e
	}
	baselineSHA, err := resolve(baseline)
	if err != nil {
		return s, fmt.Errorf("baseline commit: %w", err)
	}
	head, err := resolve("HEAD")
	if err != nil {
		return s, err
	}
	if base == "" {
		base = baselineSHA
	}
	baseSHA, err := resolve(base)
	if err != nil {
		return s, fmt.Errorf("comparison base: %w", err)
	}
	mb, err := snapshotGitRead(ctx, root, "merge-base", baseSHA, head)
	if err != nil {
		return s, err
	}
	mergeBase := strings.TrimSpace(string(mb))
	s.Subject = Subject{RepoRoot: root, BaselineCommit: baselineSHA, ComparisonBase: baseSHA, Head: head, MergeBase: mergeBase, ChangedPaths: []string{}, OutOfScopePaths: []string{}, ExcludedCategories: []string{"git metadata", "dependency and generated output", "environment and credential/private-key files", "ignored untracked files", "source npmrc (identity only; generated safe config used)"}}
	selected := map[string]bool{}
	addSelected := func(paths []string) error {
		for _, name := range paths {
			if err := snapshotValidPath(name); err != nil {
				return err
			}
			if snapshotInScope(name, p) && !snapshotExcluded(name) {
				selected[name] = true
			}
		}
		return nil
	}
	bfiles, err := BaselineFiles(ctx, root, baselineSHA, p)
	if err != nil {
		return s, err
	}
	if err = addSelected(bfiles); err != nil {
		return s, err
	}
	for _, args := range [][]string{{"ls-files", "-z"}, {"ls-files", "--others", "--exclude-standard", "-z"}} {
		b, e := snapshotGitRead(ctx, root, args...)
		if e != nil {
			return s, e
		}
		if e = addSelected(snapshotNUL(b)); e != nil {
			return s, e
		}
	}
	if err = addSelected(p.SourceFiles); err != nil {
		return s, err
	}
	changed := map[string]bool{}
	for _, args := range [][]string{{"diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-status", "-z", mergeBase, head, "--"}, {"diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-status", "-z", "--cached", head, "--"}, {"diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-status", "-z", "--"}} {
		b, e := snapshotGitRead(ctx, root, args...)
		if e != nil {
			return s, e
		}
		parts := snapshotNUL(b)
		if len(parts)%2 != 0 {
			return s, fmt.Errorf("invalid NUL-delimited Git change output")
		}
		for i := 1; i < len(parts); i += 2 {
			changed[parts[i]] = true
		}
	}
	untracked, err := snapshotGitRead(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return s, err
	}
	for _, name := range snapshotNUL(untracked) {
		changed[name] = true
	}
	for name := range changed {
		if err = snapshotValidPath(name); err != nil {
			return s, err
		}
		s.Subject.ChangedPaths = append(s.Subject.ChangedPaths, name)
		if snapshotInScope(name, p) && !snapshotExcluded(name) {
			selected[name] = true
		}
		if !snapshotInScope(name, p) || snapshotExcluded(name) {
			s.Subject.OutOfScopePaths = append(s.Subject.OutOfScopePaths, name)
		}
	}
	sort.Strings(s.Subject.ChangedPaths)
	sort.Strings(s.Subject.OutOfScopePaths)
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, v := range []string{"preflight-snapshot-v1", root, baselineSHA, baseSHA, head, mergeBase, profileDigest, policyDigest} {
		snapshotHashField(h, v)
	}
	// Out-of-scope path names also determine the scope-review finding; their
	// contents are deliberately never opened.
	for _, name := range s.Subject.ChangedPaths {
		snapshotHashField(h, "changed")
		snapshotHashField(h, name)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return s, err
	}
	defer r.Close()
	var total int64
	for _, name := range names {
		if err = ctx.Err(); err != nil {
			return s, err
		}
		f := SnapshotFile{Path: name, Kind: "missing"}
		info, e := snapshotLstat(r, name)
		if e != nil && !os.IsNotExist(e) {
			return s, e
		}
		if e == nil {
			if !info.Mode().IsRegular() {
				return s, fmt.Errorf("unsupported special source file %q", name)
			}
			if info.Size() > snapshotFileLimit {
				return s, fmt.Errorf("source file %q exceeds 20 MiB", name)
			}
			file, e := r.Open(name)
			if e != nil {
				return s, e
			}
			actual, e := file.Stat()
			if e != nil {
				file.Close()
				return s, e
			}
			if !os.SameFile(info, actual) {
				file.Close()
				return s, fmt.Errorf("source file %q changed while opening", name)
			}
			data, e := io.ReadAll(io.LimitReader(file, snapshotFileLimit+1))
			file.Close()
			if e != nil {
				return s, e
			}
			if len(data) > snapshotFileLimit {
				return s, fmt.Errorf("source file %q exceeds 20 MiB", name)
			}
			total += int64(len(data))
			if total > snapshotTotalLimit {
				return s, fmt.Errorf("source snapshot exceeds 256 MiB")
			}
			f.Kind = "regular"
			f.Mode = uint32(info.Mode().Perm() & 0111)
			digest := sha256.Sum256(data)
			f.Digest = "sha256:" + hex.EncodeToString(digest[:])
			if path.Base(name) == ".npmrc" {
				s.UnsupportedNPMKeys = append(s.UnsupportedNPMKeys, snapshotNPMReview(data)...)
			} else if dest != "" {
				out := filepath.Join(dest, filepath.FromSlash(name))
				if e = os.MkdirAll(filepath.Dir(out), 0700); e != nil {
					return s, e
				}
				if e = os.WriteFile(out, data, 0600|os.FileMode(f.Mode)); e != nil {
					return s, e
				}
			}
		}
		s.Files = append(s.Files, f)
		for _, v := range []string{f.Path, f.Kind, fmt.Sprint(f.Mode), f.Digest} {
			snapshotHashField(h, v)
		}
	}
	sort.Strings(s.UnsupportedNPMKeys)
	s.UnsupportedNPMKeys = snapshotUnique(s.UnsupportedNPMKeys)
	s.Subject.SnapshotDigest = "sha256:" + hex.EncodeToString(h.Sum(nil))
	return s, nil
}

// BaselineFiles lists approved path names only; excluded content is never read.
func BaselineFiles(ctx context.Context, repo, baseline string, p Profile) ([]string, error) {
	b, err := snapshotGitRead(ctx, repo, "rev-parse", "--verify", "--end-of-options", baseline+"^{commit}")
	if err != nil {
		return nil, err
	}
	b, err = snapshotGitRead(ctx, repo, "ls-tree", "-r", "-z", "--name-only", strings.TrimSpace(string(b)), "--")
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, name := range snapshotNUL(b) {
		if err := snapshotValidPath(name); err != nil {
			return nil, err
		}
		if snapshotInScope(name, p) && !snapshotExcluded(name) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func ReadBaselineFile(ctx context.Context, repo, baseline, name string) ([]byte, error) {
	if err := snapshotValidPath(name); err != nil {
		return nil, err
	}
	if snapshotExcluded(name) || path.Base(name) == ".npmrc" {
		return nil, fmt.Errorf("refusing excluded baseline file %q", name)
	}
	b, err := snapshotGitRead(ctx, repo, "rev-parse", "--verify", "--end-of-options", baseline+"^{commit}")
	if err != nil {
		return nil, err
	}
	sha := strings.TrimSpace(string(b))
	b, err = snapshotGitRead(ctx, repo, "cat-file", "-s", sha+":"+name)
	if err != nil {
		return nil, err
	}
	var size int64
	if _, err = fmt.Sscan(string(b), &size); err != nil || size < 0 || size > snapshotFileLimit {
		return nil, fmt.Errorf("invalid or oversized baseline file %q", name)
	}
	return snapshotGitRead(ctx, repo, "cat-file", "blob", sha+":"+name)
}

func snapshotLstat(r *os.Root, name string) (os.FileInfo, error) {
	parts := strings.Split(name, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		info, err := r.Lstat(p)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("source symlink rejected: %q", p)
		}
		if i == len(parts)-1 {
			return info, nil
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source parent is not a directory: %q", p)
		}
	}
	return nil, fmt.Errorf("empty source path")
}

func snapshotCanonicalPath(name string) (string, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", err
	}
	resolved, err = snapshotCanonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(abs)), nil
}

func snapshotValidPath(name string) error {
	if name == "" || strings.ContainsRune(name, 0) || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") {
		return fmt.Errorf("unsupported repository path %q", name)
	}
	return nil
}
func snapshotInScope(name string, p Profile) bool {
	for _, f := range p.SourceFiles {
		if name == f {
			return true
		}
	}
	for _, prefix := range p.SourcePrefixes {
		prefix = strings.TrimSuffix(prefix, "/")
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	return false
}
func snapshotExcluded(name string) bool {
	for _, part := range strings.Split(strings.ToLower(name), "/") {
		switch part {
		case ".git", "node_modules", ".nuxt", ".output", "dist", "coverage", "reports", "test-results", "playwright-report", ".ssh", ".aws", ".azure", ".gnupg":
			return true
		}
		if part == ".env" || strings.HasPrefix(part, ".env.") || strings.Contains(part, "credential") || part == "id_rsa" || part == "id_ed25519" || part == "id_ecdsa" || part == ".netrc" || part == ".npm-token" || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") || strings.HasSuffix(part, ".p12") || strings.HasSuffix(part, ".pfx") {
			return true
		}
	}
	return false
}
func snapshotNPMReview(data []byte) []string {
	keys := []string{}
	allowed := map[string]string{"save-exact": "true", "fund": "false", "audit": "false", "registry": "https://registry.npmjs.org"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			keys = append(keys, "invalid-setting")
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		want, known := allowed[key]
		if !ok || !known || value != want {
			if len(key) > 256 {
				key = "unsupported-key"
			}
			keys = append(keys, key)
		}
	}
	return keys
}
func snapshotUnique(values []string) []string {
	out := []string{}
	for _, v := range values {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}
func snapshotHashField(h hash.Hash, v string) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(v)))
	h.Write(n[:])
	h.Write([]byte(v))
}
func snapshotNUL(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
}

type snapshotBoundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *snapshotBoundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("Git output exceeds limit")
	}
	return b.Buffer.Write(p)
}
func snapshotGitRead(ctx context.Context, repo string, args ...string) ([]byte, error) {
	fixed := []string{"--no-pager", "--no-replace-objects", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "diff.external=", "-C", repo}
	cmd := exec.CommandContext(ctx, "git", append(fixed, args...)...)
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if !strings.HasPrefix(key, "GIT_") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "LC_ALL=C")
	stdout := &snapshotBoundedBuffer{limit: 32 << 20}
	stderr := &snapshotBoundedBuffer{limit: 64 << 10}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Git %s failed: %w", args[0], err)
	}
	return stdout.Bytes(), nil
}
