package projectconfig

import (
	"io/fs"
	"os"
)

// diskRoot is what the on-disk checks use of the repository root: a stat that does not follow links,
// and an open. It is *os.Root in production. A test gives it a file system that folds case, because the
// checks for a name spelled in another case can only fire on one, and a Linux disk is not.
type diskRoot interface {
	Lstat(name string) (fs.FileInfo, error)
	OpenFile(name string, flag int, perm fs.FileMode) (diskFile, error)
}

// diskFile is the part of an opened entry the checks use.
type diskFile interface {
	Stat() (fs.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	Close() error
}

// osDiskRoot is a diskRoot over an *os.Root.
type osDiskRoot struct{ root *os.Root }

func (r osDiskRoot) Lstat(name string) (fs.FileInfo, error) {
	return r.root.Lstat(name)
}

func (r osDiskRoot) OpenFile(name string, flag int, perm fs.FileMode) (diskFile, error) {
	file, err := r.root.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return file, nil
}
