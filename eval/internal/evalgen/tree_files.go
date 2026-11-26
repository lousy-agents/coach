package evalgen

import (
	"os"
	"path/filepath"
	"strings"
)

// WriteTree writes a tree under dir; scripts are executable.
func WriteTree(dir string, tree map[string][]byte) error {
	for path, content := range tree {
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(target, content, mode); err != nil {
			return err
		}
	}
	return nil
}
