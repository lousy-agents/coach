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

// forbiddenTSPair implements the eligibility rule documented on
// EvaluateTypeScriptLayerViolations.
func forbiddenTSPair(edge domain.ImportEdge, layers []ArchitectureLayer, forbidden map[[2]string]struct{}) (layerPairKey, [2]string, bool) {
	if edge.Kind != "import" && edge.Kind != "reexport" {
		return layerPairKey{}, [2]string{}, false
	}
	importerFile, okFrom := strings.CutPrefix(edge.From, "file:")
	importeeFile, okTo := strings.CutPrefix(edge.To, "file:")
	if !okFrom || !okTo {
		return layerPairKey{}, [2]string{}, false
	}

	layerFrom, okFrom := matchLayer(layers, importerFile)
	layerTo, okTo := matchLayer(layers, importeeFile)
	if !okFrom || !okTo {
		return layerPairKey{}, [2]string{}, false
	}
	if _, isForbidden := forbidden[[2]string{layerFrom.Name, layerTo.Name}]; !isForbidden {
		return layerPairKey{}, [2]string{}, false
	}
	return layerPairKey{importer: importerFile, importee: importeeFile}, [2]string{layerFrom.Name, layerTo.Name}, true
}
