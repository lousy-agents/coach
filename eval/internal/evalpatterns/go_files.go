package evalpatterns

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// productionGoFiles lists, sorted, the slash-separated paths of every .go
// file under root except _test.go files and anything under testdata, vendor
// or a directory whose name starts with . or _. Build constraints are not
// read, so a //go:build ignore file or an a_windows.go file is listed too.
// A .go entry that is not a regular file, such as a symlink, is an error
// rather than a file to read: the checkout is untrusted, and a link could
// reach a file on the host.
func productionGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return skipIgnored(root, path, entry.Name())
		}
		if !isProductionGo(entry.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if !entry.Type().IsRegular() {
			return fmt.Errorf("file %s is not a regular file", filepath.ToSlash(rel))
		}
		files = append(files, filepath.ToSlash(rel))
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
