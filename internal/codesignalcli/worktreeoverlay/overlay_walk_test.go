package worktreeoverlay_test

import (
	"io/fs"
	"slices"
	"sort"
	"testing"
)

func walk(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	var files []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	sort.Strings(files)
	return files
}

// assertWalkDoesNotList fails when a walk of the overlay still lists name.
func assertWalkDoesNotList(t *testing.T, overlay fs.FS, name string) {
	t.Helper()
	for _, listed := range walk(t, overlay) {
		if listed == name {
			t.Errorf("WalkDir listed %q, which must be absent", name)
		}
	}
}

// assertListedSize fails unless the listing of dir names the file and reports
// want bytes for it.
func assertListedSize(t *testing.T, overlay fs.FS, dir, name string, want int64) {
	t.Helper()
	entries, err := fs.ReadDir(overlay, dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	at := slices.IndexFunc(entries, func(entry fs.DirEntry) bool { return entry.Name() == name })
	if at < 0 {
		t.Fatalf("ReadDir(%q) did not list %q", dir, name)
	}
	info, err := entries[at].Info()
	if err != nil {
		t.Fatalf("Info of %q in %q: %v", name, dir, err)
	}
	if info.Size() != want {
		t.Errorf("listing size of %s = %d, want the disk size %d", name, info.Size(), want)
	}
}
