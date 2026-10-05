package projectmodel

import (
	"golang.org/x/tools/go/ssa"

	"path"
	"path/filepath"
)

func requiredLayerNodeSet(layer BypassLayer, nodePositions map[string]layerBypassNodePosition) map[string]bool {
	nodes := map[string]bool{}
	for node, pos := range nodePositions {
		if layerBypassContainsDir(layer, pos.Dir) {
			nodes[node] = true
		}
	}
	return nodes
}

// fnPosition resolves fn's declaration position to a repository-relative
// package directory, file, and 1-based line, stripping materializeSnapshot's
// absolute tempDir prefix the same way relCallSitePath does for call sites.
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

// layerBypassSteps converts stepPath (reconstructReachabilityPath's shared
// ReachabilityStep shape) into LayerBypassSteps, filling Path/Line from
// nodePositions for whichever nodes resolved a position -- leaving them
// zero-valued for the rest (typically only the sink).
func layerBypassSteps(stepPath []ReachabilityStep, nodePositions map[string]layerBypassNodePosition) []LayerBypassStep {
	steps := make([]LayerBypassStep, len(stepPath))
	for i, step := range stepPath {
		ls := LayerBypassStep{NodeID: step.NodeID}
		if pos, ok := nodePositions[step.NodeID]; ok {
			ls.Path = pos.File
			ls.Line = pos.Line
		}
		steps[i] = ls
	}
	return steps
}
