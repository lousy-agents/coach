package worktreeoverlay

import (
	"errors"
	"io/fs"
	"path"
	"syscall"
)

// overlayFS implements fs.FS, fs.ReadDirFS, fs.ReadFileFS and fs.StatFS over a
// base snapshot. files holds disk bytes for modified and added paths, hidden
// holds paths that must read as absent, and added holds the directory entries
// the overlay contributes or replaces, by parent directory.
type overlayFS struct {
	base   fs.FS
	files  map[string][]byte
	hidden map[string]bool
	dirs   map[string]bool
	added  map[string]map[string]fs.DirEntry
}

type layer int

const (
	layerBase layer = iota
	layerFile
	layerDir
)

func validName(name string) bool {
	return name == "." || fs.ValidPath(name)
}

func notExist(op, name string) error {
	return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
}

// layerOf decides once, for every operation, whether name is invalid or hidden
// and otherwise which layer holds it. An overlaid file wins over an overlay
// directory of the same name, and either wins over a hidden mark: a path is
// hidden because the entry committed under that name is gone, and a directory
// that now stands there holds overlaid files, so it is not absent.
func (o *overlayFS) layerOf(op, name string) (layer, error) {
	_, overlaid := o.files[name]
	switch {
	case !validName(name):
		return layerBase, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	case overlaid:
		return layerFile, nil
	case o.dirs[name]:
		return layerDir, nil
	case o.hidden[name]:
		return layerBase, notExist(op, name)
	}
	return layerBase, nil
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	which, err := o.layerOf("open", name)
	if err != nil {
		return nil, err
	}
	if which == layerFile {
		return newMemFile(path.Base(name), o.files[name]), nil
	}
	if which == layerBase && !baseHasDir(o.base, name) {
		return o.base.Open(name)
	}
	entries, err := o.ReadDir(name)
	if err != nil {
		return nil, err
	}
	return &dirFile{info: memInfo{name: path.Base(name), dir: true}, entries: entries}, nil
}

func (o *overlayFS) Stat(name string) (fs.FileInfo, error) {
	which, err := o.layerOf("stat", name)
	switch {
	case err != nil:
		return nil, err
	case which == layerFile:
		return memInfo{name: path.Base(name), size: int64(len(o.files[name]))}, nil
	case which == layerDir:
		return memInfo{name: path.Base(name), dir: true}, nil
	}
	return fs.Stat(o.base, name)
}

func (o *overlayFS) ReadFile(name string) ([]byte, error) {
	which, err := o.layerOf("readfile", name)
	switch {
	case err != nil:
		return nil, err
	case which == layerFile:
		return append([]byte(nil), o.files[name]...), nil
	case which == layerDir:
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: syscall.EISDIR}
	}
	return fs.ReadFile(o.base, name)
}

func (o *overlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	which, err := o.layerOf("readdir", name)
	switch {
	case err != nil:
		return nil, err
	case which == layerFile:
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: syscall.ENOTDIR}
	}
	entries, err := o.baseListing(name, which)
	if err != nil {
		return nil, err
	}
	return o.mergeEntries(name, entries), nil
}

// baseListing is what the committed snapshot lists under name. A directory the
// overlay contributes lists nothing from the base when the base has no
// directory there, either because nothing is committed at that name or because
// a file is: the committed file is what the new directory replaced.
func (o *overlayFS) baseListing(name string, which layer) ([]fs.DirEntry, error) {
	if which == layerDir && baseHoldsNoDir(o.base, name) {
		return nil, nil
	}
	return fs.ReadDir(o.base, name)
}

func baseHoldsNoDir(base fs.FS, name string) bool {
	info, err := fs.Stat(base, name)
	return errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir())
}

func baseHasDir(base fs.FS, name string) bool {
	info, err := fs.Stat(base, name)
	return err == nil && info.IsDir()
}
