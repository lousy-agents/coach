package codesignalcli

import (
	"io/fs"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// worktreeViewDeps names every collaborator a WorktreeView reads through. A test
// states all of them, so none of its answers comes from the real git or disk
// behind the production defaults, and the outcome does not depend on the
// machine's git configuration or filesystem.
type worktreeViewDeps struct {
	status     WorktreeStatus
	committed  fs.FS
	disk       worktreeoverlay.DiskSource
	sparse     gitrepo.SparseState
	ignoreCase bool
	gitlinks   []string

	// The errors a read returns when a test needs git to fail it. The zero value
	// is a read that succeeds.
	sparseErr   error
	gitlinksErr error
}

func newWorktreeViewForTest(t *testing.T, deps worktreeViewDeps) WorktreeView {
	t.Helper()
	if deps.committed == nil || deps.disk == nil {
		t.Fatal("a WorktreeView test states its committed snapshot and its disk")
	}
	return WorktreeView{
		status:      deps.status,
		committed:   func(string, string) (fs.FS, error) { return deps.committed, nil },
		diskAt:      func(string) worktreeoverlay.DiskSource { return deps.disk },
		sparseState: func(string) (gitrepo.SparseState, error) { return deps.sparse, deps.sparseErr },
		ignoreCase:  func(string) (bool, error) { return deps.ignoreCase, nil },
		gitlinks:    func(string) ([]string, error) { return deps.gitlinks, deps.gitlinksErr },
	}
}

// statusFromPorcelain is the status a scan would have read from this
// `git status --porcelain=v1 -z` output, or the error that stopped the read.
func statusFromPorcelain(output []byte, err error) WorktreeStatus {
	if err != nil {
		return WorktreeStatus{Err: err}
	}
	return WorktreeStatus{Entries: gitrepo.ParseWorktreeStatus(output)}
}

// absentDisk holds nothing: every path is missing from it.
type absentDisk struct{}

func (absentDisk) Lstat(name string) (fs.FileInfo, error) {
	return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
}

func (absentDisk) ReadFile(name string, _ fs.FileInfo) ([]byte, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (absentDisk) ReadDirNames(string) ([]string, error) { return nil, nil }
