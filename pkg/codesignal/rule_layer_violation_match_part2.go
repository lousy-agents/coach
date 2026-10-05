package codesignal

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
)

func groupForbiddenTSEdges(edges []domain.ImportEdge, policy LayerPolicy) (map[layerPairKey][]domain.ImportEdge, map[layerPairKey][2]string) {
	forbidden := buildForbiddenLayerPairSet(policy)

	groups := make(map[layerPairKey][]domain.ImportEdge)
	groupLayers := make(map[layerPairKey][2]string)
	for _, edge := range edges {
		key, layers, ok := forbiddenTSPair(edge, policy.Layers, forbidden)
		if !ok {
			continue
		}
		groups[key] = append(groups[key], edge)
		groupLayers[key] = layers
	}
	return groups, groupLayers
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
