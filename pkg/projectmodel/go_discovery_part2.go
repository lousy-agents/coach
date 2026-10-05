package projectmodel

import (
	"io/fs"
	"path"
	"sort"

	"golang.org/x/mod/modfile"
)

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

	walkErr := fs.WalkDir(snapshot, ".", func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return d.handleWalkError(p)
		}
		if entry.IsDir() {
			return d.visitDiscoveryDir(p)
		}
		return d.visitDiscoveryFile(snapshot, p, budgets)
	})
	_ = walkErr

	if d.truncated {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootIncomplete})
	}

	return d
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
func (r *useDirectiveResolution) resolveWorkspace(d *goProjectDiscovery, w string) {
	ws := workspaceUseResolution{useDirectiveResolution: r, seen: map[string]bool{}}
	for _, use := range d.Workspaces[w].Use {
		ws.resolveUse(d, w, use.Path)
	}
}
