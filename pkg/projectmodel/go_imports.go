package projectmodel

import (
	"fmt"
	"io/fs"

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
