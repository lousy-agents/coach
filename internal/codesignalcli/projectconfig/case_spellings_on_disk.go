package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
)

// throughARepository is whether a spelling runs through a directory, below the root, that holds a .git entry: the
// checkout of a submodule or another repository. A policy there cannot be committed here, which the submodule refusal
// says; a difference in case below it is not the reason.
func throughARepository(root, spelling string) bool {
	parts := strings.Split(spelling, "/")
	for depth := 1; depth < len(parts); depth++ {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(strings.Join(parts[:depth], "/")), ".git")); err == nil {
			return true
		}
	}
	return false
}

// diskEntries lists a directory of the working tree, through the repository root so that a link cannot lead the walk
// outside it. A directory that cannot be listed has no spellings to offer: the checks of the path itself say why it
// cannot be read.
func diskEntries(root string) directoryEntries {
	return func(directory string) ([]string, error) {
		opened, err := os.OpenRoot(root)
		if err != nil {
			return nil, nil
		}
		defer func() { _ = opened.Close() }()
		name := "."
		if directory != "" {
			name = filepath.FromSlash(directory)
		}
		listing, err := opened.OpenFile(name, os.O_RDONLY|openWithoutWaiting, 0)
		if err != nil {
			return nil, nil
		}
		if info, err := listing.Stat(); err != nil || !info.IsDir() {
			_ = listing.Close()
			return nil, nil
		}
		defer func() { _ = listing.Close() }()
		names, err := listing.Readdirnames(-1)
		if err != nil {
			return nil, nil
		}
		return names, nil
	}
}

// regularFileUnder is whether a spelling names a regular file under root. Anything else there, a directory, a link or
// a pipe, is no policy under another name.
func regularFileUnder(root, spelling string) bool {
	opened, err := os.OpenRoot(root)
	if err != nil {
		return false
	}
	defer func() { _ = opened.Close() }()
	info, err := opened.Lstat(filepath.FromSlash(spelling))
	return err == nil && info.Mode().IsRegular()
}
