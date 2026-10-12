// Package revisionfs serves one immutable Git revision as a read-only fs.FS,
// so project analysis never reads the live worktree.
package revisionfs

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// revisionFS is a read-only fs.FS over one immutable Git revision. Every
// tracked path is enumerated once at construction; file reads are served
// lazily via `git show` so a large repository is not fully prefetched into
// memory. A later change could batch reads via `git cat-file --batch` (see
// gitrepo.RevisionFileReader) if per-file `git show` latency becomes
// a measured problem; that is out of scope here.
type revisionFS struct {
	dir      string
	revision string
	isDir    map[string]bool
	isFile   map[string]bool
	sizes    map[string]int64
	children map[string][]fs.DirEntry
}

// ListError wraps a failed `git ls-tree` listing (New's
// construction step). Error() renders the same human-readable text the
// previous fmt.Errorf-based message did, dir included, for any consumer
// that just wants a message to display; Unwrap exposes the underlying git
// failure alone, with no path interpolated, for a consumer that must not
// leak dir into a diagnostic (see New's doc comment).
type ListError struct {
	Revision string
	Dir      string
	Err      error
}

func (e *ListError) Error() string {
	return fmt.Sprintf("coach: git ls-tree failed for revision %q in %q: %s", e.Revision, e.Dir, e.Err)
}

func (e *ListError) Unwrap() error { return e.Err }

// New returns an fs.FS that reads every file tracked at
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
// calls (reusing gitrepo.RunBytesBounded's shared implementation). It implements fs.FS, fs.ReadDirFS, fs.ReadFileFS, and
// fs.StatFS so that fs.WalkDir, fs.ReadFile, and fs.Stat all work
// efficiently: fs.Stat in particular must resolve from the cached listing
// alone, never by falling back to Open (which would run `git show` and
// buffer an entire blob's content just to report its length).
//
// If revision is unresolvable or dir is not a Git repository, New
// returns an error; it never returns a silently empty FS for such a failure.
// A failed listing is returned as *ListError, not a bare
// fmt.Errorf, so a caller that must not leak dir into a diagnostic message
// (SuggestProjectConfig's snapshotUnavailableMessage) can render its own
// message from Unwrap() alone instead of scrubbing dir out of formatted
// text via substring match.
func New(dir, revision string) (fs.FS, error) {
	if revision == "" {
		return nil, fmt.Errorf("coach: revision must be a non-empty Git revision")
	}

	output, err := runSnapshotGit(dir, MaxListBytes, maxSnapshotGitStderr, snapshotGitTimeout, "ls-tree", "-r", "-z", "-l", revision)
	if err != nil {
		return nil, &ListError{Revision: revision, Dir: dir, Err: err}
	}

	fsys := &revisionFS{
		dir:      dir,
		revision: revision,
		isFile:   map[string]bool{},
		sizes:    map[string]int64{},
	}
	paths := snapshotPaths{childSets: map[string]map[string]bool{}, isDir: map[string]bool{".": true}}

	for _, entry := range gitrepo.SplitNULPaths(output) {
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
