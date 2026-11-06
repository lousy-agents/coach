package gitrepo

import "time"

// The bounds of a snapshot read. A snapshot streams to disk one object at a time, so what is held in memory
// is the largest object, and an object past the limit is refused from its size alone, before any of it is
// read. The wall time keeps git that stops answering from holding the scan. 512 MiB is the working-set
// budget the rest of the git reads share.
const (
	snapshotWallTime    = 10 * time.Minute
	snapshotObjectBytes = 512 << 20
)

// snapshotBudget is the wall time of the git process that reads a snapshot's objects, and the most bytes of
// content one object may have.
type snapshotBudget struct {
	wall   time.Duration
	object int64
}

var defaultSnapshotBudget = snapshotBudget{wall: snapshotWallTime, object: snapshotObjectBytes}
