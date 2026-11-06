package worktreeoverlay_test

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

type listingCase struct {
	name  string
	disk  map[string]diskEntry
	paths []string
	dir   string
	want  []string
}

func listingCases() []listingCase {
	return []listingCase{
		{
			name: "files added around the committed ones in a directory both layers fill",
			disk: map[string]diskEntry{
				"pkg/handlers/z.go": regular("package handlers // z\n"),
				"pkg/handlers/a.go": regular("package handlers // a\n"),
			},
			paths: []string{"pkg/handlers/z.go", "pkg/handlers/a.go"},
			dir:   "pkg/handlers",
			want:  []string{"a.go", "h.go", "other.go", "z.go"},
		},
		{
			name: "files added to a directory the overlay creates",
			disk: map[string]diskEntry{
				"pkg/new/b.go": regular("package n // b\n"),
				"pkg/new/a.go": regular("package n // a\n"),
				"pkg/new/c.go": regular("package n // c\n"),
			},
			paths: []string{"pkg/new/b.go", "pkg/new/a.go", "pkg/new/c.go"},
			dir:   "pkg/new",
			want:  []string{"a.go", "b.go", "c.go"},
		},
	}
}

func TestOverlayListsADirectorySortedByFilename(t *testing.T) {
	for _, tc := range listingCases() {
		t.Run(tc.name, func(t *testing.T) { assertListedInFilenameOrder(t, tc) })
	}
}

func assertListedInFilenameOrder(t *testing.T, tc listingCase) {
	t.Helper()
	overlay, _, err := worktreeoverlay.New(committed(), tc.paths, newFakeDisk(tc.disk))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entries, err := fs.ReadDir(overlay, tc.dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", tc.dir, err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	if !slices.Equal(got, tc.want) {
		t.Errorf("ReadDir(%q) = %v, want %v in filename order", tc.dir, got, tc.want)
	}
}
