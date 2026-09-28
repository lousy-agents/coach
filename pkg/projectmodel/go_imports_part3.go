package projectmodel

import (
	"io/fs"

	"golang.org/x/mod/modfile"

	"sort"
	"strings"
)

// classifyGoImportViaOwner checks importPath against owner's own
// replace/exclude/require directives, in that precedence order.
func classifyGoImportViaOwner(importPath string, owner *modfile.File) (kind, to string, ok bool) {
	for _, r := range owner.Replace {
		if matchesModulePrefix(importPath, r.Old.Path) {
			return "replaced", importPath, true
		}
	}
	for _, e := range owner.Exclude {
		if matchesModulePrefix(importPath, e.Mod.Path) {
			return "excluded", importPath, true
		}
	}
	for _, req := range owner.Require {
		if matchesModulePrefix(importPath, req.Mod.Path) {
			return "external", importPath, true
		}
	}
	return "", "", false
}

// moduleGoFiles returns every .go file under moduleDir, repository-relative
// and sorted, excluding any subtree that is itself a different module's
// root (moduleDirs), plus any testdata/, vendor/, or dot-prefixed
// subdirectory -- the same directories the go tool itself never walks into.
func moduleGoFiles(snapshot fs.FS, moduleDir string, moduleDirs map[string]bool) []string {
	var files []string
	_ = fs.WalkDir(snapshot, moduleDir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if shouldSkipModuleWalkDir(p, moduleDir, moduleDirs) {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files
}
