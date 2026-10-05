package codesignalcli

import (
	"fmt"

	"io/fs"
)

func (f *goSnapshotFS) readFile(clean string) ([]byte, error) {
	data, err := runSnapshotGit(f.dir, maxSnapshotFileBytes, maxSnapshotGitStderr, snapshotGitTimeout, "show", f.revision+":"+clean)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: clean, Err: fmt.Errorf("git show %s:%s: %w", f.revision, clean, err)}
	}
	return data, nil
}
