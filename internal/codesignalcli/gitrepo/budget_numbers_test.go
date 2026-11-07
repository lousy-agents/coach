package gitrepo

import (
	"testing"
	"time"
)

// The numbers a read of a scan is held to are pinned here and in the table test next to RunBytes, so that a
// change to one is a decision.

// The snapshot stream is not a RunBytes read, so its limits are constants the stream's own code takes; the
// numbers are pinned here, with the table, so that a change is a decision.
func TestTheSnapshotStreamHasAWallTimeAnObjectSizeAndATotal(t *testing.T) {
	if snapshotWallTime != 10*time.Minute || snapshotObjectBytes != 32<<20 || snapshotTotalBytes != 4<<30 {
		t.Errorf("snapshot limits = %s, %d bytes per object, %d bytes in all; want 10m0s, %d, %d", snapshotWallTime, int64(snapshotObjectBytes), int64(snapshotTotalBytes), int64(32<<20), int64(4<<30))
	}
	if defaultSnapshotBudget.wall != snapshotWallTime || defaultSnapshotBudget.object != snapshotObjectBytes || snapshotTotal != snapshotTotalBytes {
		t.Errorf("the default snapshot budget = %+v and total %d, want the wall time, object size and total above", defaultSnapshotBudget, snapshotTotal)
	}
}

// The --baseline reader is held to the numbers of every other read of one file.
func TestTheBaselineReaderIsHeldToTheNumbersOfEveryOtherFileRead(t *testing.T) {
	if fileReadBudget.wall != 30*time.Second || fileReadBudget.object != 32<<20 {
		t.Errorf("fileReadBudget = %+v, want 30s and 32 MiB, as the show row has", fileReadBudget)
	}
}

// The reads that name a budget of their own take their numbers from the table's file; a package aliases them
// (projectconfig.MaxBytes is CommittedFileBytes) and this pins each one.
func TestTheNamedReadsHoldTheirNumbersInOnePlace(t *testing.T) {
	cases := []struct {
		name      string
		got, want int64
	}{
		{"a committed file: bytes (also a policy's size limit)", CommittedFileBytes, 1 << 20},
		{"a committed file: seconds", int64(CommittedFileTimeout / time.Second), 30},
		{"stderr of every named read", GitStderrBytes, 64 << 10},
		{"git status: bytes", WorktreeStatusBytes, 16 << 20},
		{"git status: seconds", int64(WorktreeStatusTimeout / time.Second), 30},
		{"the setup residue status: bytes", SetupResidueBytes, 1 << 20},
		{"the setup residue status: seconds", int64(SetupResidueTimeout / time.Second), 10},
		{"the worktree root: bytes", WorktreeRootBytes, 4 << 10},
		{"the worktree root: stderr", WorktreeRootStderr, 4 << 10},
		{"the worktree root: seconds", int64(WorktreeRootTimeout / time.Second), 10},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
