package projectmodel

import "io/fs"

// Stable TS root-discovery diagnostic codes for
// TSRootDiscoveryResult.Coverage.Diagnostics[i].Code.
const (
	DiagTSRootUnavailable = "ts_project_root_unavailable"
	DiagTSRootIncomplete  = "ts_project_root_incomplete"
)

// TSRootDiscoveryResult is DiscoverTSRoots' result. Roots names every
// directory holding a tsconfig.json (the same exact-basename match
// tsWorkspaceFactsFromCollected uses to build Model.Workspaces); Candidates
// names every directory holding a package.json but no tsconfig.json of its
// own -- a place a guided policy-authoring flow may offer to configure, not
// a project Coach has confirmed. Both are flat, deduplicated, sorted
// directory paths and nothing else: DiscoverTSRoots never groups, orders,
// or names them by architectural role, so this type has no field capable of
// expressing a layer boundary -- do not add a grouping/labeling field here.
type TSRootDiscoveryResult struct {
	Roots      []string
	Candidates []string
	Complete   bool
	Coverage   Coverage
}

// DiscoverTSRoots walks snapshot for tsconfig.json/package.json files and
// reports the directories that contain them. It requires no TypeScript
// compiler and never shells out to one: this is a pure fs.FS walk, mirroring
// DiscoverGoRoots' no-compiler contract for Go. It never descends into
// node_modules/ or dot-prefixed directories. It never returns a non-nil
// error: unreadable snapshots and budget truncation are reported through
// Coverage.Diagnostics/TSRootDiscoveryResult.Complete instead.
func DiscoverTSRoots(snapshot fs.FS, budgets GoBudgets) (TSRootDiscoveryResult, error) {
	d := discoverTSProject(snapshot, budgets)
	roots, candidates := d.rootsAndCandidates()

	return TSRootDiscoveryResult{
		Roots:      roots,
		Candidates: candidates,
		Complete:   d.Complete,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "ts_root_discovery",
			Complete: d.Complete,
			Counts: map[string]int{
				"files_seen":         d.FilesSeen,
				"files_skipped":      d.FilesSkipped,
				"tsconfigs_seen":     len(d.TSConfigDirs),
				"package_jsons_seen": len(d.PackageDirs),
				"roots_emitted":      len(roots),
				"candidates_emitted": len(candidates),
			},
			Budgets:     EffectiveGoBudgets(budgets),
			Diagnostics: d.Diagnostics,
		}),
	}, nil
}
