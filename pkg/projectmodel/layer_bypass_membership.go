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

// layerBypassContainsPath reports whether any prefix of layer contains the file
// at filePath: the file itself, or any file under a directory the prefix names.
func layerBypassContainsPath(layer BypassLayer, filePath string) bool {
	for _, prefix := range layer.Prefixes {
		if domain.LayerPrefixContains(prefix, filePath) {
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
