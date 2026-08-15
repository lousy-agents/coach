package projectmodel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// Stable file-level diagnostic codes for Model.Coverage.Diagnostics[i].Code,
// distinct from and never overlapping with the DiagRoot* vocabulary (which
// describes workspace/module root discovery, not individual files).
const (
	DiagFileSyntaxError    = "project_file_syntax_error"
	DiagFileUnavailable    = "project_file_unavailable"
	DiagFileBudgetExceeded = "project_file_budget_exceeded"
)

// SnapshotMeta carries revision/config/backend identities the caller
// already resolved (e.g. from Git) needed to populate Model.Snapshot.
// BuildGoModel does not compute these itself.
type SnapshotMeta struct {
	Revision           string
	TreeID             string
	ConfigDigest       string
	BackendDigest      string
	BuildContextDigest string
	Repository         string
}

// GoBuildOptions bounds one BuildGoModel call. Roots optionally scopes
// discovery to specific repository-relative workspace/module roots; when
// empty, BuildGoModel auto-discovers every go.work/go.mod root under
// snapshot.
type GoBuildOptions struct {
	Roots   []string
	Budgets GoBudgets
}

// BuildGoModel builds a Model's raw Go workspace/module/package/file/import
// facts by reading snapshot -- and only snapshot. A file with syntax errors
// keeps its File entry (so callers still see it exists) but contributes no
// import edges; a DiagFileSyntaxError diagnostic is recorded instead of
// failing the whole build. BuildGoModel never returns a non-nil error for
// per-file or per-root problems -- only for failures in the semantics
// analyzer itself that indicate a programming error (e.g. misconfigured
// AnalyzerOptions).
func BuildGoModel(snapshot fs.FS, meta SnapshotMeta, opts GoBuildOptions) (Model, error) {
	discovery := discoverGoProject(snapshot, opts.Budgets)
	useDiagnostics, _ := discovery.resolveUseDirectives()
	diagnostics := append(append([]Diagnostic{}, discovery.Diagnostics...), useDiagnostics...)

	modules, workspaces := discovery.Modules, discovery.Workspaces
	if len(opts.Roots) > 0 {
		modules, workspaces = filterToRoots(modules, workspaces, opts.Roots)
	}

	moduleDirs := mapKeysSorted(modules)
	moduleFileList, allFiles, packageFiles, fileModule := collectGoSourceInventory(snapshot, modules, moduleDirs)

	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{Languages: []semantics.Language{semantics.LanguageGo}})
	if err != nil {
		return Model{}, fmt.Errorf("projectmodel: constructing semantics analyzer: %w", err)
	}

	// The read/analyze phase is the expensive part of BuildGoModel, so it is
	// the phase opts.Budgets bounds: allFiles is already in deterministic
	// lexical order, so truncating at MaxInputFiles/MaxInputBytes always
	// drops the same trailing files across repeated calls.
	analyzed, edges, unresolvedEdges, excludedEdges, sourceTruncated, moreDiags :=
		analyzeGoSources(snapshot, analyzer, allFiles, modules, packageFiles, fileModule, opts.Budgets)
	diagnostics = append(diagnostics, moreDiags...)

	filesSkipped := 0
	if sourceTruncated {
		// Truncation breaks before admitting the overflowing file, and every
		// non-truncated path (including unreadable ones) is admitted, so the
		// skipped tail is exactly the inventory remainder.
		filesSkipped = len(allFiles) - len(analyzed)
		diagnostics = append(diagnostics, Diagnostic{Code: DiagFileBudgetExceeded})
	}

	analyzedPaths := filePathSet(analyzed)
	workspaceList := buildWorkspaceFacts(workspaces, modules)
	moduleList := buildModuleFacts(moduleDirs, moduleFileList, analyzedPaths)
	packageList := buildPackageFacts(packageFiles, analyzedPaths)

	return Model{
		SchemaVersion: SchemaVersion,
		Repository:    meta.Repository,
		Snapshot: Snapshot{
			Revision:           meta.Revision,
			TreeID:             meta.TreeID,
			ConfigDigest:       meta.ConfigDigest,
			BackendDigest:      meta.BackendDigest,
			BuildContextDigest: meta.BuildContextDigest,
			SelectedRoots:      selectedRootsFrom(opts.Roots),
		},
		Workspaces:  workspaceList,
		Modules:     moduleList,
		Packages:    packageList,
		Files:       analyzed,
		ImportEdges: edges,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "go_model_build",
			Complete: discovery.Complete && !sourceTruncated,
			Counts: map[string]int{
				"roots_seen":       countDistinctRoots(modules, workspaces),
				"files_seen":       len(analyzed),
				"files_skipped":    filesSkipped,
				"packages_seen":    len(packageList),
				"unresolved_edges": unresolvedEdges,
				"excluded_edges":   excludedEdges,
			},
			Budgets:     EffectiveGoBudgets(opts.Budgets),
			Diagnostics: diagnostics,
		}),
	}, nil
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

func analyzeGoSources(
	snapshot fs.FS,
	analyzer *semantics.Analyzer,
	allFiles []string,
	modules map[string]*modfile.File,
	packageFiles map[string][]string,
	fileModule map[string]string,
	budgets GoBudgets,
) (files []File, edges []ImportEdge, unresolvedEdges, excludedEdges int, truncated bool, diagnostics []Diagnostic) {
	files = make([]File, 0, len(allFiles))
	filesProcessed := 0
	bytesProcessed := int64(0)

	for _, f := range allFiles {
		if budgets.MaxInputFiles > 0 && filesProcessed >= budgets.MaxInputFiles {
			truncated = true
			break
		}

		content, readErr := fs.ReadFile(snapshot, f)
		if readErr != nil {
			files = append(files, File{ID: "file:" + f, Path: f, Language: "go"})
			diagnostics = append(diagnostics, Diagnostic{Code: DiagFileUnavailable, Path: f, Message: readErr.Error()})
			filesProcessed++
			continue
		}

		if budgets.MaxInputBytes > 0 && bytesProcessed+int64(len(content)) > budgets.MaxInputBytes {
			truncated = true
			break
		}
		bytesProcessed += int64(len(content))

		files = append(files, File{ID: "file:" + f, Path: f, Language: "go"})
		filesProcessed++

		result, analyzeErr := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path:     f,
			Language: semantics.LanguageGo,
			Content:  content,
		})
		if analyzeErr != nil {
			code := DiagFileUnavailable
			if errors.Is(analyzeErr, semantics.ErrSyntax) {
				code = DiagFileSyntaxError
			}
			diagnostics = append(diagnostics, Diagnostic{Code: code, Path: f, Message: analyzeErr.Error()})
			continue
		}

		fromID := "package:" + path.Dir(f)
		owner := modules[fileModule[f]]
		for _, imp := range result.Imports {
			kind, to := classifyGoImport(imp.Path, owner, modules, packageFiles)
			switch kind {
			case "unresolved":
				unresolvedEdges++
			case "excluded":
				excludedEdges++
			}
			edges = append(edges, ImportEdge{
				From: fromID,
				To:   to,
				Kind: kind,
				Site: fmt.Sprintf("%s:%d", f, imp.Location.StartRow+1),
			})
		}
	}
	return files, edges, unresolvedEdges, excludedEdges, truncated, diagnostics
}

func filePathSet(files []File) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, f := range files {
		out[f.Path] = true
	}
	return out
}

func buildWorkspaceFacts(workspaces map[string]*modfile.WorkFile, modules map[string]*modfile.File) []Workspace {
	workspaceList := make([]Workspace, 0, len(workspaces))
	for _, wdir := range mapKeysSorted(workspaces) {
		wf := workspaces[wdir]
		var projects []string
		seen := map[string]bool{}
		for _, use := range wf.Use {
			resolved := path.Clean(path.Join(wdir, use.Path))
			if _, ok := modules[resolved]; !ok || seen[resolved] {
				continue
			}
			seen[resolved] = true
			projects = append(projects, "module:"+resolved)
		}
		sort.Strings(projects)
		workspaceList = append(workspaceList, Workspace{
			ID:       "workspace:" + wdir,
			Language: "go",
			Root:     wdir,
			Projects: projects,
		})
	}
	return workspaceList
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

// shouldSkipModuleWalkDir reports whether the moduleGoFiles walk should
// prune p: nested module roots plus the same testdata/vendor/dot dirs
// discovery skips. The module root itself is never pruned.
func shouldSkipModuleWalkDir(p, moduleDir string, moduleDirs map[string]bool) bool {
	if p == moduleDir {
		return false
	}
	return moduleDirs[p] || shouldSkipDiscoveryDir(p)
}

// classifyGoImport assigns importPath one of the six frozen ImportEdge.Kind
// values ("internal", "stdlib", "external", "replaced", "excluded",
// "unresolved") for a file owned by owner's module, and returns the
// ImportEdge.To value that goes with that kind: a "package:" fact ID for
// "internal" edges, the raw import path for every other kind.
//
// Classification order is deliberate: stdlib and internal (workspace-local)
// imports are recognized first since they never appear in a go.mod's
// require/replace/exclude lists; then, for anything else, replace takes
// priority over exclude, which takes priority over require, since a module
// path can legally appear in more than one of those lists at once (e.g.
// required and then excluded).
func classifyGoImport(importPath string, owner *modfile.File, allModules map[string]*modfile.File, packageDirs map[string][]string) (kind, to string) {
	if isStdlibImport(importPath) {
		return "stdlib", importPath
	}

	// Iterate module directories in sorted order (never Go's randomized map
	// order) and keep the longest matching module path so results are
	// deterministic and reproducible across processes: two modules can
	// legally declare colliding module paths, or a nested module whose
	// directory doesn't mirror its module path, and either can otherwise
	// leave the choice of which one "wins" to map iteration order.
	bestModPath := ""
	bestPkgDir := ""
	matched := false
	for _, mdir := range mapKeysSorted(allModules) {
		mf := allModules[mdir]
		if mf.Module == nil {
			continue
		}
		modPath := mf.Module.Mod.Path
		if !matchesModulePrefix(importPath, modPath) {
			continue
		}
		sub := strings.TrimPrefix(strings.TrimPrefix(importPath, modPath), "/")
		pkgDir := mdir
		if sub != "" {
			pkgDir = path.Join(mdir, sub)
		}
		if _, ok := packageDirs[pkgDir]; !ok {
			continue
		}
		if !matched || len(modPath) > len(bestModPath) {
			bestModPath, bestPkgDir, matched = modPath, pkgDir, true
		}
	}
	if matched {
		return "internal", "package:" + bestPkgDir
	}

	if owner != nil {
		for _, r := range owner.Replace {
			if matchesModulePrefix(importPath, r.Old.Path) {
				return "replaced", importPath
			}
		}
		for _, e := range owner.Exclude {
			if matchesModulePrefix(importPath, e.Mod.Path) {
				return "excluded", importPath
			}
		}
		for _, req := range owner.Require {
			if matchesModulePrefix(importPath, req.Mod.Path) {
				return "external", importPath
			}
		}
	}

	return "unresolved", importPath
}

func matchesModulePrefix(importPath, modPath string) bool {
	return importPath == modPath || strings.HasPrefix(importPath, modPath+"/")
}

// isStdlibImport reports whether importPath looks like a standard-library
// import: its first path segment has no dot. This is the same heuristic
// goimports and similar tools use to separate stdlib from third-party
// imports, and holds for every module path convention in use (domains
// always contain a dot).
func isStdlibImport(importPath string) bool {
	first := importPath
	if idx := strings.Index(importPath, "/"); idx >= 0 {
		first = importPath[:idx]
	}
	return !strings.Contains(first, ".")
}

func filterToRoots(modules map[string]*modfile.File, workspaces map[string]*modfile.WorkFile, roots []string) (map[string]*modfile.File, map[string]*modfile.WorkFile) {
	allowed := func(dir string) bool {
		for _, r := range roots {
			r = path.Clean(r)
			// "." is the snapshot root and is an ancestor of every
			// repository-relative path; r+"/" would be "./", which never
			// prefixes normal paths like "modulea".
			if r == "." || dir == r || strings.HasPrefix(dir, r+"/") {
				return true
			}
		}
		return false
	}

	fm := make(map[string]*modfile.File, len(modules))
	for k, v := range modules {
		if allowed(k) {
			fm[k] = v
		}
	}
	fw := make(map[string]*modfile.WorkFile, len(workspaces))
	for k, v := range workspaces {
		if allowed(k) {
			fw[k] = v
		}
	}
	return fm, fw
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
