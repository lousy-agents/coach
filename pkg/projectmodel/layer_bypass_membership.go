package projectmodel

import "github.com/lousy-agents/coach/pkg/domain"

func requiredLayerNodeSet(layer BypassLayer, nodePositions map[string]layerBypassNodePosition) map[string]bool {
	nodes := map[string]bool{}
	for node, pos := range nodePositions {
		if domain.AnyLayerPrefixContains(layer.Prefixes, pos.Dir) {
			nodes[node] = true
		}
	}
	return nodes
}
