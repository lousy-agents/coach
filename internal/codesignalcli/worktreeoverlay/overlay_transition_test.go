package worktreeoverlay_test

import (
	"errors"
	"io/fs"
	"reflect"
	"sort"
	"syscall"
	"testing"
)

// transitionCase is a path whose kind changed between the commit and the disk:
// a file that became a directory or a directory that became a file.
type transitionCase struct {
	name string
	disk map[string]diskEntry
	// paths are the dirty paths named to New: the vanished entries and the new ones.
	paths []string
	// changed is the path whose kind flipped.
	changed string
	// children are the names the overlay must list under changed, or nil when
	// changed is a file.
	children []string
	// walked is every file a walk of the overlay must visit.
	walked []string
}

func transitionCases() []transitionCase {
	return []transitionCase{
		{
			name: "a tracked file that became a directory holding a new file",
			disk: map[string]diskEntry{
				"docs/readme.md":      {mode: fs.ModeDir | 0o755},
				"docs/readme.md/n.go": regular("package n\n"),
			},
			paths:    []string{"docs/readme.md", "docs/readme.md/n.go"},
			changed:  "docs/readme.md",
			children: []string{"n.go"},
			walked: []string{
				"docs/readme.md/n.go", "go.mod", "pkg/db/db.go", "pkg/handlers/h.go", "pkg/handlers/other.go",
				"web/src/index.ts", "web/tsconfig.json",
			},
		},
		{
			name: "a tracked file that became a directory whose own removal was not named",
			disk: map[string]diskEntry{
				"docs/readme.md":      {mode: fs.ModeDir | 0o755},
				"docs/readme.md/n.go": regular("package n\n"),
			},
			paths:    []string{"docs/readme.md/n.go"},
			changed:  "docs/readme.md",
			children: []string{"n.go"},
			walked: []string{
				"docs/readme.md/n.go", "go.mod", "pkg/db/db.go", "pkg/handlers/h.go", "pkg/handlers/other.go",
				"web/src/index.ts", "web/tsconfig.json",
			},
		},
		{
			name: "a tracked directory that became a file",
			disk: map[string]diskEntry{
				"pkg/db": regular("not a directory any more\n"),
			},
			paths:   []string{"pkg/db", "pkg/db/db.go"},
			changed: "pkg/db",
			walked: []string{
				"docs/readme.md", "go.mod", "pkg/db", "pkg/handlers/h.go", "pkg/handlers/other.go",
				"web/src/index.ts", "web/tsconfig.json",
			},
		},
	}
}

func TestOverlayAgreesOnWhatAPathIsWhenAFileAndADirectoryReplaceEachOther(t *testing.T) {
	for _, tt := range transitionCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertTransitionIsConsistent(t, tt)
		})
	}
}

func assertTransitionIsConsistent(t *testing.T, tt transitionCase) {
	t.Helper()
	overlay, _ := newOverlay(t, tt.paths, newFakeDisk(tt.disk))
	wantDir := tt.children != nil

	assertStatKind(t, overlay, tt.changed, wantDir)
	assertOpenKind(t, overlay, tt.changed, wantDir)
	assertReadFileKind(t, overlay, tt.changed, wantDir)
	assertReadDirKind(t, overlay, tt.changed, tt.children)
	if got := walk(t, overlay); !reflect.DeepEqual(got, tt.walked) {
		t.Errorf("walk = %v, want %v", got, tt.walked)
	}
}

func assertStatKind(t *testing.T, overlay fs.FS, name string, wantDir bool) {
	t.Helper()
	info, err := fs.Stat(overlay, name)
	if err != nil || info.IsDir() != wantDir {
		t.Errorf("Stat(%q) = dir:%v, %v; want dir:%v", name, info != nil && info.IsDir(), err, wantDir)
	}
}

func assertOpenKind(t *testing.T, overlay fs.FS, name string, wantDir bool) {
	t.Helper()
	file, err := overlay.Open(name)
	if err != nil {
		t.Errorf("Open(%q): %v", name, err)
		return
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || info.IsDir() != wantDir {
		t.Errorf("Open(%q).Stat() = dir:%v, %v; want dir:%v", name, info != nil && info.IsDir(), err, wantDir)
	}
}

func assertReadFileKind(t *testing.T, overlay fs.FS, name string, wantDir bool) {
	t.Helper()
	data, err := fs.ReadFile(overlay, name)
	switch {
	case wantDir && !errors.Is(err, syscall.EISDIR):
		t.Errorf("ReadFile(%q) = %q, %v; a directory must fail with EISDIR, not serve the bytes it replaced", name, data, err)
	case !wantDir && err != nil:
		t.Errorf("ReadFile(%q): %v", name, err)
	}
}

// assertReadDirKind checks the listing of name: its children when it is a
// directory, ENOTDIR when it is a file.
func assertReadDirKind(t *testing.T, overlay fs.FS, name string, children []string) {
	t.Helper()
	entries, err := fs.ReadDir(overlay, name)
	if children == nil {
		if !errors.Is(err, syscall.ENOTDIR) {
			t.Errorf("ReadDir(%q) = %d entries, %v; a file must fail with ENOTDIR, not list as an empty directory", name, len(entries), err)
		}
		return
	}
	if err != nil {
		t.Errorf("ReadDir(%q): %v", name, err)
		return
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, children) {
		t.Errorf("ReadDir(%q) = %v, want %v", name, got, children)
	}
}
