package gitrepo

import (
	"os"
	"path/filepath"
)

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
	file, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, permissions)
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
