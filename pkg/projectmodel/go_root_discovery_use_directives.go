package projectmodel

import (
	"path"
	"sort"
	"strings"
)

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

func (r *useDirectiveResolution) resolveWorkspace(d *goProjectDiscovery, w string) {
	ws := workspaceUseResolution{useDirectiveResolution: r, seen: map[string]bool{}}
	for _, use := range d.Workspaces[w].Use {
		ws.resolveUse(d, w, use.Path)
	}
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
