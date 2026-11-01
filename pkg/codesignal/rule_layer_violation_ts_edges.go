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
		key, layers, ok := forbiddenTSPair(edge, policy.Layers, forbidden, policy.ExcludeTypeOnlyEdges)
		if !ok {
			continue
		}
		groups[key] = append(groups[key], edge)
		groupLayers[key] = layers
	}
	return groups, groupLayers
}

// evaluatedTSEdgeKind is the one place that decides which sidecar edge kinds
// layer rules evaluate, so the violation path and the diagnostics for
// targets it cannot place never disagree about an edge.
func evaluatedTSEdgeKind(kind string, excludeTypeOnly bool) bool {
	switch kind {
	case "import", "reexport", "commonjs_require", "dynamic_import":
		return true
	case "type_only":
		return !excludeTypeOnly
	}
	return false
}

// forbiddenTSPair implements the eligibility rule documented on
// EvaluateTypeScriptLayerViolations.
func forbiddenTSPair(edge domain.ImportEdge, layers []ArchitectureLayer, forbidden map[[2]string]struct{}, excludeTypeOnly bool) (layerPairKey, [2]string, bool) {
	if !evaluatedTSEdgeKind(edge.Kind, excludeTypeOnly) {
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
