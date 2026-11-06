package projectconfig

import (
	"io/fs"
	"path"
	"strings"
	"testing/fstest"
)

// foldingDisk is a disk that resolves a name spelled in another case to the entry that exists, as a
// macOS or Windows file system does, so the checks for a name spelled in another case run on a machine
// whose own disk tells cases apart.
type foldingDisk struct{ files fstest.MapFS }

func foldingDiskHolding(paths ...string) foldingDisk {
	files := fstest.MapFS{}
	for _, p := range paths {
		files[p] = &fstest.MapFile{Data: []byte("{}")}
	}
	return foldingDisk{files}
}

// stored is the name as the disk keeps it, or false when no entry has it in any case.
func (d foldingDisk) stored(name string) (string, bool) {
	resolved := "."
	for _, part := range strings.Split(path.Clean(name), "/") {
		if part == "." {
			continue
		}
		match, found := d.entryFolding(resolved, part)
		if !found {
			return "", false
		}
		resolved = path.Join(resolved, match)
	}
	return resolved, true
}

func (d foldingDisk) entryFolding(directory, part string) (string, bool) {
	entries, err := fs.ReadDir(d.files, directory)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), part) {
			return entry.Name(), true
		}
	}
	return "", false
}

func (d foldingDisk) Lstat(name string) (fs.FileInfo, error) {
	stored, found := d.stored(name)
	if !found {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
	}
	return fs.Lstat(d.files, stored)
}

func (d foldingDisk) OpenFile(name string, _ int, _ fs.FileMode) (diskFile, error) {
	stored, found := d.stored(name)
	if !found {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	file, err := d.files.Open(stored)
	if err != nil {
		return nil, err
	}
	return foldedFile{file}, nil
}
