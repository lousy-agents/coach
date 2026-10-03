package projectmodel

import (
	"context"
	"fmt"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"path/filepath"
)

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
