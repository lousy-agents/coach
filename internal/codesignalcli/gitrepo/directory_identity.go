package gitrepo

import (
	"os"
	"path/filepath"
)

// sameDirectory compares by file identity, so that a spelling of the path that differs in letter case or in links
// is still the same directory.
func sameDirectory(a, b string) (bool, error) {
	first, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	second, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(first, second), nil
}

// directoryIsInside is whether dir is ancestor or below it, by file identity at each step up from dir.
func directoryIsInside(dir, ancestor string) (bool, error) {
	current, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false, err
	}
	for {
		same, err := sameDirectory(current, ancestor)
		if err != nil {
			return false, err
		}
		if same {
			return true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
	}
}
