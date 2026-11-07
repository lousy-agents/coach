package projectconfig

import (
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// checkReadable opens the entry and closes it again, so a file that exists but
// cannot be read is unusable on disk instead of being replaced by its committed
// copy. The open does not wait for a writer, and the opened handle, not the
// earlier stat, decides that the entry is a regular file: a path swapped for a
// FIFO between the two would otherwise block the scan on the open.
func checkReadable(root diskRoot, repoPath string) error {
	file, err := root.OpenFile(filepath.FromSlash(repoPath), os.O_RDONLY|openWithoutWaiting, 0)
	if err != nil {
		return classifyOpenError(repoPath, err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return classifyOpenError(repoPath, err)
	}
	if !info.Mode().IsRegular() {
		return unusableOnDiskError(repoPath, projectreadiness.CauseNotARegularFile, "is not a regular file")
	}
	return nil
}
