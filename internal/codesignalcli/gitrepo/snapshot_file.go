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

func snapshotName(entry snapshotEntry) string { return filepath.FromSlash(entry.path) }

func makeDirectoryOf(root *os.Root, name string) error {
	return root.MkdirAll(filepath.Dir(name), 0o755)
}

func createEntryFile(root *os.Root, name string, mode string) (io.WriteCloser, error) {
	permissions := os.FileMode(0o644)
	if mode == modeExecutable {
		permissions = 0o755
	}
	return createSnapshotFile(root, name, permissions)
}

// closeAfter closes file and returns the error the write before it had, or else the one the close has.
func closeAfter(file io.Closer, writeErr error) error {
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func writeSnapshotEntry(root *os.Root, entry snapshotEntry, content []byte) error {
	name := snapshotName(entry)
	if err := makeDirectoryOf(root, name); err != nil {
		return err
	}
	if entry.mode == modeSymlink {
		return root.Symlink(string(content), name)
	}
	file, err := createEntryFile(root, name, entry.mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	return closeAfter(file, writeErr)
}

// streamSnapshotEntry writes size bytes, read from body a chunk at a time, to the file of entry. A body that
// ends before size is an error, so that a stream cut inside an object is never taken for the object.
func streamSnapshotEntry(root *os.Root, entry snapshotEntry, size int64, body io.Reader) error {
	name := snapshotName(entry)
	if err := makeDirectoryOf(root, name); err != nil {
		return err
	}
	file, err := createEntryFile(root, name, entry.mode)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, body)
	if copyErr == nil && written != size {
		copyErr = io.ErrUnexpectedEOF
	}
	return closeAfter(file, copyErr)
}

// copySnapshotEntry writes to the file of entry what the file of from holds, size bytes.
func copySnapshotEntry(root *os.Root, entry, from snapshotEntry, size int64) error {
	source, err := root.Open(snapshotName(from))
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	return streamSnapshotEntry(root, entry, size, source)
}
