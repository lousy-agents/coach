package worktreeoverlay

import (
	"bytes"
	"io"
	"io/fs"
	"time"
)

type memInfo struct {
	name string
	size int64
	dir  bool
}

func (i memInfo) Name() string { return i.name }
func (i memInfo) Size() int64  { return i.size }
func (i memInfo) IsDir() bool  { return i.dir }
func (i memInfo) Sys() any     { return nil }

func (i memInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func (i memInfo) ModTime() time.Time { return time.Time{} }

type memFile struct {
	*bytes.Reader
	info memInfo
}

func newMemFile(name string, data []byte) *memFile {
	return &memFile{Reader: bytes.NewReader(data), info: memInfo{name: name, size: int64(len(data))}}
}

func (f *memFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *memFile) Close() error               { return nil }

type dirFile struct {
	info    memInfo
	entries []fs.DirEntry
	offset  int
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *dirFile) Close() error               { return nil }

func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: fs.ErrInvalid}
}

func (d *dirFile) ReadDir(count int) ([]fs.DirEntry, error) {
	remaining := d.entries[d.offset:]
	if count <= 0 {
		d.offset = len(d.entries)
		return remaining, nil
	}
	if len(remaining) == 0 {
		return nil, io.EOF
	}
	if count > len(remaining) {
		count = len(remaining)
	}
	d.offset += count
	return remaining[:count], nil
}
