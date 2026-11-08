package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSnapshotKeepsTheFilesTheScanReadsAndDropsTheBytesOfTheRest(t *testing.T) {
	// the asset is listed first; the source is still written first
	repository, entries := snapshotOfFiles(t, namedFile{"a.txt", strings.Repeat("a", 60)}, namedFile{"b.go", strings.Repeat("b", 60)})
	dir := t.TempDir()

	err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 100), repository, dir, entries)

	if err != nil {
		t.Fatalf("writeSnapshotWith = %v, want nil: only a file nothing reads passes the total", err)
	}
	if got := sizeOf(t, dir, "b.go"); got != 60 {
		t.Errorf("b.go is %d bytes, want its 60: the files the scan reads are written first", got)
	}
	if got := sizeOf(t, dir, "a.txt"); got != 0 {
		t.Errorf("a.txt is %d bytes, want a zero-length file at the path", got)
	}
}

func TestWriteSnapshotSpendsWhatIsLeftOnTheSmallerFilesThatStillFit(t *testing.T) {
	// the first file does not fit and is dropped; it must not use up the total the second one fits in
	repository, entries := snapshotOfFiles(t, namedFile{"a.txt", strings.Repeat("a", 90)}, namedFile{"b.txt", strings.Repeat("b", 20)})
	dir := t.TempDir()

	if err := writeSnapshotWith(snapshotWriteHeldTo(snapshotObjectBytes, 80), repository, dir, entries); err != nil {
		t.Fatalf("writeSnapshotWith = %v, want nil", err)
	}

	if a, b := sizeOf(t, dir, "a.txt"), sizeOf(t, dir, "b.txt"); a != 0 || b != 20 {
		t.Errorf("a.txt %d bytes, b.txt %d bytes; want a zero-length file and the 20 bytes that still fit", a, b)
	}
}

func TestExtractRevisionWritesAZeroLengthFileForAnObjectPastTheLimitOnOneObject(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"big.bin", strings.Repeat("x", 100)}, namedFile{"small.txt", "small"})
	write := snapshotWriteHeldTo(50, snapshotTotalBytes)

	dir, err := extractedWith(write, repository)
	if err != nil {
		t.Fatalf("ExtractRevision = %v, want nil: an object past the limit is not a reason to refuse a scan", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if got := sizeOf(t, dir, "big.bin"); got != 0 {
		t.Errorf("big.bin is %d bytes, want a zero-length file: the path stays and the bytes go", got)
	}
	if content, _ := os.ReadFile(filepath.Join(dir, "small.txt")); string(content) != "small" {
		t.Errorf("small.txt = %q, want %q: the stream must stay in step after the discarded object", content, "small")
	}
}

func TestExtractRevisionDropsTheBytesOfAFileNothingReadsPastTheTotal(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.png", strings.Repeat("p", 40)})
	write := snapshotWriteHeldTo(snapshotObjectBytes, 50)

	dir, err := extractedWith(write, repository)
	if err != nil {
		t.Fatalf("ExtractRevision = %v, want nil", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if a, p := sizeOf(t, dir, "a.go"), sizeOf(t, dir, "b.png"); a != 40 || p != 0 {
		t.Errorf("a.go %d bytes, b.png %d bytes; want 40 and a zero-length file", a, p)
	}
}
