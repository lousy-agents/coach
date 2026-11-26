package evalgen

import (
	"io/fs"
	"os"
	"path/filepath"
)

func readTree(root string) (map[string][]byte, error) {
	tree := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(path)
		tree[filepath.ToSlash(rel)] = content
		return readErr
	})
	return tree, err
}
