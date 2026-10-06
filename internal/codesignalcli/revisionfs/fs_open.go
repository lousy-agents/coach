package revisionfs

import (
	"fmt"
	"io/fs"
	"path"
)

// normalizeSnapshotName validates a caller-supplied fs.FS name per the
// io/fs contract (slash-separated, no ./.. elements); "." denotes the
// snapshot root.
func normalizeSnapshotName(name string) (string, error) {
	if name == "." {
		return ".", nil
	}
	if !fs.ValidPath(name) {
		return "", fmt.Errorf("invalid path %q", name)
	}
	return name, nil
}

func (f *revisionFS) Open(name string) (fs.File, error) {
	clean, err := normalizeSnapshotName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if f.isDir[clean] {
		return &snapshotDirFile{name: clean, entries: f.children[clean]}, nil
	}
	if f.isFile[clean] {
		data, err := f.readFile(clean)
		if err != nil {
			return nil, err
		}
		return &snapshotFile{name: clean, data: data}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// Stat implements fs.StatFS entirely from the listing cached at
// construction, with no git child process of its own -- in particular, it
// never falls back to Open+git-show the way the io/fs package's own
// fs.Stat helper would if this method were absent, which would buffer an
// entire blob's content just to report its length.
func (f *revisionFS) Stat(name string) (fs.FileInfo, error) {
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
