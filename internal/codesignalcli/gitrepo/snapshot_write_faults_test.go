package gitrepo

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file where a directory must go, and a disk that cannot be written, fail the snapshot with the error of the
// step that met them. Each test makes exactly one step fail: the others have nothing to fail on.

func TestWriteSnapshotFailsWhenItsDirectoryCannotBeOpened(t *testing.T) {
	err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 1<<20), "repository", filepath.Join(t.TempDir(), "absent"), nil)

	if err == nil {
		t.Error("writeSnapshotWith = nil, want the failure to open the directory it was to write")
	}
}

func rootWithAFileAt(t *testing.T, name string) *os.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func TestMakeSubmoduleDirectoriesReturnsTheDirectoryItCouldNotMake(t *testing.T) {
	root := rootWithAFileAt(t, "x")

	blobs, err := makeSubmoduleDirectories(root, []snapshotEntry{{mode: modeGitlink, path: "x/sub", oid: "1"}})

	if err == nil || blobs != nil {
		t.Errorf("makeSubmoduleDirectories = %v, %v; want the failure to make a directory below a file", blobs, err)
	}
}

func TestWriteSnapshotEntryReturnsTheDirectoryItCouldNotMake(t *testing.T) {
	root := rootWithAFileAt(t, "x")

	if err := writeSnapshotEntry(directoryCreating(root, failingFile{}), snapshotEntry{mode: "100644", path: "x/a.go"}, []byte("package a")); err == nil {
		t.Error("writeSnapshotEntry = nil, want the failure to make a directory below a file")
	}
}

// The zero-length file that stands for the bytes a snapshot drops is written like any other, and its failure
// ends the snapshot in both places that write one.
func TestAPlaceholderThatCannotBeWrittenEndsTheSnapshot(t *testing.T) {
	full := errors.New("no space left on device")

	t.Run("a blob past the total that nothing reads", func(t *testing.T) {
		root := directoryCreating(rootWithAFileAt(t, "unrelated"), failingFile{writeErr: full})
		writer := &snapshotWriter{root: root, byObject: map[string][]snapshotEntry{"1": {{mode: "100644", path: "assets/blob.bin", oid: "1"}}}, total: 0}

		if err := writer.write("1", []byte("some bytes")); !errors.Is(err, full) {
			t.Errorf("write = %v, want the failed placeholder", err)
		}
	})

	t.Run("a blob past the limit on one object", func(t *testing.T) {
		repository, entries := snapshotOfFiles(t, namedFile{"assets/blob.bin", strings.Repeat("b", 40)})
		write := creatingFiles(snapshotWriteHeldTo(10, 1<<20), failingFile{writeErr: full})

		if err := writeSnapshotWith(write, repository, t.TempDir(), entries); !errors.Is(err, full) {
			t.Errorf("writeSnapshotWith = %v, want the failed placeholder", err)
		}
	})
}

func TestWriteSnapshotReturnsTheSubmoduleDirectoryItCouldNotMake(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", "package a"}) // a real repository, so that only the directory can fail
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("a file where a directory must go"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 1<<20), repository, dir, []snapshotEntry{{mode: modeGitlink, path: "x/sub", oid: "1"}})

	if err == nil {
		t.Error("writeSnapshotWith = nil, want the failure to make the directory of a submodule below a file")
	}
}

func TestWriteSnapshotEntryReturnsTheFileItCouldNotCreate(t *testing.T) {
	denied := errors.New("permission denied")
	cannotCreate := func(*os.Root, string, os.FileMode) (io.WriteCloser, error) { return nil, denied }

	if err := writeOneFile(t, cannotCreate); !errors.Is(err, denied) {
		t.Errorf("writeSnapshotEntry = %v, want the file that could not be created", err)
	}
}
