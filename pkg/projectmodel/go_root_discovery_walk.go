package projectmodel

import (
	"io/fs"
	"path"
	"strings"

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
	_ = walkErr // walkFn only ever returns nil or fs.SkipAll, so WalkDir never propagates an error here.

	if d.truncated {
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
