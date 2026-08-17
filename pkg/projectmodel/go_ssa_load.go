package projectmodel

import (
	"context"
	"fmt"
	"go/types"
	"io/fs"
	"path/filepath"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
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

// loadedGoSnapshot is one materializeSnapshot + one packages.Load/SSA build
// per module root. Callers must invoke cleanup.
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
func loadGoSnapshot(ctx context.Context, snapshot fs.FS, roots []string, budgets GoBudgets) (*loadedGoSnapshot, error) {
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

	tempDir, cleanup, err := materializeSnapshot(snapshot)
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
		root.pkgs, root.prog, root.localPkgPaths, root.loadErr = loadGoSSAProgram(ctx, tempDir, mdir)
		loaded.roots = append(loaded.roots, root)
	}
	return loaded, nil
}

// loadGoSSAProgram loads moduleDir's own packages from tempDir/moduleDir and
// builds an SSA program for them. The load mode is packages.LoadSyntax: typed
// syntax for the snapshot's own packages, export data for dependencies.
// NeedDeps+NeedSyntax is LoadAllSyntax and would parse the standard library
// (net/http, database/sql, reflect) on every call; StaticCallee and stdlib
// type identity only need those packages' types, not their syntax.
func loadGoSSAProgram(ctx context.Context, tempDir, moduleDir string) ([]*packages.Package, *ssa.Program, map[string]bool, error) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     filepath.Join(tempDir, filepath.FromSlash(moduleDir)),
		Mode:    packages.LoadSyntax,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, nil, nil, err
	}
	if len(pkgs) == 0 {
		return nil, nil, nil, fmt.Errorf("no Go packages found under %q", moduleDir)
	}

	localPkgPaths := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		localPkgPaths[p.PkgPath] = true
	}

	prog, _ := ssautil.AllPackages(pkgs, 0)
	prog.Build()
	return pkgs, prog, localPkgPaths, nil
}

// typesPackageByPath returns the *types.Package for pkgPath from prog if
// ssautil created an SSA package for it, otherwise from the initial
// packages' type-checker import graph. LoadSyntax does not populate
// packages.Package.Types on dependencies, so prog.ImportedPackage can be
// nil for net/http even when the fixture type-checked against it.
func typesPackageByPath(prog *ssa.Program, pkgs []*packages.Package, pkgPath string) *types.Package {
	if prog != nil {
		if pkg := prog.ImportedPackage(pkgPath); pkg != nil {
			return pkg.Pkg
		}
	}
	for _, p := range pkgs {
		if p.Types == nil {
			continue
		}
		if p.PkgPath == pkgPath {
			return p.Types
		}
		for _, ip := range p.Types.Imports() {
			if ip.Path() == pkgPath {
				return ip
			}
		}
	}
	return nil
}
