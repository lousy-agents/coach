package projectmodel

import "time"

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

// EffectiveGoBudgets renders b as the frozen budgets map vocabulary shared
// by RootDiscoveryResult.Coverage.Budgets, Model.Coverage.Budgets,
// CallGraphResult.Coverage.Budgets, and (with an added search_nodes key)
// ReachabilityResult.Coverage.Budgets. It is exported so a caller that
// needs to report this vocabulary before calling
// DiscoverGoRoots/BuildGoModel (e.g. a zero-value coverage for a
// pre-discovery failure) can reuse it instead of duplicating the key set.
//
// stderr_bytes always reports 0: the call-graph/reachability path
// (BuildGoCallGraph, and BuildGoReachability through it) does shell out
// via go/packages (which invokes the Go toolchain) but does not capture
// stderr, so the key stays 0; it is reserved so a future backend that does
// capture stderr can report it without changing the vocabulary.
func EffectiveGoBudgets(b GoBudgets) map[string]int {
	return map[string]int{
		"wall_time_ms":      int(b.WallTime / time.Millisecond),
		"input_files":       b.MaxInputFiles,
		"input_bytes":       int(b.MaxInputBytes),
		"graph_nodes":       b.MaxGraphNodes,
		"graph_edges":       b.MaxGraphEdges,
		"working_set_bytes": int(b.MaxWorkingSetBytes),
		"stderr_bytes":      0,
	}
}
