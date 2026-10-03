package projectmodel

import (
	"io/fs"
	"path"
	"sort"

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
func buildWorkspaceFacts(workspaces map[string]*modfile.WorkFile, modules map[string]*modfile.File) []Workspace {
	workspaceList := make([]Workspace, 0, len(workspaces))
	for _, wdir := range mapKeysSorted(workspaces) {
		wf := workspaces[wdir]
		projects := workspaceProjects(wdir, wf.Use, modules)
		workspaceList = append(workspaceList, Workspace{
			ID:       "workspace:" + wdir,
			Language: "go",
			Root:     wdir,
			Projects: projects,
		})
	}
	return workspaceList
}

func workspaceProjects(wdir string, uses []*modfile.Use, modules map[string]*modfile.File) []string {
	var projects []string
	seen := map[string]bool{}
	for _, use := range uses {
		resolved := path.Clean(path.Join(wdir, use.Path))
		if _, ok := modules[resolved]; !ok || seen[resolved] {
			continue
		}
		seen[resolved] = true
		projects = append(projects, "module:"+resolved)
	}
	sort.Strings(projects)
	return projects
}
func buildPackageFacts(packageFiles map[string][]string, analyzed map[string]bool) []Package {
	packageList := make([]Package, 0, len(packageFiles))
	for _, pdir := range mapKeysSorted(packageFiles) {
		kept := filterAnalyzedPaths(packageFiles[pdir], analyzed)
		if len(kept) == 0 {
			continue
		}
		packageList = append(packageList, Package{
			ID:       "package:" + pdir,
			Path:     pdir,
			Language: "go",
			Files:    kept,
		})
	}
	return packageList
}
