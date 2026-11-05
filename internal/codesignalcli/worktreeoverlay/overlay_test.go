package worktreeoverlay_test

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

func committed() fstest.MapFS {
	return fstest.MapFS{
		"go.mod":                {Data: []byte("module committed\n")},
		"pkg/db/db.go":          {Data: []byte("package db // committed\n")},
		"pkg/handlers/h.go":     {Data: []byte("package handlers // committed\n")},
		"pkg/handlers/other.go": {Data: []byte("package handlers // other\n")},
		"web/src/index.ts":      {Data: []byte("export const a = 1;\n")},
		"web/tsconfig.json":     {Data: []byte("{}\n")},
		"docs/readme.md":        {Data: []byte("# docs\n")},
	}
}

// servesCase is one scenario for TestOverlayServesDiskStateForTheNamedPaths:
// the disk, the dirty paths named to New, and what the overlay must then serve.
type servesCase struct {
	name    string
	disk    map[string]diskEntry
	paths   []string
	want    map[string]string
	absent  []string
	outcome worktreeoverlay.Outcome
}

func TestOverlayServesDiskStateForTheNamedPaths(t *testing.T) {
	tests := []servesCase{
		{
			name:    "a modified tracked file is served from disk",
			disk:    map[string]diskEntry{"go.mod": regular("module edited\n")},
			paths:   []string{"go.mod"},
			want:    map[string]string{"go.mod": "module edited\n", "pkg/db/db.go": "package db // committed\n"},
			outcome: worktreeoverlay.Outcome{Overlaid: []string{"go.mod"}},
		},
		{
			name:    "a tracked file gone from disk is absent, not empty",
			disk:    map[string]diskEntry{},
			paths:   []string{"pkg/db/db.go"},
			absent:  []string{"pkg/db/db.go"},
			want:    map[string]string{"pkg/handlers/h.go": "package handlers // committed\n"},
			outcome: worktreeoverlay.Outcome{Removed: []string{"pkg/db/db.go"}},
		},
		{
			name:    "an untracked file in a new directory is added",
			disk:    map[string]diskEntry{"pkg/new/n.go": regular("package n\n")},
			paths:   []string{"pkg/new/n.go"},
			want:    map[string]string{"pkg/new/n.go": "package n\n"},
			outcome: worktreeoverlay.Outcome{Overlaid: []string{"pkg/new/n.go"}},
		},
		{
			name:    "a symbolic link is absent and its committed bytes are not served",
			disk:    map[string]diskEntry{"pkg/handlers/h.go": {mode: fs.ModeSymlink | 0o777}},
			paths:   []string{"pkg/handlers/h.go"},
			absent:  []string{"pkg/handlers/h.go"},
			want:    map[string]string{"pkg/handlers/other.go": "package handlers // other\n"},
			outcome: worktreeoverlay.Outcome{SymlinkSkipped: []string{"pkg/handlers/h.go"}},
		},
		{
			name:    "a new symbolic link is skipped too",
			disk:    map[string]diskEntry{"pkg/link.go": {mode: fs.ModeSymlink | 0o777}},
			paths:   []string{"pkg/link.go"},
			absent:  []string{"pkg/link.go"},
			outcome: worktreeoverlay.Outcome{SymlinkSkipped: []string{"pkg/link.go"}},
		},
		{
			name:    "an unreadable tracked file is absent, not served from its committed bytes, and is reported",
			disk:    map[string]diskEntry{"go.mod": {mode: 0o644, readErr: errors.New("permission denied")}},
			paths:   []string{"go.mod"},
			absent:  []string{"go.mod"},
			outcome: worktreeoverlay.Outcome{Unreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonReadFailed}}},
		},
		{
			name:    "a file that changes while it is read is absent and reported",
			disk:    map[string]diskEntry{"go.mod": {mode: 0o644, data: []byte("module racing\n"), growsAfterRead: true}},
			paths:   []string{"go.mod"},
			absent:  []string{"go.mod"},
			outcome: worktreeoverlay.Outcome{Changed: []string{"go.mod"}},
		},
		{
			name:    "a file rewritten to the same size and mode while it is read is absent and reported as changed",
			disk:    map[string]diskEntry{"go.mod": {mode: 0o644, data: []byte("module racing\n"), touchedAfterRead: true}},
			paths:   []string{"go.mod"},
			absent:  []string{"go.mod"},
			outcome: worktreeoverlay.Outcome{Changed: []string{"go.mod"}},
		},
		{
			name:    "a file swapped for another one between the stat and the open is absent and reported as changed",
			disk:    map[string]diskEntry{"go.mod": {mode: 0o644, data: []byte("module swapped\n"), replacedBeforeOpen: true}},
			paths:   []string{"go.mod"},
			absent:  []string{"go.mod"},
			outcome: worktreeoverlay.Outcome{Changed: []string{"go.mod"}},
		},
		{
			name:    "a file over the size bound is absent and reported",
			disk:    map[string]diskEntry{"go.mod": {mode: 0o644, data: []byte("x"), reportedSizeBig: true}},
			paths:   []string{"go.mod"},
			absent:  []string{"go.mod"},
			outcome: worktreeoverlay.Outcome{Unreadable: []worktreeoverlay.Unreadable{{Path: "go.mod", Reason: worktreeoverlay.ReasonTooLarge}}},
		},
		{
			name:    "a directory that replaced a tracked file removes it",
			disk:    map[string]diskEntry{"go.mod": {mode: fs.ModeDir | 0o755}},
			paths:   []string{"go.mod"},
			absent:  []string{"go.mod"},
			outcome: worktreeoverlay.Outcome{Removed: []string{"go.mod"}},
		},
		{
			name:    "a dirty path that is neither tracked nor on disk is ignored",
			disk:    map[string]diskEntry{},
			paths:   []string{"pkg/ghost.go"},
			absent:  []string{"pkg/ghost.go"},
			outcome: worktreeoverlay.Outcome{},
		},
		{
			name:    "a duplicate path is read once",
			disk:    map[string]diskEntry{"go.mod": regular("module edited\n")},
			paths:   []string{"go.mod", "go.mod"},
			want:    map[string]string{"go.mod": "module edited\n"},
			outcome: worktreeoverlay.Outcome{Overlaid: []string{"go.mod"}},
		},
		{
			name:    "paths outside the named set keep their committed bytes",
			disk:    map[string]diskEntry{"go.mod": regular("module edited\n")},
			paths:   []string{"go.mod"},
			want:    map[string]string{"docs/readme.md": "# docs\n", "web/tsconfig.json": "{}\n"},
			outcome: worktreeoverlay.Outcome{Overlaid: []string{"go.mod"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOverlayServes(t, tt)
		})
	}
}

// assertOverlayServes builds the overlay for one scenario and checks, in order,
// the outcome New reports, the bytes the overlay serves, and the paths it hides.
func assertOverlayServes(t *testing.T, tt servesCase) {
	t.Helper()
	overlay, outcome := newOverlay(t, tt.paths, newFakeDisk(tt.disk))
	assertOutcome(t, outcome, tt.outcome)
	assertServedContents(t, overlay, tt.want)
	assertAllAbsent(t, overlay, tt.absent)
}

// editedHandlerSource is what the walk tests' disk holds for pkg/handlers/h.go:
// longer than its committed copy, so a size read from the wrong side differs.
const editedHandlerSource = "package handlers // edited and longer\n"

// newWalkOverlay builds the overlay the three walk tests read: a modified file,
// an added file, a symbolic link over a tracked file, a tracked file that is
// gone from disk, and a modified file whose disk copy is longer than its
// committed copy.
func newWalkOverlay(t *testing.T) fs.FS {
	t.Helper()
	disk := newFakeDisk(map[string]diskEntry{
		"go.mod":            regular("module edited\n"),
		"pkg/new/n.go":      regular("package n\n"),
		"web/src/index.ts":  {mode: fs.ModeSymlink | 0o777},
		"pkg/handlers/h.go": regular(editedHandlerSource),
	})
	paths := []string{"go.mod", "pkg/new/n.go", "web/src/index.ts", "pkg/handlers/h.go", "pkg/db/db.go"}

	overlay, _ := newOverlay(t, paths, disk)
	return overlay
}

func TestOverlayWalkListsAddedAndModifiedFilesAndDropsRemovedOnes(t *testing.T) {
	overlay := newWalkOverlay(t)

	got := walk(t, overlay)

	want := []string{"docs/readme.md", "go.mod", "pkg/handlers/h.go", "pkg/handlers/other.go", "pkg/new/n.go", "web/tsconfig.json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("walk = %v, want %v", got, want)
	}
}

func TestOverlayStatReportsTheDiskSizeOfAModifiedFile(t *testing.T) {
	overlay := newWalkOverlay(t)

	info, err := fs.Stat(overlay, "pkg/handlers/h.go")

	if err != nil || info.Size() != int64(len(editedHandlerSource)) {
		t.Errorf("Stat(h.go) = %v, %v; the size must be the disk size", info, err)
	}
}

func TestOverlayDirectoryListingReportsTheDiskSizeOfAModifiedFile(t *testing.T) {
	overlay := newWalkOverlay(t)

	assertListedSize(t, overlay, "pkg/handlers", "h.go", int64(len(editedHandlerSource)))
}

func TestOverlayRejectsPathsThatAreNotRepositoryRelative(t *testing.T) {
	for _, bad := range []string{"", ".", "../escape.go", "/abs.go", "a//b.go", "a/../b.go"} {
		t.Run(bad, func(t *testing.T) {
			assertNewRejectsPath(t, bad)
		})
	}
}

func assertNewRejectsPath(t *testing.T, bad string) {
	t.Helper()
	if _, _, err := worktreeoverlay.New(committed(), []string{bad}, newFakeDisk(nil)); err == nil {
		t.Errorf("New accepted %q", bad)
	}
}

// A backslash is an ordinary character in a Git path on Unix, so a tracked file
// named a\b.go must not make the scan fail, and must not be read as a/b.go.
func TestOverlayTreatsABackslashAsPartOfAFileName(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{`a\b.go`: {mode: 0o644, data: []byte("package a\n")}})

	overlay, outcome, err := worktreeoverlay.New(committed(), []string{`a\b.go`}, disk)

	if err != nil {
		t.Fatalf("New rejected a path with a backslash: %v", err)
	}
	if !reflect.DeepEqual(outcome.Overlaid, []string{`a\b.go`}) {
		t.Errorf("Overlaid = %v, want the literal name", outcome.Overlaid)
	}
	if _, err := fs.Stat(overlay, "a/b.go"); err == nil {
		t.Error("the backslash was read as a path separator")
	}
}
