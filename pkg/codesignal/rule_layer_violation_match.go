package codesignal

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

type layerPairKey struct {
	importer string
	importee string
}

func groupForbiddenInternalEdges(edges []projectmodel.ImportEdge, policy LayerPolicy) (map[layerPairKey][]projectmodel.ImportEdge, map[layerPairKey][2]string) {
	forbidden := make(map[[2]string]struct{}, len(policy.ForbiddenImports))
	for _, f := range policy.ForbiddenImports {
		forbidden[[2]string{f.From, f.To}] = struct{}{}
	}

	groups := make(map[layerPairKey][]projectmodel.ImportEdge)
	groupLayers := make(map[layerPairKey][2]string)
	for _, edge := range edges {
		key, layers, ok := forbiddenInternalPair(edge, policy.Layers, forbidden)
		if !ok {
			continue
		}
		groups[key] = append(groups[key], edge)
		groupLayers[key] = layers
	}
	return groups, groupLayers
}

func forbiddenInternalPair(edge projectmodel.ImportEdge, layers []ArchitectureLayer, forbidden map[[2]string]struct{}) (layerPairKey, [2]string, bool) {
	if edge.Kind != "internal" {
		return layerPairKey{}, [2]string{}, false
	}
	// "package:" is Go's ImportEdge.From/To addressing scheme (see
	// pkg/projectmodel/go_imports.go); TS/TSX edges use a "file:" prefix
	// instead, so a TS edge reaching here would silently match no layer.
	importerDir := strings.TrimPrefix(edge.From, "package:")
	importeeDir := strings.TrimPrefix(edge.To, "package:")

	layerFrom, okFrom := matchLayer(layers, importerDir)
	layerTo, okTo := matchLayer(layers, importeeDir)
	if !okFrom || !okTo {
		return layerPairKey{}, [2]string{}, false
	}
	if _, isForbidden := forbidden[[2]string{layerFrom.Name, layerTo.Name}]; !isForbidden {
		return layerPairKey{}, [2]string{}, false
	}
	return layerPairKey{importer: importerDir, importee: importeeDir}, [2]string{layerFrom.Name, layerTo.Name}, true
}

// matchLayer returns the first layer whose Prefixes contains dir or an
// ancestor of dir. Prefixes are guaranteed non-overlapping by caller-side
// schema validation, so at most one layer can ever match; "first match
// wins" is defensive only and should be unreachable in practice.
//
// Prefix "." is the universal repository-root ancestor: it matches every
// package directory, consistent with config validation treating "." as an
// ancestor of every other prefix (hasDuplicateOrOverlappingPaths).
func matchLayer(layers []ArchitectureLayer, dir string) (ArchitectureLayer, bool) {
	for _, layer := range layers {
		if layerContainsDir(layer, dir) {
			return layer, true
		}
	}
	return ArchitectureLayer{}, false
}

func layerContainsDir(layer ArchitectureLayer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if prefix == "." || dir == prefix || strings.HasPrefix(dir, prefix+"/") {
			return true
		}
	}
	return false
}
