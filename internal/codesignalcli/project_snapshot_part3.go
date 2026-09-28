package codesignalcli

import (
	"fmt"

	"io/fs"

	"path"
)

// Stat implements fs.StatFS entirely from the listing cached at
// construction, with no git child process of its own -- in particular, it
// never falls back to Open+git-show the way the io/fs package's own
// fs.Stat helper would if this method were absent, which would buffer an
// entire blob's content just to report its length.
func (f *goSnapshotFS) Stat(name string) (fs.FileInfo, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if f.isDir[clean] {
		return snapshotDirInfo{name: path.Base(clean)}, nil
	}
	if f.isFile[clean] {
		return snapshotFileInfo{name: path.Base(clean), size: f.sizes[clean]}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}
func (f *goSnapshotFS) ReadDir(name string) ([]fs.DirEntry, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	if !f.isDir[clean] {
		if f.isFile[clean] {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fmt.Errorf("not a directory")}
		}
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	entries := f.children[clean]
	out := make([]fs.DirEntry, len(entries))
	copy(out, entries)
	return out, nil
}
func (f *goSnapshotFS) ReadFile(name string) ([]byte, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	if f.isDir[clean] {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fmt.Errorf("is a directory")}
	}
	if !f.isFile[clean] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return f.readFile(clean)
}
func (e snapshotDirEntry) Info() (fs.FileInfo, error) {
	if e.isDir {
		return snapshotDirInfo{name: e.name}, nil
	}
	return snapshotFileInfo{name: e.name}, nil
}
