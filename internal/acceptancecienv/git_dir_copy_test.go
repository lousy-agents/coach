package acceptancecienv

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// copyGitDir copies the .git directory at source to target. It lists every
// entry first and then copies each, calling afterList in between, so a spec
// can remove a file git was holding, as a concurrent git process would.
func copyGitDir(source, target string, afterList func()) error {
	var entries []string
	err := filepath.WalkDir(source, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries = append(entries, path)
		return nil
	})
	if err != nil {
		return err
	}
	if afterList != nil {
		afterList()
	}
	for _, path := range entries {
		if err := copyGitEntry(source, target, path); err != nil {
			return err
		}
	}
	return nil
}

func copyGitEntry(source, target, path string) error {
	rel, err := filepath.Rel(source, path)
	if err != nil {
		return err
	}
	destination := filepath.Join(target, rel)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	switch {
	case info.IsDir():
		return os.MkdirAll(destination, info.Mode().Perm())
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		return os.Symlink(link, destination)
	}
	return copyGitFile(path, destination, info.Mode().Perm())
}

func copyGitFile(path, destination string, mode os.FileMode) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
