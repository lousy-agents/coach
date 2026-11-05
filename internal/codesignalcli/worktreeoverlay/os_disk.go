package worktreeoverlay

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// OSDisk reads the working tree rooted at Root. It never follows a symbolic
// link out of the repository: a path under a symlinked directory reads as a
// symlink, and the read itself is confined to Root.
type OSDisk struct{ Root string }

// ReadFile reads a regular file under Root. The open is confined to Root, never
// blocks on a FIFO or device that replaced the file after the caller's Lstat,
// refuses a file that is not the one examined describes, and reads at most
// MaxFileBytes. The identity check is on the opened handle, because the path
// can be swapped for another file inside the root and swapped back again before
// any later stat of the path.
func (d OSDisk) ReadFile(name string, examined fs.FileInfo) ([]byte, error) {
	root, err := os.OpenRoot(d.Root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	file, err := root.OpenFile(filepath.FromSlash(name), os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file: %w", name, ErrNotRegular)
	}
	if !os.SameFile(examined, info) {
		return nil, fmt.Errorf("%s is not the file that was examined: %w", name, ErrReplaced)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("%s is over the %d-byte limit: %w", name, MaxFileBytes, ErrTooLarge)
	}
	return data, nil
}

// ReadDirNames lists a directory through the Root handle, like every other read,
// so a name that escapes the root, or a link that leads out of it, is an error
// and never a listing of an outside directory.
func (d OSDisk) ReadDirNames(dir string) ([]string, error) {
	root, err := os.OpenRoot(d.Root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	handle, err := root.Open(filepath.FromSlash(dir))
	if err != nil {
		return nil, err
	}
	defer func() { _ = handle.Close() }()
	entries, err := handle.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}
