package worktreeoverlay_test

import (
	"errors"
	"io"
	"io/fs"
	"slices"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// openedDirectory is the handle the overlay returns for a directory, which has to list its entries.
func openedDirectory(t *testing.T, overlay fs.FS, name string) fs.ReadDirFile {
	t.Helper()
	handle, err := overlay.Open(name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	dir, ok := handle.(fs.ReadDirFile)
	if !ok {
		t.Fatalf("Open(directory) returned %T, which cannot list its entries", handle)
	}
	return dir
}

// pagedNames lists dir pages of two entries at a time until the handle reports the end, or fails if it never does.
func pagedNames(t *testing.T, dir fs.ReadDirFile) []string {
	t.Helper()
	var names []string
	for pages := 0; pages <= 10; pages++ {
		page, err := dir.ReadDir(2)
		for _, entry := range page {
			names = append(names, entry.Name())
		}
		if errors.Is(err, io.EOF) {
			return names
		}
		if err != nil {
			t.Fatalf("ReadDir(2): %v", err)
		}
	}
	t.Fatalf("ReadDir(2) never reported the end of the listing; names so far: %v", names)
	return nil
}

func TestOverlayDirectoryHandleEndsAPagedListingWithEOF(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"pkg/handlers/a.go": regular("package handlers // a\n")})
	overlay, _, err := worktreeoverlay.New(committed(), []string{"pkg/handlers/a.go"}, disk)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	names := pagedNames(t, openedDirectory(t, overlay, "pkg/handlers"))

	if want := []string{"a.go", "h.go", "other.go"}; !slices.Equal(names, want) {
		t.Errorf("paged listing = %v, want %v: every entry once, in order", names, want)
	}
}

func TestOverlayDirectoryHandleReportsEOFAfterTheLastEntryWasRead(t *testing.T) {
	overlay, _, err := worktreeoverlay.New(committed(), nil, newFakeDisk(nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dir := openedDirectory(t, overlay, "docs")

	if entries, err := dir.ReadDir(1); err != nil || len(entries) != 1 {
		t.Fatalf("first ReadDir(1) = %v, %v; want the one entry and no error", entries, err)
	}
	if entries, err := dir.ReadDir(1); !errors.Is(err, io.EOF) || len(entries) != 0 {
		t.Errorf("ReadDir(1) past the last entry = %v, %v; want no entries and io.EOF", entries, err)
	}
}
