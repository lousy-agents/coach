package projectmodel

import (
	"path"
	"sort"

	"golang.org/x/mod/modfile"
)

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

func buildModuleFacts(moduleDirs []string, moduleFileList map[string][]string, analyzed map[string]bool) []Module {
	moduleList := make([]Module, 0, len(moduleDirs))
	for _, mdir := range moduleDirs {
		moduleList = append(moduleList, Module{
			ID:       "module:" + mdir,
			Path:     mdir,
			Language: "go",
			Files:    filterAnalyzedPaths(moduleFileList[mdir], analyzed),
		})
	}
	return moduleList
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

func filterAnalyzedPaths(paths []string, analyzed map[string]bool) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if analyzed[p] {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filePathSet(files []File) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, f := range files {
		out[f.Path] = true
	}
	return out
}
