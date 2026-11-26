package evalpatterns

import (
	"go/ast"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
)

// modulePath is the module path in root's go.mod, or "" without one.
func modulePath(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
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
