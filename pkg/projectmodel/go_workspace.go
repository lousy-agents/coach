package projectmodel

import (
	"io/fs"
	"time"
)

// GoBudgets bounds one Go discovery/build call. A zero field means
// unbounded for that dimension -- there is no implicit default ceiling;
// callers that want a safe limit must set it explicitly. MaxInputFiles and
// MaxInputBytes are enforced by discoverGoProject (the go.work/go.mod walk)
// and BuildGoModel's source-file read/analyze phase. WallTime,
// MaxGraphNodes, and MaxGraphEdges are enforced by BuildGoCallGraph (wall
// clock via context deadline; nodes/edges by bounding its local-function
// call-site walk). MaxWorkingSetBytes remains reserved for future
// enforcement: it is echoed back in Coverage.Budgets but has no truncating
// effect anywhere yet.
type GoBudgets struct {
	WallTime      time.Duration
	MaxInputFiles int
	MaxInputBytes int64
	MaxGraphNodes int
	MaxGraphEdges int
	// MaxWorkingSetBytes is reserved; not enforced yet.
	MaxWorkingSetBytes int64
}

// Stable root-discovery diagnostic codes for RootDiscoveryResult.Coverage.Diagnostics[i].Code.
const (
	DiagRootUnavailable     = "project_root_unavailable"
	DiagRootOutsideSnapshot = "project_root_outside_snapshot"
	DiagRootInvalid         = "project_root_invalid"
	DiagRootDuplicate       = "project_root_duplicate"
	DiagRootAmbiguous       = "project_root_ambiguous"
	DiagRootIncomplete      = "project_root_incomplete"
)

// RootDiscoveryResult is the public, candidate-only-consumer discovery
// result (issue #210, consumed later by issue #220's initializer).
type RootDiscoveryResult struct {
	Roots    []string
	Complete bool
	Coverage Coverage
}

// DiscoverGoRoots walks snapshot for go.work/go.mod files and returns every
// distinct workspace/module root. A directory with a go.mod is always a
// root; a directory with a go.work is a root only if at least one of its
// use directives resolves onto a module root (an empty or fully-escaping
// workspace contributes nothing on its own). The walk never descends into
// testdata/, vendor/, or dot-prefixed directories, matching the go tool's
// own convention, so a go.mod/go.work nested inside one of those is never
// reported as a root. It never returns a non-nil error: unreadable
// snapshots and budget truncation are reported through
// Coverage.Diagnostics/RootDiscoveryResult.Complete instead.
func DiscoverGoRoots(snapshot fs.FS, budgets GoBudgets) (RootDiscoveryResult, error) {
	d := discoverGoProject(snapshot, budgets)
	useDiagnostics, validWorkspaces := d.resolveUseDirectives()
	diagnostics := append(append([]Diagnostic{}, d.Diagnostics...), useDiagnostics...)
	roots := d.roots(validWorkspaces)

	return RootDiscoveryResult{
		Roots:    roots,
		Complete: d.Complete,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "go_root_discovery",
			Complete: d.Complete,
			Counts: map[string]int{
				"files_seen":      d.FilesSeen,
				"files_skipped":   d.FilesSkipped,
				"modules_seen":    len(d.Modules),
				"modules_skipped": d.ModulesSkipped,
				"roots_emitted":   len(roots),
			},
			Budgets:     EffectiveGoBudgets(budgets),
			Diagnostics: diagnostics,
		}),
	}, nil
}
