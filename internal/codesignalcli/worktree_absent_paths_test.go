package codesignalcli

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestAPathNeitherHeadNorDiskHoldsIsAbsentRatherThanReadFromHead(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    statusFromPorcelain(porcelainZ("?? ghost.go"), nil),
		committed: fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		disk:      absentDisk{},
	})

	_, covered, err := view.HeadBytes(".", "HEAD", "ghost.go")

	if !covered || !errors.Is(err, errAbsentFromWorktree) {
		t.Errorf("HeadBytes(ghost.go) covered=%t err=%v, want covered with errAbsentFromWorktree: a committed read of a path HEAD lacks fails with git's own error", covered, err)
	}
}
