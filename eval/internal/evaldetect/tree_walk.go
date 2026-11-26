package evaldetect

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// skippedDirs are trees no detector reads: dependencies, fixtures and VCS data.
var skippedDirs = map[string]bool{"vendor": true, "testdata": true, "node_modules": true}

// walkFiles calls visit with the slash-separated, root-relative path of every
// file under root whose name satisfies keep.
func walkFiles(root string, keep func(name string) bool, visit func(rel string) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return skipDir(path, root, entry.Name())
		}
		if !keep(entry.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return visit(filepath.ToSlash(rel))
	})
}

func skipDir(path, root, name string) error {
	if path != root && (skippedDirs[name] || strings.HasPrefix(name, ".")) {
		return filepath.SkipDir
	}
	return nil
}
