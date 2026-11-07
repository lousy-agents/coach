package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// An object past the limit on one object, or past the total the snapshot may write, is not a reason to fail
// a scan that reads no such file: the tree keeps its shape and the bytes are dropped. The files the scan
// does read are written first, and the snapshot fails only when those pass the total by themselves.

type namedFile struct{ name, content string }

func snapshotOfFiles(t *testing.T, files ...namedFile) (repository string, entries []snapshotEntry) {
	t.Helper()
	repository = gitfixture.Init(t)
	for _, file := range files {
		gitfixture.CommitFile(t, repository, file.name, file.content)
	}
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

func sizeOf(t *testing.T, dir, name string) int64 {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	return info.Size()
}

func withSnapshotLimits(t *testing.T, object, total int64) {
	t.Helper()
	originalBudget, originalTotal := defaultSnapshotBudget, snapshotTotal
	t.Cleanup(func() { defaultSnapshotBudget, snapshotTotal = originalBudget, originalTotal })
	defaultSnapshotBudget.object, snapshotTotal = object, total
}

func TestWriteSnapshotFailsOnlyWhenTheFilesTheScanReadsPassTheTotalByThemselves(t *testing.T) {
	repository, entries := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.go", strings.Repeat("b", 40)}, namedFile{"c.go", strings.Repeat("c", 40)})
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

func TestWriteSnapshotKeepsTheFilesTheScanReadsAndDropsTheBytesOfTheRest(t *testing.T) {
	// the asset is listed first; the source is still written first
	repository, entries := snapshotOfFiles(t, namedFile{"a.txt", strings.Repeat("a", 60)}, namedFile{"b.go", strings.Repeat("b", 60)})
	dir := t.TempDir()

	err := writeSnapshotWithin(repository, dir, entries, 100)

	if err != nil {
		t.Fatalf("writeSnapshotWithin = %v, want nil: only a file nothing reads passes the total", err)
	}
	if got := sizeOf(t, dir, "b.go"); got != 60 {
		t.Errorf("b.go is %d bytes, want its 60: the files the scan reads are written first", got)
	}
	if got := sizeOf(t, dir, "a.txt"); got != 0 {
		t.Errorf("a.txt is %d bytes, want a zero-length file at the path", got)
	}
}

func TestWriteSnapshotAcceptsASnapshotAtExactlyItsTotal(t *testing.T) {
	repository, entries := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.go", strings.Repeat("b", 40)}, namedFile{"c.go", strings.Repeat("c", 40)})

	if err := writeSnapshotWithin(repository, t.TempDir(), entries, 120); err != nil {
		t.Errorf("writeSnapshotWithin = %v, want nil for 120 bytes against a total of 120", err)
	}
}

func TestWriteSnapshotCountsABlobOnceForEveryPathThatNamesIt(t *testing.T) {
	repository, entries := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)})
	twice := append(entries, snapshotEntry{path: "copy.go", mode: entries[0].mode, oid: entries[0].oid})

	err := writeSnapshotWithin(repository, t.TempDir(), twice, 70)

	if err == nil || !strings.Contains(err.Error(), "larger than 70 bytes") {
		t.Errorf("writeSnapshotWithin = %v, want a refusal: one 40-byte source named by two paths writes 80 bytes", err)
	}
}

func TestWriteSnapshotSpendsWhatIsLeftOnTheSmallerFilesThatStillFit(t *testing.T) {
	// the first file does not fit and is dropped; it must not use up the total the second one fits in
	repository, entries := snapshotOfFiles(t, namedFile{"a.txt", strings.Repeat("a", 90)}, namedFile{"b.txt", strings.Repeat("b", 20)})
	dir := t.TempDir()

	if err := writeSnapshotWithin(repository, dir, entries, 80); err != nil {
		t.Fatalf("writeSnapshotWithin = %v, want nil", err)
	}

	if a, b := sizeOf(t, dir, "a.txt"), sizeOf(t, dir, "b.txt"); a != 0 || b != 20 {
		t.Errorf("a.txt %d bytes, b.txt %d bytes; want a zero-length file and the 20 bytes that still fit", a, b)
	}
}

func TestExtractRevisionWritesAZeroLengthFileForAnObjectPastTheLimitOnOneObject(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"big.bin", strings.Repeat("x", 100)}, namedFile{"small.txt", "small"})
	withSnapshotLimits(t, 50, snapshotTotalBytes)

	dir, err := ExtractRevision(repository, "HEAD")
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

// The total a scan runs under is the one the constant names: ExtractRevision, which a scan calls, is held to it.
func TestExtractRevisionHoldsASnapshotToTheTotal(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.go", strings.Repeat("b", 40)})
	withSnapshotLimits(t, snapshotObjectBytes, 60)

	dir, err := ExtractRevision(repository, "HEAD")

	if err == nil || !strings.Contains(err.Error(), "larger than 60 bytes") || dir != "" {
		t.Errorf("ExtractRevision = %q, %v; want a refusal that names the total and no directory", dir, err)
	}
}

func TestExtractRevisionDropsTheBytesOfAFileNothingReadsPastTheTotal(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", strings.Repeat("a", 40)}, namedFile{"b.png", strings.Repeat("p", 40)})
	withSnapshotLimits(t, snapshotObjectBytes, 50)

	dir, err := ExtractRevision(repository, "HEAD")
	if err != nil {
		t.Fatalf("ExtractRevision = %v, want nil", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if a, p := sizeOf(t, dir, "a.go"), sizeOf(t, dir, "b.png"); a != 40 || p != 0 {
		t.Errorf("a.go %d bytes, b.png %d bytes; want 40 and a zero-length file", a, p)
	}
}

func TestTheSnapshotReadsTheFilesAnalyzersReadFirst(t *testing.T) {
	entries := []snapshotEntry{{path: "x.png"}, {path: "pkg/a.go"}, {path: "go.mod"}, {path: "y.bin"}, {path: "tsconfig.base.json"}, {path: "web/app.TSX"}}

	var got []string
	for _, entry := range neededFirst(entries) {
		got = append(got, entry.path)
	}

	want := "pkg/a.go go.mod tsconfig.base.json web/app.TSX x.png y.bin"
	if strings.Join(got, " ") != want {
		t.Errorf("neededFirst = %v, want %q: the files an analyzer reads, in listing order, then the rest", got, want)
	}
}
