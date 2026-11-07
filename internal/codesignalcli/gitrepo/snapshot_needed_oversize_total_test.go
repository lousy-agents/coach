package gitrepo

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// The total a snapshot may write, and the failures of a write, hold for an object read to disk as they do for
// one that is held.

// totalCase is a snapshot of some files held to a total, and whether it is refused.
type totalCase struct {
	name  string
	files []namedFile
	total int64
	fails bool
}

func (c totalCase) run(t *testing.T) {
	repository, entries := snapshotOfFiles(t, c.files...)
	withSnapshotLimits(t, oversizeObject, c.total)

	err := writeSnapshotWithin(repository, t.TempDir(), entries, c.total)

	if c.fails != (err != nil) || (c.fails && !strings.Contains(err.Error(), "larger than")) {
		t.Errorf("writeSnapshotWithin = %v, want a refusal that names the total: %v", err, c.fails)
	}
}

// An object read to disk counts once for every path that names it, as every other object does.
func TestAnObjectReadToDiskCountsAgainstTheTotalForEveryPathThatNamesIt(t *testing.T) {
	source := "package x // " + strings.Repeat("x", 100)
	size := int64(len(source))
	cases := []totalCase{
		{"one path under the total", []namedFile{{"a.go", source}}, size, false},
		{"one path past the total", []namedFile{{"a.go", source}}, size - 1, true},
		{"two paths past the total", []namedFile{{"a.go", source}, {"b.go", source}}, size + 10, true},
		{"two paths under the total", []namedFile{{"a.go", source}, {"b.go", source}}, 2 * size, false},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

func TestAnObjectReadToDiskThatCannotBeWrittenEndsTheSnapshot(t *testing.T) {
	full := errors.New("no space left on device")
	repository, entries := snapshotOfFiles(t, namedFile{"big.go", "package big // " + strings.Repeat("b", 100)})
	withSnapshotLimits(t, oversizeObject, snapshotTotalBytes)
	withSnapshotFile(t, failingFile{writeErr: full})

	if err := writeSnapshotWithin(repository, t.TempDir(), entries, snapshotTotalBytes); !errors.Is(err, full) {
		t.Errorf("writeSnapshotWithin = %v, want the failed write of the file", err)
	}
}

// The other paths of an object read to disk are copies of the first file. A copy that finds less than the object
// holds is refused, and is not left as if it were whole.
func TestACopyOfAnObjectThatFindsLessThanTheObjectHoldsIsRefused(t *testing.T) {
	root := rootWithAFileAt(t, "a.go") // one byte

	err := copySnapshotEntry(root, snapshotEntry{mode: "100644", path: "b.go"}, snapshotEntry{mode: "100644", path: "a.go"}, 100)

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("copySnapshotEntry = %v, want io.ErrUnexpectedEOF: one byte is not the hundred the object holds", err)
	}
}
