package evalpatterns

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// productionGoFiles lists, sorted, the slash-separated paths of the .go
// files under root that a build would compile: no _test.go file, and
// nothing under testdata, vendor or a directory whose name starts with . or
// _, which the go tool ignores too.
func productionGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return skipIgnored(root, path, entry.Name())
		}
		if isProductionGo(entry.Name()) {
			rel, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func ignoredDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func skipIgnored(root, path, name string) error {
	if path != root && ignoredDir(name) {
		return filepath.SkipDir
	}
	return nil
}

func isProductionGo(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}
