package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// loadedGoRoot is one module root's packages.Load + SSA program. A single
// loadedGoSnapshot owns one of these per discovered root so BuildGoCallGraph,
// source identification, and layer-bypass node classification can share it.
type loadedGoRoot struct {
	dir           string
	pkgs          []*packages.Package
	prog          *ssa.Program
	localPkgPaths map[string]bool
	loadErr       error
}

// goProgramLoader is the port loadGoSnapshot materializes a snapshot and
// builds one SSA program per module root through. ssaload.Loader is the
// production adapter.
type goProgramLoader interface {
	Materialize(snapshot fs.FS) (dir string, cleanup func(), err error)
	LoadModule(ctx context.Context, dir, moduleDir string) ([]*packages.Package, *ssa.Program, map[string]bool, error)
}

// loadedGoSnapshot is one goProgramLoader.Materialize + one
// packages.Load/SSA build per module root. Callers must invoke cleanup.
type loadedGoSnapshot struct {
	tempDir     string
	cleanup     func()
	discovery   *goProjectDiscovery
	moduleDirs  []string
	roots       []loadedGoRoot
	loadStopped bool
}

func (s *loadedGoSnapshot) programsBuilt() int {
	n := 0
	for _, root := range s.roots {
		if root.loadErr == nil && root.prog != nil {
			n++
		}
	}
	return n
}

// loadGoSnapshot discovers module roots, materializes snapshot once, and
// builds one SSA program per root. ctx cancellation stops further loads
// (loadStopped) without discarding roots already built. A non-nil error
// means the snapshot could not be materialized; per-root load failures are
// recorded on loadedGoRoot.loadErr instead.
func loadGoSnapshot(ctx context.Context, loader goProgramLoader, snapshot fs.FS, roots []string, budgets GoBudgets) (*loadedGoSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	discovery := discoverGoProject(snapshot, budgets)
	modules := discovery.Modules
	if len(roots) > 0 {
		modules, _ = filterToRoots(modules, discovery.Workspaces, roots)
	}
	loaded := &loadedGoSnapshot{
		cleanup:    func() {},
		discovery:  discovery,
		moduleDirs: mapKeysSorted(modules),
	}
	if ctx.Err() != nil {
		loaded.loadStopped = true
		return loaded, nil
	}

	tempDir, cleanup, err := loader.Materialize(snapshot)
	if err != nil {
		return nil, fmt.Errorf("projectmodel: materializing snapshot for call-graph build: %w", err)
	}
	loaded.tempDir = tempDir
	loaded.cleanup = cleanup

	for _, mdir := range loaded.moduleDirs {
		if ctx.Err() != nil {
			loaded.loadStopped = true
			break
		}
		root := loadedGoRoot{dir: mdir}
		root.pkgs, root.prog, root.localPkgPaths, root.loadErr = loader.LoadModule(ctx, tempDir, mdir)
		loaded.roots = append(loaded.roots, root)
	}
	return loaded, nil
}

// stripTempDir removes the materialized snapshot's absolute temp-dir prefix from
// msg (as embedded by go/packages error text), so
// CallGraphResult.Coverage.Diagnostics stays deterministic across runs and
// across different absolute snapshot roots -- mirroring relCallSitePath's
// tempDir stripping for call-site paths.
func stripTempDir(msg, tempDir string) string {
	return strings.ReplaceAll(msg, tempDir, "")
}
