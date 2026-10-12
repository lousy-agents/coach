package projectmodel

import (
	"context"
	"path"
	"path/filepath"

	"golang.org/x/tools/go/ssa"
)

// layerBypassNodePackageDirsFromLoaded walks loaded's local functions,
// returning every local function's RelString(nil) identity mapped to its
// resolved declaration position. This walk stays separate from the
// call-graph walk because CallFact's From/To strings carry no directory or
// position information, and separate from source identification because it
// needs every local function, not just handler-shaped ones.
func layerBypassNodePackageDirsFromLoaded(ctx context.Context, loaded *loadedGoSnapshot) (map[string]layerBypassNodePosition, bool, []Diagnostic) {
	if ctx.Err() != nil {
		return nil, false, []Diagnostic{{Code: DiagLayerBypassBudgetExceeded}}
	}

	acc := &layerBypassPositionAcc{
		positions: map[string]layerBypassNodePosition{},
		complete:  loaded.discovery.Complete,
		tempDir:   loaded.tempDir,
	}

	for _, root := range loaded.roots {
		if ctx.Err() != nil {
			acc.complete = false
			acc.diagnostics = append(acc.diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded, Path: root.dir})
			break
		}
		if root.loadErr != nil {
			acc.complete = false
			acc.diagnostics = append(acc.diagnostics, Diagnostic{Code: DiagLayerBypassSourceLoadFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
			continue
		}
		acc.addFunctions(ctx, root)
	}

	return acc.positions, acc.complete, acc.diagnostics
}

type layerBypassPositionAcc struct {
	positions   map[string]layerBypassNodePosition
	diagnostics []Diagnostic
	complete    bool
	tempDir     string
}

func (a *layerBypassPositionAcc) addFunctions(ctx context.Context, root loadedGoRoot) {
	for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
		if ctx.Err() != nil {
			a.complete = false
			a.diagnostics = append(a.diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded, Path: root.dir})
			return
		}
		pos, ok := fnPosition(a.tempDir, fn)
		if !ok {
			continue
		}
		a.positions[fn.RelString(nil)] = pos
	}
}

// fnPosition resolves fn's declaration position to a repository-relative
// package directory, file, and 1-based line, stripping the materialized
// snapshot's absolute tempDir prefix the same way relCallSitePath does for
// call sites.
// It reports false for a function with no resolvable position (e.g. a
// synthetic wrapper).
func fnPosition(tempDir string, fn *ssa.Function) (layerBypassNodePosition, bool) {
	pos := fn.Prog.Fset.Position(fn.Pos())
	if pos.Filename == "" {
		return layerBypassNodePosition{}, false
	}
	rel, err := filepath.Rel(tempDir, pos.Filename)
	if err != nil {
		return layerBypassNodePosition{}, false
	}
	relSlash := filepath.ToSlash(rel)
	return layerBypassNodePosition{Dir: path.Dir(relSlash), File: relSlash, Line: pos.Line}, true
}
