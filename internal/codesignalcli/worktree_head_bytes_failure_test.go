package codesignalcli

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestHeadFileBytesFailsClosedWhenTheWorkingTreeOverlayCannotBeBuilt(t *testing.T) {
	failure := errors.New("git status failed")
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    WorktreeStatus{Err: failure},
		committed: fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		disk:      absentDisk{},
	})
	committedReads := 0
	committed := func(string) ([]byte, error) {
		committedReads++
		return []byte("package a\n"), nil
	}

	data, err := headFileBytes(&view, ".", "HEAD", "a.go", committed)

	if !errors.Is(err, failure) {
		t.Errorf("headFileBytes error = %v, want the failure that stopped the overlay: %q", err, data)
	}
	if committedReads != 0 {
		t.Errorf("headFileBytes read the committed bytes %d time(s) after the overlay failed: a scan must not quietly analyze HEAD in place of the working tree", committedReads)
	}
}
