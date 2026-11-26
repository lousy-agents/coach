package evalharness

import (
	"io/fs"
	"os"
	"path/filepath"
)

func copyProgram(from, to string) error {
	if err := os.RemoveAll(to); err != nil {
		return err
	}
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, content, info.Mode().Perm())
	})
}
