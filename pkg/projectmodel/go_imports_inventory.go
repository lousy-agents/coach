package projectmodel

import (
	"io/fs"
	"path"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

func collectGoSourceInventory(snapshot fs.FS, modules map[string]*modfile.File, moduleDirs []string) (moduleFileList map[string][]string, allFiles []string, packageFiles map[string][]string, fileModule map[string]string) {
	moduleDirSet := make(map[string]bool, len(modules))
	for dir := range modules {
		moduleDirSet[dir] = true
	}

	moduleFileList = make(map[string][]string, len(moduleDirs))
	for _, mdir := range moduleDirs {
		mfiles := moduleGoFiles(snapshot, mdir, moduleDirSet)
		moduleFileList[mdir] = mfiles
		allFiles = append(allFiles, mfiles...)
	}
	sort.Strings(allFiles)

	packageFiles = map[string][]string{}
	fileModule = map[string]string{}
	for mdir, mfiles := range moduleFileList {
		for _, f := range mfiles {
			pkgDir := path.Dir(f)
			packageFiles[pkgDir] = append(packageFiles[pkgDir], f)
			fileModule[f] = mdir
		}
	}
	for pkgDir := range packageFiles {
		sort.Strings(packageFiles[pkgDir])
	}
	return moduleFileList, allFiles, packageFiles, fileModule
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
		if entry.IsDir() && shouldSkipModuleWalkDir(p, moduleDir, moduleDirs) {
			return fs.SkipDir
		}
		if entry.IsDir() {
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

// shouldSkipModuleWalkDir reports whether the moduleGoFiles walk should
// prune p: nested module roots plus the same testdata/vendor/dot dirs
// discovery skips. The module root itself is never pruned.
func shouldSkipModuleWalkDir(p, moduleDir string, moduleDirs map[string]bool) bool {
	if p == moduleDir {
		return false
	}
	return moduleDirs[p] || shouldSkipDiscoveryDir(p)
}
