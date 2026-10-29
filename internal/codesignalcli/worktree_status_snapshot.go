package codesignalcli

import "github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"

// CaptureWorktreeStatus reads the status a scan keeps after it creates a
// worktree file the report must not treat as customer dirt.
func CaptureWorktreeStatus(dir string) ([]gitrepo.WorktreeEntry, error) {
	return listWorktreeStatus(dir)
}

// UseWorktreeStatusSnapshot makes later disclosure and project worktree
// diagnostics read the captured status until restore runs. Paths already
// in that capture stay visible; a file created after the capture does not
// appear.
func UseWorktreeStatusSnapshot(entries []gitrepo.WorktreeEntry, statusErr error) (restore func()) {
	previous := listWorktreeStatus
	captured := append([]gitrepo.WorktreeEntry(nil), entries...)
	listWorktreeStatus = func(string) ([]gitrepo.WorktreeEntry, error) {
		if statusErr != nil {
			return nil, statusErr
		}
		return append([]gitrepo.WorktreeEntry(nil), captured...), nil
	}
	return func() { listWorktreeStatus = previous }
}
