package codesignalcli

import (
	"context"
	"fmt"

	"io/fs"

	"os/exec"
	"path"

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
	sizes    map[string]int64
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
// The returned fs.FS enumerates the full file list, and every file's blob
// size, once via one bounded `git ls-tree -r -z -l <revision>` call at
// construction time (the `-l` long-format flag reuses the same listing call
// to carry sizes too, rather than a second git invocation), then serves
// individual file reads lazily via bounded `git show <revision>:<path>`
// calls (reusing runGitBytesBounded's shared implementation from
// project.go). It implements fs.FS, fs.ReadDirFS, fs.ReadFileFS, and
// fs.StatFS so that fs.WalkDir, fs.ReadFile, and fs.Stat all work
// efficiently: fs.Stat in particular must resolve from the cached listing
// alone, never by falling back to Open (which would run `git show` and
// buffer an entire blob's content just to report its length).
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

	output, err := runSnapshotGit(dir, maxSnapshotListBytes, maxSnapshotGitStderr, snapshotGitTimeout, "ls-tree", "-r", "-z", "-l", revision)
	if err != nil {
		return nil, &snapshotListError{revision: revision, dir: dir, err: err}
	}

	fsys := &goSnapshotFS{
		dir:      dir,
		revision: revision,
		isFile:   map[string]bool{},
		sizes:    map[string]int64{},
	}
	paths := snapshotPaths{childSets: map[string]map[string]bool{}, isDir: map[string]bool{".": true}}

	for _, entry := range splitNULPaths(output) {
		p, size, err := parseSnapshotLsTreeEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("coach: git ls-tree reported an unparseable entry %q: %w", entry, err)
		}
		if err := validateSnapshotPath(p); err != nil {
			return nil, fmt.Errorf("coach: git ls-tree reported an unsafe path %q: %w", p, err)
		}
		fsys.isFile[p] = true
		fsys.sizes[p] = size
		paths.add(p)
	}

	fsys.isDir = paths.isDir
	fsys.children = finalizeSnapshotChildren(paths.childSets)
	return fsys, nil
}

// parseSnapshotLsTreeEntry parses one `git ls-tree -l` entry (already split
// on the `-z` NUL terminator) into its path and blob size. The long format
// is "<mode> SP <type> SP <object> SP <size> TAB <path>"; size is
// whitespace-padded, not tab-separated from the preceding fields, so the
// metadata is split by field first and the path is taken verbatim after the
// first tab (a path may itself contain spaces). git reports a size it could
// not determine as a non-numeric sentinel: "-" for a non-blob entry (e.g. a
// submodule gitlink), and the literal "BAD" for a blob whose object is
// missing or corrupt from the local object store. Both are parsed here as
// size 0 rather than rejected: this construction step must not be the place
// a missing blob surfaces as a failure, since erroring the whole listing
// here would discard the specific path the caller needs to report -- the
// existing per-path failure instead surfaces naturally, with that path
// intact, the moment something actually tries to read the blob's content.

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

type snapshotPaths struct {
	childSets map[string]map[string]bool
	isDir     map[string]bool
}

// normalizeSnapshotName validates a caller-supplied fs.FS name per the
// io/fs contract (slash-separated, no ./.. elements); "." denotes the
// snapshot root.

// Stat implements fs.StatFS entirely from the listing cached at
// construction, with no git child process of its own -- in particular, it
// never falls back to Open+git-show the way the io/fs package's own
// fs.Stat helper would if this method were absent, which would buffer an
// entire blob's content just to report its length.

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

type snapshotFile struct {
	name string
	data []byte
	pos  int
}

func (sf *snapshotFile) Stat() (fs.FileInfo, error) {
	return snapshotFileInfo{name: path.Base(sf.name), size: int64(len(sf.data))}, nil
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
