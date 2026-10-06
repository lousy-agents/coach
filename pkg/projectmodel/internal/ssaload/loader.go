// Package ssaload is pkg/projectmodel's driven adapter for building Go SSA
// programs: it writes an fs.FS snapshot to a temporary directory and loads
// it through golang.org/x/tools/go/packages, which runs the local Go
// toolchain against those files.
package ssaload

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Loader materializes snapshots on disk and builds one SSA program per
// module directory inside them.
type Loader struct{}

// Materialize copies snapshot into a new temporary directory so
// golang.org/x/tools/go/packages (which shells out to the Go toolchain) has
// real files to load; the caller must invoke the returned cleanup func.
func (Loader) Materialize(snapshot fs.FS) (string, func(), error) {
	dir, err := os.MkdirTemp("", "projectmodel-callgraph-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err := os.CopyFS(dir, snapshot); err != nil {
		cleanup()
		// dir is still returned (already removed by cleanup above) so a
		// caller can strip it from err's embedded absolute path before
		// surfacing err in a diagnostic.
		return dir, func() {}, err
	}
	return dir, cleanup, nil
}

// LoadModule loads moduleDir's own packages from dir/moduleDir and builds
// an SSA program for them. The load mode is packages.LoadSyntax: typed
// syntax for the snapshot's own packages, export data for dependencies.
// NeedDeps+NeedSyntax is LoadAllSyntax and would parse the standard library
// (net/http, database/sql, reflect) on every call; StaticCallee and stdlib
// type identity only need those packages' types, not their syntax.
func (Loader) LoadModule(ctx context.Context, dir, moduleDir string) ([]*packages.Package, *ssa.Program, map[string]bool, error) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     filepath.Join(dir, filepath.FromSlash(moduleDir)),
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
