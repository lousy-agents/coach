package projectconfig

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// openDirectory opens directory without waiting for a writer and requires the
// opened handle to be a directory. The stat that found a directory can be out
// of date: a FIFO swapped in since would make a plain open wait for a writer
// that never comes, with nothing to end the wait. Like checkReadable, it lets
// the handle, not the earlier stat, decide what the entry is.
func openDirectory(root diskRoot, directory string) (diskFile, error) {
	listing, err := root.OpenFile(filepath.FromSlash(directory), os.O_RDONLY|openWithoutWaiting, 0)
	if err != nil {
		return nil, err
	}
	info, err := listing.Stat()
	if err != nil {
		_ = listing.Close()
		return nil, err
	}
	if !info.IsDir() {
		_ = listing.Close()
		return nil, &fs.PathError{Op: "open", Path: directory, Err: syscall.ENOTDIR}
	}
	return listing, nil
}
