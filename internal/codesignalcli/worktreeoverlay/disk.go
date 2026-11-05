package worktreeoverlay

import (
	"errors"
	"io/fs"
)

// MaxFileBytes bounds one overlaid file, matching the committed snapshot's
// per-file bound so an overlay cannot admit what the snapshot would refuse.
const MaxFileBytes = 32 << 20

// Reasons OSDisk.ReadFile refuses a file, for errors.Is.
var (
	ErrNotRegular = errors.New("not a regular file")
	ErrTooLarge   = errors.New("over the size limit")
	ErrReplaced   = errors.New("replaced after it was examined")
	// ErrNameMismatch means the path exists, but only under another spelling.
	ErrNameMismatch = errors.New("spelled differently on disk")
)

// DiskSource is the working-tree read port. Names are slash-separated and
// relative to the repository root.
type DiskSource interface {
	Lstat(name string) (fs.FileInfo, error)
	// ReadFile reads name, which must still be the file that examined, the
	// result of an earlier Lstat of name, describes. A path swapped for another
	// file in between fails with ErrReplaced.
	ReadFile(name string, examined fs.FileInfo) ([]byte, error)
	// ReadDirNames lists the entry names of the directory dir, spelled as the
	// filesystem stores them. "." is the repository root.
	ReadDirNames(dir string) ([]string, error)
}

type diskKind int

const (
	diskRegular diskKind = iota
	diskMissing
	diskSymlink
	diskNonRegular
	diskTooLarge
	diskPermissionDenied
	diskReadFailed
	diskChanged
	diskNameMismatch
)

// readDisk classifies name on disk and returns its bytes when it is a regular
// file that stayed the same size and mode across the read.
func readDisk(disk DiskSource, name string) (diskKind, []byte) {
	before, err := disk.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return diskMissing, nil
	}
	if err != nil {
		return failureKind(err), nil
	}
	if kind, final := classifyBeforeRead(before); final {
		return kind, nil
	}
	data, err := disk.ReadFile(name, before)
	if err != nil {
		return failureKind(err), nil
	}
	after, err := disk.Lstat(name)
	if err != nil {
		return failureKind(err), nil
	}
	if !unchanged(before, after, len(data)) {
		return diskChanged, nil
	}
	return diskRegular, data
}

// failureKind names why a stat or a read failed. The read re-checks what the
// stat saw, so a file that became a special file or outgrew the bound in
// between fails with the same reasons the stat would have given.
func failureKind(err error) diskKind {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return diskPermissionDenied
	case errors.Is(err, ErrNotRegular):
		return diskNonRegular
	case errors.Is(err, ErrTooLarge):
		return diskTooLarge
	case errors.Is(err, ErrReplaced):
		return diskChanged
	case errors.Is(err, ErrNameMismatch):
		return diskNameMismatch
	}
	return diskReadFailed
}

// classifyBeforeRead decides from Lstat alone, so a FIFO or a device is never
// opened: opening one can block forever.
func classifyBeforeRead(info fs.FileInfo) (diskKind, bool) {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return diskSymlink, true
	case info.IsDir():
		return diskMissing, true
	case !info.Mode().IsRegular():
		return diskNonRegular, true
	case info.Size() > MaxFileBytes:
		return diskTooLarge, true
	}
	return diskRegular, false
}

func unchanged(before, after fs.FileInfo, readLen int) bool {
	return after.Mode() == before.Mode() &&
		after.Size() == before.Size() &&
		after.ModTime().Equal(before.ModTime()) &&
		int64(readLen) == after.Size()
}
