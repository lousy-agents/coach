package projectmodel

import (
	"io/fs"
	"path"

	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

// goProjectDiscovery is the shared result of walking a Go snapshot for
// go.work/go.mod files. DiscoverGoRoots and BuildGoModel both build on it so
// root-discovery semantics (what counts as a workspace/module directory,
// which diagnostics fire) never drift between the two entry points.
type goProjectDiscovery struct {
	// Workspaces maps a snapshot-relative directory to its successfully
	// parsed go.work file.
	Workspaces map[string]*modfile.WorkFile
	// Modules maps a snapshot-relative directory to its successfully
	// parsed go.mod file.
	Modules     map[string]*modfile.File
	Diagnostics []Diagnostic
	Complete    bool
	FilesSeen   int
	BytesSeen   int64
	// FilesSkipped counts files the walk never processed because a budget
	// was already exhausted (the file that crosses MaxInputFiles/
	// MaxInputBytes and everything after it, since the walk stops there).
	FilesSkipped int
	// ModulesSkipped counts go.mod files that were seen but failed to
	// parse, distinct from go.work parse failures (which are not modules).
	ModulesSkipped int
	truncated      bool
}

// discoverGoProject walks snapshot once, collecting every go.work/go.mod
// file it can parse and recording DiagRoot* diagnostics for anything it
// can't. It never returns an error: unreadable snapshots and truncated
// walks are reported through Diagnostics/Complete instead, matching
// DiscoverGoRoots' fail-open-with-diagnostics contract.

// walkFn only ever returns nil or fs.SkipAll, so WalkDir never propagates an error here.

// handleWalkError applies DiscoverGoRoots' fail-open walk-error policy:
// a failure at the snapshot root is DiagRootUnavailable + SkipAll; any
// other unreadable subtree is skipped so the rest of the walk continues.

func (d *goProjectDiscovery) visitDiscoveryDir(p string) error {
	if shouldSkipDiscoveryDir(p) {
		return fs.SkipDir
	}
	return nil
}

// visitDiscoveryFile counts p against MaxInputFiles/MaxInputBytes and, for
// go.mod/go.work paths, reads and records them. A go.mod/go.work path that
// WalkDir enumerated but whose content can't be read (e.g. a missing Git
// blob object) is the same "snapshot cannot be read" case DiagRootUnavailable
// already covers for the top-level walk failure -- do not silently drop it,
// or a multi-root discovery can go Complete with a wrong, truncated root set
// instead of failing closed.
func (d *goProjectDiscovery) visitDiscoveryFile(snapshot fs.FS, p string, budgets GoBudgets) error {
	d.FilesSeen++
	if budgets.MaxInputFiles > 0 && d.FilesSeen > budgets.MaxInputFiles {
		d.truncated = true
		d.FilesSkipped++
		return fs.SkipAll
	}

	base := path.Base(p)
	if base != "go.mod" && base != "go.work" {
		return nil
	}

	data, readErr := fs.ReadFile(snapshot, p)
	if readErr != nil {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootUnavailable, Path: p})
		return nil
	}
	d.BytesSeen += int64(len(data))
	if budgets.MaxInputBytes > 0 && d.BytesSeen > budgets.MaxInputBytes {
		d.truncated = true
		d.FilesSkipped++
		return fs.SkipAll
	}

	d.recordGoFile(p, base, data)
	return nil
}

// shouldSkipDiscoveryDir reports whether the walk should prune the directory
// at p: testdata/vendor fixtures and dot-prefixed directories (e.g. .git)
// never contain go.mod/go.work files relevant to root discovery.

// recordGoFile parses the go.mod/go.work file already read at p (data) and
// records the successful module/workspace, or a DiagRootInvalid diagnostic
// on parse failure. base must be "go.mod" or "go.work"; any other value is a
// no-op, since the caller only invokes this after that check.

// resolveUseDirectives walks every discovered go.work's use directives in
// deterministic (sorted-by-directory) order, resolving each entry relative
// to the snapshot root. It returns the DiagRootOutsideSnapshot/
// DiagRootDuplicate/DiagRootAmbiguous diagnostics those entries produce,
// plus the set of workspace directories that resolve at least one entry
// onto a known module directory (used to decide whether the workspace
// itself is emitted as a root).
func (d *goProjectDiscovery) resolveUseDirectives() ([]Diagnostic, map[string]bool) {
	resolution := useDirectiveResolution{
		validWorkspaces: map[string]bool{},
		ambiguousSeen:   map[string]bool{},
	}
	for _, w := range mapKeysSorted(d.Workspaces) {
		resolution.resolveWorkspace(d, w)
	}
	return resolution.diagnostics, resolution.validWorkspaces
}

type useDirectiveResolution struct {
	diagnostics     []Diagnostic
	validWorkspaces map[string]bool
	ambiguousSeen   map[string]bool
}

type workspaceUseResolution struct {
	*useDirectiveResolution
	seen map[string]bool
}

func (ws *workspaceUseResolution) resolveUse(d *goProjectDiscovery, w, usePath string) {
	resolved := path.Clean(path.Join(w, usePath))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		ws.diagnostics = append(ws.diagnostics, Diagnostic{Code: DiagRootOutsideSnapshot, Path: resolved})
		return
	}
	if ws.seen[resolved] {
		ws.diagnostics = append(ws.diagnostics, Diagnostic{Code: DiagRootDuplicate, Path: resolved})
	}
	ws.seen[resolved] = true
	if d.Workspaces[resolved] != nil && !ws.ambiguousSeen[resolved] {
		ws.ambiguousSeen[resolved] = true
		ws.diagnostics = append(ws.diagnostics, Diagnostic{Code: DiagRootAmbiguous, Path: resolved})
	}
	if _, ok := d.Modules[resolved]; ok {
		ws.validWorkspaces[w] = true
	}
}

// roots returns the deduplicated, sorted set of every module directory plus
// every workspace directory in validWorkspaces (see resolveUseDirectives).

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
