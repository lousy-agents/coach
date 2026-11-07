package gitrepo

import (
	"io"
	"os"
	"path/filepath"
)

// createSnapshotFile opens a file of the snapshot for writing, through the root that keeps every write inside
// it. It is a variable so that a test can make a write fail (a full disk), which a real file cannot be made to
// do on demand; production code never assigns it.
var createSnapshotFile = func(root *os.Root, name string, permissions os.FileMode) (io.WriteCloser, error) {
	return root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, permissions)
}

func writeSnapshotEntry(root *os.Root, entry snapshotEntry, content []byte) error {
	name := filepath.FromSlash(entry.path)
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	if entry.mode == modeSymlink {
		return root.Symlink(string(content), name)
	}
	permissions := os.FileMode(0o644)
	if entry.mode == modeExecutable {
		permissions = 0o755
	}
	file, err := createSnapshotFile(root, name, permissions)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
