package projectmodel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

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

type goSourceAnalysis struct {
	files           []File
	edges           []ImportEdge
	unresolvedEdges int
	excludedEdges   int
	truncated       bool
	diagnostics     []Diagnostic
	filesProcessed  int
	bytesProcessed  int64
}

func (a *goSourceAnalysis) consume(
	snapshot fs.FS,
	analyzer *semantics.Analyzer,
	f string,
	modules map[string]*modfile.File,
	packageFiles map[string][]string,
	fileModule map[string]string,
	budgets GoBudgets,
) (stop bool) {
	if budgets.MaxInputFiles > 0 && a.filesProcessed >= budgets.MaxInputFiles {
		a.truncated = true
		return true
	}
	content, readErr := fs.ReadFile(snapshot, f)
	if readErr != nil {
		a.files = append(a.files, File{ID: "file:" + f, Path: f, Language: "go"})
		a.diagnostics = append(a.diagnostics, Diagnostic{Code: DiagFileUnavailable, Path: f, Message: readErr.Error()})
		a.filesProcessed++
		return false
	}
	if budgets.MaxInputBytes > 0 && a.bytesProcessed+int64(len(content)) > budgets.MaxInputBytes {
		a.truncated = true
		return true
	}
	a.bytesProcessed += int64(len(content))
	a.files = append(a.files, File{ID: "file:" + f, Path: f, Language: "go"})
	a.filesProcessed++

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
		a.diagnostics = append(a.diagnostics, Diagnostic{Code: code, Path: f, Message: analyzeErr.Error()})
		return false
	}

	fromID := "package:" + path.Dir(f)
	owner := modules[fileModule[f]]
	for _, imp := range result.Imports {
		kind, to := classifyGoImport(imp.Path, owner, modules, packageFiles)
		switch kind {
		case "unresolved":
			a.unresolvedEdges++
		case "excluded":
			a.excludedEdges++
		}
		a.edges = append(a.edges, ImportEdge{
			From: fromID,
			To:   to,
			Kind: kind,
			Site: fmt.Sprintf("%s:%d", f, imp.Location.StartRow+1),
		})
	}
	return false
}

// selectedRootsFrom cleans and sorts roots for Snapshot.SelectedRoots, so
// callers passing the same roots in a different order still produce
// byte-identical canonical Model JSON. A nil/empty roots yields a nil
// slice, matching Snapshot.SelectedRoots' omitempty contract.

// moduleGoFiles returns every .go file under moduleDir, repository-relative
// and sorted, excluding any subtree that is itself a different module's
// root (moduleDirs), plus any testdata/, vendor/, or dot-prefixed
// subdirectory -- the same directories the go tool itself never walks into.

// shouldSkipModuleWalkDir reports whether the moduleGoFiles walk should
// prune p: nested module roots plus the same testdata/vendor/dot dirs
// discovery skips. The module root itself is never pruned.

// classifyGoImport assigns importPath one of the six frozen ImportEdge.Kind
// values ("internal", "stdlib", "external", "replaced", "excluded",
// "unresolved") for a file owned by owner's module, and returns the
// ImportEdge.To value that goes with that kind: a "package:" fact ID for
// "internal" edges, the raw import path for every other kind.
//
// Classification order is deliberate: internal (workspace-local) module-path
// matching is checked before the stdlib heuristic, because isStdlibImport is
// a dot-only heuristic that can't distinguish a stdlib name from a dotless
// workspace module path (e.g. "module myapp" from `go mod init myapp`). A
// module path that shadows a stdlib name (e.g. `go mod init fmt`) is an
// accepted, vanishingly rare trade-off. Beyond that, replace outranks
// exclude, which outranks require, since a module path can legally appear in
// more than one of those lists (e.g. required and then excluded).

// bestInternalModulePackage finds importPath's owning package directory
// among allModules's declared modules, iterating in sorted directory order
// (never Go's randomized map order) and keeping the longest matching
// module path so the result is deterministic and reproducible across
// processes: two modules can legally declare colliding module paths, or a
// nested module whose directory doesn't mirror its module path, and either
// can otherwise leave the choice of which one "wins" to map iteration
// order.

// classifyGoImportViaOwner checks importPath against owner's own
// replace/exclude/require directives, in that precedence order.

func matchesModulePrefix(importPath, modPath string) bool {
	return importPath == modPath || strings.HasPrefix(importPath, modPath+"/")
}

// isStdlibImport reports whether importPath looks like a standard-library
// import: its first path segment has no dot. This is the same heuristic
// goimports and similar tools use to separate stdlib from third-party
// imports. It cannot distinguish a stdlib name from a dotless module path
// (`go mod init myapp`), so callers must match workspace module paths first
// — see classifyGoImport.

// "." is the snapshot root and is an ancestor of every
// repository-relative path; r+"/" would be "./", which never
// prefixes normal paths like "modulea".
