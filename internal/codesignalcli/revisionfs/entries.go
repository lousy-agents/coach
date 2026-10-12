package revisionfs

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"time"
)

type snapshotDirEntry struct {
	name  string
	isDir bool
}

func (e snapshotDirEntry) Name() string { return e.name }

func (e snapshotDirEntry) IsDir() bool { return e.isDir }

func (e snapshotDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}

type snapshotFile struct {
	name string
	data []byte
	pos  int
}

func (sf *snapshotFile) Stat() (fs.FileInfo, error) {
	return snapshotFileInfo{name: path.Base(sf.name), size: int64(len(sf.data))}, nil
}

func (sf *snapshotFile) Close() error { return nil }

type snapshotDirFile struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

func (sd *snapshotDirFile) Stat() (fs.FileInfo, error) {
	return snapshotDirInfo{name: path.Base(sd.name)}, nil
}

func (sd *snapshotDirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: sd.name, Err: fmt.Errorf("is a directory")}
}

func (sd *snapshotDirFile) Close() error { return nil }

type snapshotFileInfo struct {
	name string
	size int64
}

func (i snapshotFileInfo) Name() string { return i.name }

func (i snapshotFileInfo) Size() int64 { return i.size }

func (i snapshotFileInfo) Mode() fs.FileMode { return 0o444 }

func (i snapshotFileInfo) ModTime() time.Time { return time.Time{} }

func (i snapshotFileInfo) IsDir() bool { return false }

func (i snapshotFileInfo) Sys() any { return nil }

type snapshotDirInfo struct{ name string }

func (i snapshotDirInfo) Name() string { return i.name }

func (i snapshotDirInfo) Size() int64 { return 0 }

func (i snapshotDirInfo) Mode() fs.FileMode { return fs.ModeDir | 0o555 }

func (i snapshotDirInfo) ModTime() time.Time { return time.Time{} }

func (i snapshotDirInfo) IsDir() bool { return true }

func (i snapshotDirInfo) Sys() any { return nil }

func (e snapshotDirEntry) Info() (fs.FileInfo, error) {
	if e.isDir {
		return snapshotDirInfo{name: e.name}, nil
	}
	return snapshotFileInfo{name: e.name}, nil
}

func (sd *snapshotDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		remaining := sd.entries[sd.offset:]
		sd.offset = len(sd.entries)
		return remaining, nil
	}
	if sd.offset >= len(sd.entries) {
		return nil, io.EOF
	}
	end := sd.offset + n
	if end > len(sd.entries) {
		end = len(sd.entries)
	}
	batch := sd.entries[sd.offset:end]
	sd.offset = end
	return batch, nil
}

func (sf *snapshotFile) Read(b []byte) (int, error) {
	if sf.pos >= len(sf.data) {
		return 0, io.EOF
	}
	n := copy(b, sf.data[sf.pos:])
	sf.pos += n
	return n, nil
}
