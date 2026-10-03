package projectmodel

import (
	"io/fs"
	"path"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// selectedRootsFrom cleans and sorts roots for Snapshot.SelectedRoots, so
// callers passing the same roots in a different order still produce
// byte-identical canonical Model JSON. A nil/empty roots yields a nil
// slice, matching Snapshot.SelectedRoots' omitempty contract.
func selectedRootsFrom(roots []string) []string {
	if len(roots) == 0 {
		return nil
	}
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = path.Clean(r)
	}
	sort.Strings(out)
	return out
}
func countDistinctRoots(modules map[string]*modfile.File, workspaces map[string]*modfile.WorkFile) int {
	seen := make(map[string]bool, len(modules)+len(workspaces))
	for dir := range modules {
		seen[dir] = true
	}
	for dir := range workspaces {
		seen[dir] = true
	}
	return len(seen)
}
func analyzeGoSources(
	snapshot fs.FS,
	analyzer *semantics.Analyzer,
	allFiles []string,
	modules map[string]*modfile.File,
	packageFiles map[string][]string,
	fileModule map[string]string,
	budgets GoBudgets,
) (files []File, edges []ImportEdge, unresolvedEdges, excludedEdges int, truncated bool, diagnostics []Diagnostic) {
	analysis := goSourceAnalysis{files: make([]File, 0, len(allFiles))}
	for _, f := range allFiles {
		if analysis.consume(snapshot, analyzer, f, modules, packageFiles, fileModule, budgets) {
			break
		}
	}
	return analysis.files, analysis.edges, analysis.unresolvedEdges, analysis.excludedEdges, analysis.truncated, analysis.diagnostics
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

// isStdlibImport reports whether importPath looks like a standard-library
// import: its first path segment has no dot. This is the same heuristic
// goimports and similar tools use to separate stdlib from third-party
// imports. It cannot distinguish a stdlib name from a dotless module path
// (`go mod init myapp`), so callers must match workspace module paths first
// — see classifyGoImport.
func isStdlibImport(importPath string) bool {
	first := importPath
	if idx := strings.Index(importPath, "/"); idx >= 0 {
		first = importPath[:idx]
	}
	return !strings.Contains(first, ".")
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
func filePathSet(files []File) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, f := range files {
		out[f.Path] = true
	}
	return out
}
