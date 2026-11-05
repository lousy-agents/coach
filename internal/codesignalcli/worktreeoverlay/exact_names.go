package worktreeoverlay

import (
	"io/fs"
	"path"
	"slices"
	"strings"
)

// exactNameDisk requires every component of a path, directories included, to
// appear spelled exactly that way in its parent's listing. A case-only rename of
// a directory leaves the old spelling of the whole path resolving on a
// case-insensitive filesystem, with the leaf name unchanged. Each directory is
// listed once and remembered, so the check costs one listing per directory
// however many changed files it holds.
type exactNameDisk struct {
	DiskSource
	listings map[string]map[string]bool
	twins    caseTwins
}

func (d *exactNameDisk) Lstat(name string) (fs.FileInfo, error) {
	info, err := d.DiskSource.Lstat(name)
	if err != nil || info.Mode()&fs.ModeSymlink != 0 {
		return info, err
	}
	present, err := d.hasExactSpelling(name)
	if err != nil {
		return nil, err
	}
	if present {
		return info, nil
	}
	if d.listedUnderAnotherCase(name) {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
	}
	return nil, &fs.PathError{Op: "lstat", Path: name, Err: ErrNameMismatch}
}

// listedUnderAnotherCase reports whether Git also lists name's other spelling
// and the disk holds the file under it. Then name is the old spelling of a
// rename and the other path carries the file. Without such a twin the file
// Git calls name is on disk under a spelling nothing else reads, and treating
// name as removed would print an all-clear over bytes that were never read.
func (d *exactNameDisk) listedUnderAnotherCase(name string) bool {
	return slices.ContainsFunc(d.twins.of(name), func(twin string) bool {
		present, err := d.hasExactSpelling(twin)
		return err == nil && present
	})
}

func (d *exactNameDisk) hasExactSpelling(name string) (bool, error) {
	for component := name; component != "."; component = path.Dir(component) {
		listed, err := d.isListed(component)
		if err != nil || !listed {
			return false, err
		}
	}
	return true, nil
}

func (d *exactNameDisk) isListed(name string) (bool, error) {
	dir := path.Dir(name)
	names, listed := d.listings[dir]
	if !listed {
		entries, err := d.DiskSource.ReadDirNames(dir)
		if err != nil {
			return false, err
		}
		names = make(map[string]bool, len(entries))
		for _, entry := range entries {
			names[entry] = true
		}
		d.listings[dir] = names
	}
	return names[path.Base(name)], nil
}

// caseTwins indexes the paths Git listed by their lower-cased spelling.
type caseTwins map[string][]string

func newCaseTwins(paths []string) caseTwins {
	twins := caseTwins{}
	for _, name := range paths {
		key := strings.ToLower(name)
		twins[key] = append(twins[key], name)
	}
	return twins
}

// of lists the other paths Git listed that differ from name only in letter case.
func (t caseTwins) of(name string) []string {
	return slices.DeleteFunc(slices.Clone(t[strings.ToLower(name)]), func(other string) bool { return other == name })
}
