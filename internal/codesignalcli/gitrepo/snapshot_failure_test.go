package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A snapshot that cannot be built leaves nothing behind, and a write that fails is a failure and not a
// truncated file.

// failingFile is a snapshot file whose write or close fails, as a full disk makes them.
type failingFile struct{ writeErr, closeErr error }

func (f failingFile) Write([]byte) (int, error) { return 0, f.writeErr }
func (f failingFile) Close() error              { return f.closeErr }

func writeOneFile(t *testing.T, create fileCreator) error {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return writeSnapshotEntry(snapshotDir{Root: root, create: create}, snapshotEntry{mode: "100644", path: "a.go"}, []byte("package a"))
}

func TestWriteSnapshotEntryReturnsAWriteThatFailed(t *testing.T) {
	full := errors.New("no space left on device")

	if err := writeOneFile(t, creating(failingFile{writeErr: full})); !errors.Is(err, full) {
		t.Errorf("writeSnapshotEntry = %v, want the failed write: a full disk must not leave a truncated file and go on", err)
	}
}

func TestWriteSnapshotEntryReturnsACloseThatFailed(t *testing.T) {
	failed := errors.New("close failed")

	if err := writeOneFile(t, creating(failingFile{closeErr: failed})); !errors.Is(err, failed) {
		t.Errorf("writeSnapshotEntry = %v, want the failed close", err)
	}
}

func TestExtractRevisionFailsAndLeavesNothingWhenAWriteFails(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", "package a"})
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)
	full := errors.New("no space left on device")
	write := creatingFiles(defaultSnapshotWrite(), failingFile{writeErr: full})

	dir, err := extractedWith(write, repository)

	if !errors.Is(err, full) || dir != "" {
		t.Errorf("ExtractRevision = %q, %v; want the failed write and no directory", dir, err)
	}
	assertNoSnapshotLeft(t, temporary)
}

func TestExtractRevisionLeavesNothingInTheTemporaryDirectoryWhenThePolicyRefusesIt(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.go", strings.Repeat("b", 40)})
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)
	write := snapshotWriteHeldTo(snapshotObjectBytes, 60)

	if _, err := extractedWith(write, repository); err == nil {
		t.Fatal("ExtractRevision = nil, want the refusal of a snapshot past its total")
	}

	assertNoSnapshotLeft(t, temporary)
}

func assertNoSnapshotLeft(t *testing.T, temporary string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(temporary, "coach-codesignal-snapshot-*"))
	if len(left) != 0 {
		t.Errorf("a failed snapshot left %v in the temporary directory", left)
	}
}

// A blob named by many paths is one object in the read, and counts once per path against the total.
func TestGroupByObjectListsEachObjectOnceWhateverNumberOfPathsNameIt(t *testing.T) {
	entries := []snapshotEntry{{oid: "one", path: "a"}, {oid: "two", path: "b"}, {oid: "one", path: "c"}, {oid: "one", path: "d"}}

	order, byObject := groupByObject(entries)

	if strings.Join(order, " ") != "one two" || len(byObject["one"]) != 3 || len(byObject["two"]) != 1 {
		t.Errorf("groupByObject = %v, %v; want each object once, in order of first appearance, with all its paths", order, byObject)
	}
}

func TestWriteSnapshotAcceptsABlobNamedByManyPathsAtExactlyItsTotal(t *testing.T) {
	repository, entries := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)})
	copies := append(entries, snapshotEntry{path: "b.go", mode: entries[0].mode, oid: entries[0].oid}, snapshotEntry{path: "c.go", mode: entries[0].mode, oid: entries[0].oid})

	if err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 120), repository, t.TempDir(), copies); err != nil {
		t.Errorf("writeSnapshotWith = %v, want nil: 40 bytes named by three paths is 120, once", err)
	}
}
