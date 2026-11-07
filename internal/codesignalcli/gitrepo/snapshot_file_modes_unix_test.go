//go:build unix

package gitrepo

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A file of the snapshot is written readable by everyone who can read the tree, and executable only when the
// tree says so; a file written 0600 would be unreadable to a tool that runs as another user.
func TestSnapshotFilesAreWrittenWithTheModesTheTreeNames(t *testing.T) {
	previous := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previous) })
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	dir := root.Name()

	for _, entry := range []snapshotEntry{{mode: "100644", path: "plain.txt"}, {mode: "100755", path: "run.sh"}} {
		if err := writeSnapshotEntry(realDirectory(root), entry, []byte("x")); err != nil {
			t.Fatalf("writeSnapshotEntry(%s): %v", entry.path, err)
		}
	}

	for name, want := range map[string]os.FileMode{"plain.txt": 0o644, "run.sh": 0o755} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != want {
			t.Errorf("%s = %v, %v; want mode %o", name, info, err, want)
		}
	}
}
