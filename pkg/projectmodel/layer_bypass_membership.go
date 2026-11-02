package projectmodel

import "github.com/lousy-agents/coach/pkg/domain"

func layerBypassContainsDir(layer BypassLayer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if domain.LayerPrefixContains(prefix, dir) {
			return true
		}
	}
	return false
}

func requiredLayerNodeSet(layer BypassLayer, nodePositions map[string]layerBypassNodePosition) map[string]bool {
	nodes := map[string]bool{}
	for node, pos := range nodePositions {
		if layerBypassContainsDir(layer, pos.Dir) {
			nodes[node] = true
		}
	}
	return nodes
}
