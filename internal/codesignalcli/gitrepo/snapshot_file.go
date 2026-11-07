package gitrepo

import (
	"io"
	"os"
	"path/filepath"
)

// fileCreator opens a file of the snapshot for writing, through the root that keeps every write inside it. A test
// hands in one whose file fails to write or close (a full disk), which a real file cannot be made to do on demand.
type fileCreator func(root *os.Root, name string, permissions os.FileMode) (io.WriteCloser, error)

// createSnapshotFile is the fileCreator of a real snapshot.
func createSnapshotFile(root *os.Root, name string, permissions os.FileMode) (io.WriteCloser, error) {
	return root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, permissions)
}

// snapshotDir is the directory a snapshot is written below: the root that keeps every write inside it, and the way
// a file in it is created.
type snapshotDir struct {
	*os.Root
	create fileCreator
}

func snapshotName(entry snapshotEntry) string { return filepath.FromSlash(entry.path) }

func makeDirectoryOf(root *os.Root, name string) error {
	return root.MkdirAll(filepath.Dir(name), 0o755)
}

func createEntryFile(root snapshotDir, name string, mode string) (io.WriteCloser, error) {
	permissions := os.FileMode(0o644)
	if mode == modeExecutable {
		permissions = 0o755
	}
	return root.create(root.Root, name, permissions)
}

// closeAfter closes file and returns the error the write before it had, or else the one the close has.
func closeAfter(file io.Closer, writeErr error) error {
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func writeSnapshotEntry(root snapshotDir, entry snapshotEntry, content []byte) error {
	name := snapshotName(entry)
	if err := makeDirectoryOf(root.Root, name); err != nil {
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
func streamSnapshotEntry(root snapshotDir, entry snapshotEntry, size int64, body io.Reader) error {
	name := snapshotName(entry)
	if err := makeDirectoryOf(root.Root, name); err != nil {
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
func copySnapshotEntry(root snapshotDir, entry, from snapshotEntry, size int64) error {
	source, err := root.Open(snapshotName(from))
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	return streamSnapshotEntry(root, entry, size, source)
}
