package projectconfig

import (
	"errors"
	"io/fs"
)

// foldedFile adds the listing the checks use to an opened fs.File.
type foldedFile struct{ fs.File }

func (f foldedFile) Readdirnames(n int) ([]string, error) {
	directory, ok := f.File.(fs.ReadDirFile)
	if !ok {
		return nil, errors.New("not a directory")
	}
	entries, err := directory.ReadDir(n)
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names, err
}
