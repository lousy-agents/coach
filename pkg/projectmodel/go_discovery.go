package projectmodel

import (
	"io/fs"
	"path"
	"sort"
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
}

// discoverGoProject walks snapshot once, collecting every go.work/go.mod
// file it can parse and recording DiagRoot* diagnostics for anything it
// can't. It never returns an error: unreadable snapshots and truncated
// walks are reported through Diagnostics/Complete instead, matching
// DiscoverGoRoots' fail-open-with-diagnostics contract.
func discoverGoProject(snapshot fs.FS, budgets GoBudgets) *goProjectDiscovery {
	d := &goProjectDiscovery{
		Workspaces: map[string]*modfile.WorkFile{},
		Modules:    map[string]*modfile.File{},
		Complete:   true,
	}

	truncated := false
	walkErr := fs.WalkDir(snapshot, ".", func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return d.handleWalkError(p)
		}
		if entry.IsDir() {
			return d.visitDiscoveryDir(p)
		}
		return d.visitDiscoveryFile(snapshot, p, budgets, &truncated)
	})
	_ = walkErr // walkFn only ever returns nil or fs.SkipAll, so WalkDir never propagates an error here.

	if truncated {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootIncomplete})
	}

	return d
}

// handleWalkError applies DiscoverGoRoots' fail-open walk-error policy:
// a failure at the snapshot root is DiagRootUnavailable + SkipAll; any
// other unreadable subtree is skipped so the rest of the walk continues.
func (d *goProjectDiscovery) handleWalkError(p string) error {
	if p == "." {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootUnavailable, Path: "."})
		return fs.SkipAll
	}
	return nil
}

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
func (d *goProjectDiscovery) visitDiscoveryFile(snapshot fs.FS, p string, budgets GoBudgets, truncated *bool) error {
	d.FilesSeen++
	if budgets.MaxInputFiles > 0 && d.FilesSeen > budgets.MaxInputFiles {
		*truncated = true
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
		*truncated = true
		d.FilesSkipped++
		return fs.SkipAll
	}

	d.recordGoFile(p, base, data)
	return nil
}

// shouldSkipDiscoveryDir reports whether the walk should prune the directory
// at p: testdata/vendor fixtures and dot-prefixed directories (e.g. .git)
// never contain go.mod/go.work files relevant to root discovery.
func shouldSkipDiscoveryDir(p string) bool {
	if p == "." {
		return false
	}
	base := path.Base(p)
	return base == "testdata" || base == "vendor" || strings.HasPrefix(base, ".")
}

// recordGoFile parses the go.mod/go.work file already read at p (data) and
// records the successful module/workspace, or a DiagRootInvalid diagnostic
// on parse failure. base must be "go.mod" or "go.work"; any other value is a
// no-op, since the caller only invokes this after that check.
func (d *goProjectDiscovery) recordGoFile(p, base string, data []byte) {
	dir := path.Dir(p)
	switch base {
	case "go.mod":
		mf, parseErr := modfile.Parse(p, data, nil)
		if parseErr != nil {
			d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootInvalid, Path: p, Message: parseErr.Error()})
			d.ModulesSkipped++
			return
		}
		d.Modules[dir] = mf
	case "go.work":
		wf, parseErr := modfile.ParseWork(p, data, nil)
		if parseErr != nil {
			d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootInvalid, Path: p, Message: parseErr.Error()})
			return
		}
		d.Workspaces[dir] = wf
	}
}

// resolveUseDirectives walks every discovered go.work's use directives in
// deterministic (sorted-by-directory) order, resolving each entry relative
// to the snapshot root. It returns the DiagRootOutsideSnapshot/
// DiagRootDuplicate/DiagRootAmbiguous diagnostics those entries produce,
// plus the set of workspace directories that resolve at least one entry
// onto a known module directory (used to decide whether the workspace
// itself is emitted as a root).
func (d *goProjectDiscovery) resolveUseDirectives() ([]Diagnostic, map[string]bool) {
	validWorkspaces := map[string]bool{}
	var diagnostics []Diagnostic

	dirs := mapKeysSorted(d.Workspaces)
	ambiguousSeen := map[string]bool{}

	for _, w := range dirs {
		wf := d.Workspaces[w]
		seenInWorkspace := map[string]bool{}
		for _, use := range wf.Use {
			resolved := path.Clean(path.Join(w, use.Path))
			if resolved == ".." || strings.HasPrefix(resolved, "../") {
				diagnostics = append(diagnostics, Diagnostic{Code: DiagRootOutsideSnapshot, Path: resolved})
				continue
			}

			if seenInWorkspace[resolved] {
				diagnostics = append(diagnostics, Diagnostic{Code: DiagRootDuplicate, Path: resolved})
			}
			seenInWorkspace[resolved] = true

			if d.Workspaces[resolved] != nil && !ambiguousSeen[resolved] {
				ambiguousSeen[resolved] = true
				diagnostics = append(diagnostics, Diagnostic{Code: DiagRootAmbiguous, Path: resolved})
			}

			if _, ok := d.Modules[resolved]; ok {
				validWorkspaces[w] = true
			}
		}
	}

	return diagnostics, validWorkspaces
}

// roots returns the deduplicated, sorted set of every module directory plus
// every workspace directory in validWorkspaces (see resolveUseDirectives).
func (d *goProjectDiscovery) roots(validWorkspaces map[string]bool) []string {
	seen := make(map[string]bool, len(d.Modules)+len(validWorkspaces))
	for dir := range d.Modules {
		seen[dir] = true
	}
	for dir := range validWorkspaces {
		seen[dir] = true
	}
	out := make([]string, 0, len(seen))
	for dir := range seen {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}

// EffectiveGoBudgets renders b as the frozen budgets map vocabulary shared
// by RootDiscoveryResult.Coverage.Budgets and Model.Coverage.Budgets
// (wall_time_ms, input_files, input_bytes, graph_nodes, graph_edges,
// working_set_bytes, stderr_bytes). It is exported so a caller that needs
// to report this vocabulary before calling DiscoverGoRoots/BuildGoModel
// (e.g. a zero-value coverage for a pre-discovery failure) can reuse it
// instead of duplicating the key set.
//
// stderr_bytes always reports 0: neither entry point shells out to a
// subprocess today, but the key is reserved so a future backend that does
// can report it without changing the vocabulary.
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

func mapKeysSorted[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
