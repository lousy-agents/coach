package worktreeoverlay_test

import (
	"io/fs"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// diskEntry describes one path on the fake disk and the faults it injects.
type diskEntry struct {
	mode            fs.FileMode
	data            []byte
	lstatErr        error
	readErr         error
	growsAfterRead  bool
	reportedSizeBig bool
	// touchedAfterRead keeps size and mode but moves the modification time
	// between the two stats around the read, as a same-length rewrite does.
	touchedAfterRead bool
	// replacedBeforeOpen makes the read find another file at the path from the
	// one the stat described.
	replacedBeforeOpen bool
}

func regular(data string) diskEntry { return diskEntry{mode: 0o644, data: []byte(data)} }

// sizeAtStat is the size the fake disk reports on the stat-th Lstat of this
// entry: a file that grows after the read reports one more byte from the second
// stat on, and an over-the-bound file reports the bound plus one.
func (e diskEntry) sizeAtStat(stat int) int64 {
	size := int64(len(e.data))
	if e.growsAfterRead && stat > 1 {
		size++
	}
	if e.reportedSizeBig {
		size = worktreeoverlay.MaxFileBytes + 1
	}
	return size
}

// modTimeAtStat is the modification time reported on the stat-th Lstat of this
// entry: a same-length rewrite moves it from the second stat on.
func (e diskEntry) modTimeAtStat(stat int) time.Time {
	if e.touchedAfterRead && stat > 1 {
		return time.Unix(2, 0)
	}
	return time.Unix(1, 0)
}

type fakeInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return i.size }
func (i fakeInfo) Mode() fs.FileMode  { return i.mode }
func (i fakeInfo) ModTime() time.Time { return i.modTime }
func (i fakeInfo) IsDir() bool        { return i.mode.IsDir() }
func (i fakeInfo) Sys() any           { return nil }
