package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
)

// A symbolic link has no zero-length form: its content is its target, and a link is never large. A blob whose
// bytes are dropped gets an empty file, and a link among its paths gets nothing.
func TestAPlaceholderIsNotWrittenForALink(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()

	err = writePlaceholders(root, []snapshotEntry{{mode: modeSymlink, path: "link", oid: "1"}, {mode: "100644", path: "assets/blob.bin", oid: "1"}})

	if err != nil {
		t.Fatalf("writePlaceholders: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "link")); !os.IsNotExist(err) {
		t.Errorf("a link was written as a placeholder: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "assets", "blob.bin")); err != nil || info.Size() != 0 {
		t.Errorf("the blob's placeholder = %v, %v; want an empty file", info, err)
	}
}
