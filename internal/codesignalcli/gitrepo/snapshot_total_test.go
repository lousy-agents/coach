package gitrepo

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSnapshotFailsOnlyWhenTheFilesTheScanReadsPassTheTotalByThemselves(t *testing.T) {
	repository, entries := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.go", strings.Repeat("b", 40)}, namedFile{"c.go", strings.Repeat("c", 40)})
	dir := t.TempDir()

	err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 100), repository, dir, entries)

	if err == nil || !strings.Contains(err.Error(), "larger than 100 bytes") {
		t.Fatalf("writeSnapshotWith = %v, want a refusal that names the 100-byte total", err)
	}
	written, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(written) != 2 {
		t.Errorf("%d files written, want the two that fit (80 bytes) and not the third: %v", len(written), written)
	}
}

func TestWriteSnapshotAcceptsASnapshotAtExactlyItsTotal(t *testing.T) {
	repository, entries := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.go", strings.Repeat("b", 40)}, namedFile{"c.go", strings.Repeat("c", 40)})

	if err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 120), repository, t.TempDir(), entries); err != nil {
		t.Errorf("writeSnapshotWith = %v, want nil for 120 bytes against a total of 120", err)
	}
}

func TestWriteSnapshotCountsABlobOnceForEveryPathThatNamesIt(t *testing.T) {
	repository, entries := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)})
	twice := append(entries, snapshotEntry{path: "copy.go", mode: entries[0].mode, oid: entries[0].oid})

	err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 70), repository, t.TempDir(), twice)

	if err == nil || !strings.Contains(err.Error(), "larger than 70 bytes") {
		t.Errorf("writeSnapshotWith = %v, want a refusal: one 40-byte source named by two paths writes 80 bytes", err)
	}
}

// The total a scan runs under is the one the constant names: ExtractRevision, which a scan calls, is held to it.
func TestExtractRevisionHoldsASnapshotToTheTotal(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.go", strings.Repeat("b", 40)})
	write := snapshotWriteHeldTo(snapshotObjectBytes, 60)

	dir, err := extractedWith(write, repository)

	if err == nil || !strings.Contains(err.Error(), "larger than 60 bytes") || dir != "" {
		t.Errorf("ExtractRevision = %q, %v; want a refusal that names the total and no directory", dir, err)
	}
}
