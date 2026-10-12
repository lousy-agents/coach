package revisionfs

import (
	"fmt"
	"io/fs"
)

func (f *revisionFS) ReadDir(name string) ([]fs.DirEntry, error) {
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

func (f *revisionFS) ReadFile(name string) ([]byte, error) {
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

func (f *revisionFS) readFile(clean string) ([]byte, error) {
	data, err := runSnapshotGit(f.dir, maxSnapshotFileBytes, maxSnapshotGitStderr, snapshotGitTimeout, "show", f.revision+":"+clean)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: clean, Err: fmt.Errorf("git show %s:%s: %w", f.revision, clean, err)}
	}
	return data, nil
}
