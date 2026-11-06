package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each object is within the per-object limit and a snapshot of many of them still writes as much as they
// name, and a blob that many paths name is written once for each. The total is what the snapshot writes.

func snapshotOfCommittedFiles(t *testing.T, contents ...string) (repository string, entries []snapshotEntry) {
	t.Helper()
	repository, _ = committedBlobs(t, contents...)
	listing, err := RunBytes(repository, "ls-tree", "-r", "-z", "--full-tree", "HEAD")
	if err != nil {
		t.Fatalf("ls-tree: %v", err)
	}
	entries, err = parseSnapshotEntries(listing)
	if err != nil {
		t.Fatalf("parse the listing: %v", err)
	}
	return repository, entries
}

func TestWriteSnapshotRefusesWhatWouldPassItsTotalBeforeWritingIt(t *testing.T) {
	repository, entries := snapshotOfCommittedFiles(t, strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40))
	dir := t.TempDir()

	err := writeSnapshotWithin(repository, dir, entries, 100)

	if err == nil || !strings.Contains(err.Error(), "larger than 100 bytes") {
		t.Fatalf("writeSnapshotWithin = %v, want a refusal that names the 100-byte total", err)
	}
	written, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(written) != 2 {
		t.Errorf("%d files written, want the two that fit (80 bytes) and not the third: %v", len(written), written)
	}
}

func TestWriteSnapshotAcceptsASnapshotAtExactlyItsTotal(t *testing.T) {
	repository, entries := snapshotOfCommittedFiles(t, strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40))
	dir := t.TempDir()

	if err := writeSnapshotWithin(repository, dir, entries, 120); err != nil {
		t.Errorf("writeSnapshotWithin = %v, want nil for 120 bytes against a total of 120", err)
	}
}

func TestWriteSnapshotCountsABlobOnceForEveryPathThatNamesIt(t *testing.T) {
	repository, entries := snapshotOfCommittedFiles(t, strings.Repeat("a", 40))
	twice := append(entries, snapshotEntry{path: "copy.txt", mode: entries[0].mode, oid: entries[0].oid})
	dir := t.TempDir()

	err := writeSnapshotWithin(repository, dir, twice, 70)

	if err == nil || !strings.Contains(err.Error(), "larger than 70 bytes") {
		t.Errorf("writeSnapshotWithin = %v, want a refusal: one 40-byte blob named by two paths writes 80 bytes", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "copy.txt")); statErr == nil {
		t.Errorf("a blob that would pass the total was written")
	}
}

// The total a scan runs under is the one the constant names: ExtractRevision, which a scan calls, is held to it.
func TestExtractRevisionHoldsASnapshotToTheTotal(t *testing.T) {
	repository, _ := snapshotOfCommittedFiles(t, strings.Repeat("a", 40), strings.Repeat("b", 40))
	original := snapshotTotal
	t.Cleanup(func() { snapshotTotal = original })
	snapshotTotal = 60

	dir, err := ExtractRevision(repository, "HEAD")

	if err == nil || !strings.Contains(err.Error(), "larger than 60 bytes") || dir != "" {
		t.Errorf("ExtractRevision = %q, %v; want a refusal that names the total and no directory", dir, err)
	}
}
