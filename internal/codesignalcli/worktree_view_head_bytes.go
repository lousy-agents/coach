package codesignalcli

import (
	"errors"
	"io/fs"
)

// errAbsentFromWorktree marks a path the working tree removed or the overlay
// could not read. File-local analysis skips it: committed bytes would describe
// a file the user has already changed.
var errAbsentFromWorktree = errors.New("path is absent from the working tree scan")

// HeadBytes returns the working-tree bytes for a path the overlay read. covered
// is false for any other path, which the caller reads from the committed
// revision; errAbsentFromWorktree means the path must be skipped. A nil view
// covers nothing.
func (v *WorktreeView) HeadBytes(dir, revision, name string) (data []byte, covered bool, err error) {
	if v == nil {
		return nil, false, nil
	}
	fsys, err := v.OpenHead(dir, revision)
	if err != nil {
		return nil, true, err
	}
	switch {
	case v.overlaid[name]:
		data, err = fs.ReadFile(fsys, name)
		if err == nil {
			v.fileLocalRead[name] = true
		}
		return data, true, err
	case v.absent[name]:
		return nil, true, errAbsentFromWorktree
	}
	return nil, false, nil
}

// headFileBytes reads a head-revision file for file-local analysis: from the
// working tree when the view read it, otherwise through committed.
func headFileBytes(view *WorktreeView, dir, revision, name string, committed func(string) ([]byte, error)) ([]byte, error) {
	if data, covered, err := view.HeadBytes(dir, revision, name); covered {
		return data, err
	}
	return committed(name)
}
