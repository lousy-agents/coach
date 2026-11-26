package evalpatterns

import (
	"errors"
	"go/ast"
	"io/fs"
	pathpkg "path"
	"sort"
	"strings"
)

// modulePath is the module path in root's go.mod, or "" without one. A
// go.mod that is not a regular file is an error.
func modulePath(root string) (string, error) {
	raw, err := readRegularFile(root, "go.mod")
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		code, _, _ := strings.Cut(line, "//")
		if fields := strings.Fields(code); len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	return "", nil
}

func sortedDirs(packages map[string][]*ast.File) []string {
	dirs := make([]string, 0, len(packages))
	for dir := range packages {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func packageDir(rel string) string { return pathpkg.Dir(rel) }
