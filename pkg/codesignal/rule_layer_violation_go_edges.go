package codesignal

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
)

func groupForbiddenInternalEdges(edges []domain.ImportEdge, policy LayerPolicy) (map[layerPairKey][]domain.ImportEdge, map[layerPairKey][2]string) {
	forbidden := buildForbiddenLayerPairSet(policy)

	groups := make(map[layerPairKey][]domain.ImportEdge)
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

func forbiddenInternalPair(edge domain.ImportEdge, layers []ArchitectureLayer, forbidden map[[2]string]struct{}) (layerPairKey, [2]string, bool) {
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
