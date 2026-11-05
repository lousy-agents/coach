package worktreeoverlay_test

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

type fakeDisk struct {
	entries map[string]diskEntry
	lstats  map[string]int
	// listings counts ReadDirNames calls per directory.
	listings map[string]int
	// foldCase makes Lstat and ReadFile match names the way APFS does: without
	// regard to letter case, and treating a decomposed "é" as the composed one.
	foldCase bool
}

func newFakeDisk(entries map[string]diskEntry) *fakeDisk {
	return &fakeDisk{entries: entries, lstats: map[string]int{}, listings: map[string]int{}}
}

// lookup finds the stored spelling of name. A folding disk finds it whatever its
// case, as a case-insensitive filesystem does.
func (d *fakeDisk) lookup(name string) (string, bool) {
	for stored := range d.entries {
		if d.fold(stored) == d.fold(name) {
			return stored, true
		}
	}
	return "", false
}

func (d *fakeDisk) fold(name string) string {
	if d.foldCase {
		return strings.ReplaceAll(strings.ToLower(name), "e\u0301", "\u00e9")
	}
	return name
}

func (d *fakeDisk) ReadDirNames(dir string) ([]string, error) {
	d.listings[dir]++
	seen := map[string]bool{}
	for stored := range d.entries {
		if child, ok := d.childOf(dir, stored); ok {
			seen[child] = true
		}
	}
	return slices.Sorted(maps.Keys(seen)), nil
}

// childOf names the entry of dir that holds stored: the file itself, or the
// first directory on the way to it, spelled as stored spells it.
func (d *fakeDisk) childOf(dir, stored string) (string, bool) {
	prefix := dir + "/"
	if dir == "." {
		prefix = ""
	}
	if len(stored) <= len(prefix) || d.fold(stored[:len(prefix)]) != d.fold(prefix) {
		return "", false
	}
	child, _, _ := strings.Cut(stored[len(prefix):], "/")
	return child, true
}

func (d *fakeDisk) Lstat(name string) (fs.FileInfo, error) {
	stored, ok := d.lookup(name)
	entry := d.entries[stored]
	if !ok {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
	}
	if entry.lstatErr != nil {
		return nil, entry.lstatErr
	}
	d.lstats[name]++
	stat := d.lstats[name]
	return fakeInfo{name: name, size: entry.sizeAtStat(stat), mode: entry.mode, modTime: entry.modTimeAtStat(stat)}, nil
}

func (d *fakeDisk) ReadFile(name string, _ fs.FileInfo) ([]byte, error) {
	stored, _ := d.lookup(name)
	entry := d.entries[stored]
	if entry.replacedBeforeOpen {
		return nil, fmt.Errorf("%s: %w", name, worktreeoverlay.ErrReplaced)
	}
	if entry.readErr != nil {
		return nil, entry.readErr
	}
	return entry.data, nil
}
