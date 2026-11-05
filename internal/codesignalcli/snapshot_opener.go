package codesignalcli

import (
	"io/fs"

	"github.com/lousy-agents/coach/internal/codesignalcli/revisionfs"
)

// SnapshotOpener opens the read-only file view one revision's project model is
// built from. A nil opener means the committed revision.
type SnapshotOpener func(dir, revision string) (fs.FS, error)

func committedSnapshot(dir, revision string) (fs.FS, error) {
	return revisionfs.New(dir, revision)
}

func snapshotOpenerOrCommitted(open SnapshotOpener) SnapshotOpener {
	if open == nil {
		return committedSnapshot
	}
	return open
}
