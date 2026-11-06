package worktreeoverlay

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Lstat reports name without following a link in any component. A path that
// has a symlinked ancestor is reported as a symbolic link, because the file
// it would resolve to is not the repository's file. Every step goes through the
// Root handle, so a name that escapes the root is an error, not outside metadata.
func (d OSDisk) Lstat(name string) (fs.FileInfo, error) {
	root, err := os.OpenRoot(d.Root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	linked, err := hasSymlinkedAncestor(root, name)
	if err != nil {
		return nil, missingBeneathAFile(name, err)
	}
	if linked {
		return symlinkInfo{name: path.Base(name)}, nil
	}
	info, err := root.Lstat(filepath.FromSlash(name))
	return info, missingBeneathAFile(name, err)
}

// missingBeneathAFile reports a path under a regular file as not existing.
// A directory replaced by a file leaves nothing at the old path, and the
// operating system words that as ENOTDIR rather than ENOENT.
func missingBeneathAFile(name string, err error) error {
	if errors.Is(err, syscall.ENOTDIR) {
		return &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
	}
	return err
}

func hasSymlinkedAncestor(root *os.Root, name string) (bool, error) {
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		info, err := root.Lstat(filepath.FromSlash(strings.Join(parts[:i], "/")))
		if err != nil {
			return false, err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
}

// symlinkInfo stands in for a path whose ancestor is a symbolic link.
type symlinkInfo struct{ name string }

func (i symlinkInfo) Name() string       { return i.name }
func (i symlinkInfo) Size() int64        { return 0 }
func (i symlinkInfo) Mode() fs.FileMode  { return fs.ModeSymlink | 0o777 }
func (i symlinkInfo) ModTime() time.Time { return time.Time{} }
func (i symlinkInfo) IsDir() bool        { return false }
func (i symlinkInfo) Sys() any           { return nil }
