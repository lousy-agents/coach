package codesignalcli

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"
)

// Snapshot-read boundary budgets. Unlike maxProjectConfig* in project.go
// (sized for one small config document), these bound a whole-tree listing
// and arbitrary tracked source files, so they are deliberately larger while
// still finite: an oversized listing or file fails closed instead of
// exhausting memory or hanging the CLI (issue #210).
const (
	maxSnapshotListBytes = 64 << 20 // 64 MiB: full `git ls-tree -r` path listing
	maxSnapshotFileBytes = 32 << 20 // 32 MiB: single tracked file's content
	maxSnapshotGitStderr = 64 << 10
	snapshotGitTimeout   = 30 * time.Second
)

// snapshotGitCommandContext builds the git child used by NewGoSnapshotFS's
// reads. Unlike gitCommandContext in project.go, it never inherits the
// parent process's ambient environment (see sanitizedSnapshotGitEnv).
var snapshotGitCommandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = sanitizedSnapshotGitEnv()
	return cmd
}

// runSnapshotGit is the git seam used by NewGoSnapshotFS and its returned
// fs.FS. Tests may replace it to exercise timeout and bound failures without
// hanging, mirroring runProjectConfigGit in project.go.
var runSnapshotGit = func(dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	return runGitBytesBoundedWith(snapshotGitCommandContext, dir, maxStdout, maxStderr, timeout, args...)
}

// sanitizedSnapshotGitEnv returns the minimal environment for every git
// child a snapshot read spawns. It never forwards the parent process's
// ambient environment wholesale: only PATH (so the git executable can be
// found) and HOME are carried through, plus GIT_TERMINAL_PROMPT/
// GIT_CONFIG_NOSYSTEM to force non-interactive, system-config-free
// behavior, and GIT_NO_LAZY_FETCH to disable partial-clone promisor fetches
// so a blobless snapshot can never reach the network. Go-tooling and
// proxy/credential variables (GOPROXY, GOFLAGS,
// GOPATH, GO111MODULE, GONOSUMCHECK, GOSUMDB, HTTP(S)_PROXY, NO_PROXY, ...)
// are deliberately never forwarded, even when set in the parent process:
// this package only runs `git ls-tree`/`git show` against a local
// repository, which needs none of them, and forwarding them would be an
// unnecessary vector for those settings to influence a read that must stay
// local and hermetic.
func sanitizedSnapshotGitEnv() []string {
	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_NO_LAZY_FETCH=1",
	}
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	if value, ok := os.LookupEnv("HOME"); ok {
		env = append(env, "HOME="+value)
	}
	return env
}

// goSnapshotFS is a read-only fs.FS over one immutable Git revision. Every
// tracked path is enumerated once at construction; file reads are served
// lazily via `git show` so a large repository is not fully prefetched into
// memory. A later change could batch reads via `git cat-file --batch` (see
// revisionFileReader in catfile.go) if per-file `git show` latency becomes
// a measured problem; that is out of scope here.
type goSnapshotFS struct {
	dir      string
	revision string
	isDir    map[string]bool
	isFile   map[string]bool
	children map[string][]fs.DirEntry
}

// snapshotListError wraps a failed `git ls-tree` listing (NewGoSnapshotFS's
// construction step). Error() renders the same human-readable text the
// previous fmt.Errorf-based message did, dir included, for any consumer
// that just wants a message to display; Unwrap exposes the underlying git
// failure alone, with no path interpolated, for a consumer that must not
// leak dir into a diagnostic (see NewGoSnapshotFS's doc comment).
type snapshotListError struct {
	revision string
	dir      string
	err      error
}

func (e *snapshotListError) Error() string {
	return fmt.Sprintf("coach: git ls-tree failed for revision %q in %q: %s", e.revision, e.dir, e.err)
}

func (e *snapshotListError) Unwrap() error { return e.err }

// NewGoSnapshotFS returns an fs.FS that reads every file tracked at
// revision in the Git repository at dir, using only git plumbing
// (ls-tree/show) -- never the worktree, never `go` or another toolchain,
// never network. It is intended as the read boundary
// pkg/projectmodel.BuildGoModel/DiscoverGoRoots (issue #210) consume; this
// function does not itself call into pkg/projectmodel -- that wiring is
// issue #220's SuggestProjectConfig, its first consumer.
//
// The returned fs.FS enumerates the full file list once via one bounded
// `git ls-tree -r -z --name-only <revision>` call at construction time
// (reusing the same git invocation shape as DiscoverTrackedFiles in
// git.go), then serves individual file reads lazily via bounded
// `git show <revision>:<path>` calls (reusing runGitBytesBounded's shared
// implementation from project.go). It implements fs.FS, fs.ReadDirFS, and
// fs.ReadFileFS so that fs.WalkDir and fs.ReadFile work efficiently without
// a slow per-directory Open/ReadDir dance.
//
// If revision is unresolvable or dir is not a Git repository, NewGoSnapshotFS
// returns an error; it never returns a silently empty FS for such a failure.
// A failed listing is returned as *snapshotListError, not a bare
// fmt.Errorf, so a caller that must not leak dir into a diagnostic message
// (SuggestProjectConfig's snapshotUnavailableMessage) can render its own
// message from Unwrap() alone instead of scrubbing dir out of formatted
// text via substring match.
func NewGoSnapshotFS(dir, revision string) (fs.FS, error) {
	if revision == "" {
		return nil, fmt.Errorf("coach: revision must be a non-empty Git revision")
	}

	output, err := runSnapshotGit(dir, maxSnapshotListBytes, maxSnapshotGitStderr, snapshotGitTimeout, "ls-tree", "-r", "-z", "--name-only", revision)
	if err != nil {
		return nil, &snapshotListError{revision: revision, dir: dir, err: err}
	}

	fsys := &goSnapshotFS{
		dir:      dir,
		revision: revision,
		isDir:    map[string]bool{".": true},
		isFile:   map[string]bool{},
	}
	childSets := map[string]map[string]bool{}

	for _, p := range splitNULPaths(output) {
		if err := validateSnapshotPath(p); err != nil {
			return nil, fmt.Errorf("coach: git ls-tree reported an unsafe path %q: %w", p, err)
		}
		fsys.isFile[p] = true
		addSnapshotPath(childSets, fsys.isDir, p)
	}

	fsys.children = finalizeSnapshotChildren(childSets)
	return fsys, nil
}

// validateSnapshotPath defensively rejects any git-reported path that would
// escape the snapshot root when used as an fs.FS name. `git ls-tree`
// shouldn't report such a path, but this is treated as an operational error
// rather than silently included.
func validateSnapshotPath(p string) error {
	if p == "" {
		return fmt.Errorf("path must be non-empty")
	}
	if path.IsAbs(p) {
		return fmt.Errorf("path must not be absolute")
	}
	clean := path.Clean(p)
	if clean != p || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path must be a normalized, repository-relative path")
	}
	if strings.Contains(p, "\\") {
		return fmt.Errorf("path must use forward-slash separators")
	}
	return nil
}

// addSnapshotPath records filePath's directory ancestry into childSets
// (parent name -> child name -> isDir) and isDir (full directory paths).
func addSnapshotPath(childSets map[string]map[string]bool, isDir map[string]bool, filePath string) {
	segments := strings.Split(filePath, "/")
	parent := "."
	for i, segment := range segments {
		if childSets[parent] == nil {
			childSets[parent] = map[string]bool{}
		}
		if i == len(segments)-1 {
			childSets[parent][segment] = false
			return
		}
		childSets[parent][segment] = true
		parent = path.Join(parent, segment)
		isDir[parent] = true
	}
}

func finalizeSnapshotChildren(childSets map[string]map[string]bool) map[string][]fs.DirEntry {
	out := make(map[string][]fs.DirEntry, len(childSets))
	for dir, names := range childSets {
		entries := make([]fs.DirEntry, 0, len(names))
		for name, isDir := range names {
			entries = append(entries, snapshotDirEntry{name: name, isDir: isDir})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		out[dir] = entries
	}
	return out
}

// normalizeSnapshotName validates a caller-supplied fs.FS name per the
// io/fs contract (slash-separated, no ./.. elements); "." denotes the
// snapshot root.
func normalizeSnapshotName(name string) (string, error) {
	if name == "." {
		return ".", nil
	}
	if !fs.ValidPath(name) {
		return "", fmt.Errorf("invalid path %q", name)
	}
	return name, nil
}

func (f *goSnapshotFS) Open(name string) (fs.File, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if f.isDir[clean] {
		return &snapshotDirFile{name: clean, entries: f.children[clean]}, nil
	}
	if f.isFile[clean] {
		data, err := f.readFile(clean)
		if err != nil {
			return nil, err
		}
		return &snapshotFile{name: clean, data: data}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (f *goSnapshotFS) ReadDir(name string) ([]fs.DirEntry, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	if !f.isDir[clean] {
		if f.isFile[clean] {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fmt.Errorf("not a directory")}
		}
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	entries := f.children[clean]
	out := make([]fs.DirEntry, len(entries))
	copy(out, entries)
	return out, nil
}

func (f *goSnapshotFS) ReadFile(name string) ([]byte, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	if f.isDir[clean] {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fmt.Errorf("is a directory")}
	}
	if !f.isFile[clean] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return f.readFile(clean)
}

func (f *goSnapshotFS) readFile(clean string) ([]byte, error) {
	data, err := runSnapshotGit(f.dir, maxSnapshotFileBytes, maxSnapshotGitStderr, snapshotGitTimeout, "show", f.revision+":"+clean)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: clean, Err: fmt.Errorf("git show %s:%s: %w", f.revision, clean, err)}
	}
	return data, nil
}

type snapshotDirEntry struct {
	name  string
	isDir bool
}

func (e snapshotDirEntry) Name() string { return e.name }
func (e snapshotDirEntry) IsDir() bool  { return e.isDir }
func (e snapshotDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e snapshotDirEntry) Info() (fs.FileInfo, error) {
	if e.isDir {
		return snapshotDirInfo{name: e.name}, nil
	}
	return snapshotFileInfo{name: e.name}, nil
}

type snapshotFile struct {
	name string
	data []byte
	pos  int
}

func (sf *snapshotFile) Stat() (fs.FileInfo, error) {
	return snapshotFileInfo{name: path.Base(sf.name), size: int64(len(sf.data))}, nil
}

func (sf *snapshotFile) Read(b []byte) (int, error) {
	if sf.pos >= len(sf.data) {
		return 0, io.EOF
	}
	n := copy(b, sf.data[sf.pos:])
	sf.pos += n
	return n, nil
}

func (sf *snapshotFile) Close() error { return nil }

type snapshotDirFile struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

func (sd *snapshotDirFile) Stat() (fs.FileInfo, error) {
	return snapshotDirInfo{name: path.Base(sd.name)}, nil
}

func (sd *snapshotDirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: sd.name, Err: fmt.Errorf("is a directory")}
}

func (sd *snapshotDirFile) Close() error { return nil }

func (sd *snapshotDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		remaining := sd.entries[sd.offset:]
		sd.offset = len(sd.entries)
		return remaining, nil
	}
	if sd.offset >= len(sd.entries) {
		return nil, io.EOF
	}
	end := sd.offset + n
	if end > len(sd.entries) {
		end = len(sd.entries)
	}
	batch := sd.entries[sd.offset:end]
	sd.offset = end
	return batch, nil
}

type snapshotFileInfo struct {
	name string
	size int64
}

func (i snapshotFileInfo) Name() string       { return i.name }
func (i snapshotFileInfo) Size() int64        { return i.size }
func (i snapshotFileInfo) Mode() fs.FileMode  { return 0o444 }
func (i snapshotFileInfo) ModTime() time.Time { return time.Time{} }
func (i snapshotFileInfo) IsDir() bool        { return false }
func (i snapshotFileInfo) Sys() any           { return nil }

type snapshotDirInfo struct{ name string }

func (i snapshotDirInfo) Name() string       { return i.name }
func (i snapshotDirInfo) Size() int64        { return 0 }
func (i snapshotDirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (i snapshotDirInfo) ModTime() time.Time { return time.Time{} }
func (i snapshotDirInfo) IsDir() bool        { return true }
func (i snapshotDirInfo) Sys() any           { return nil }
