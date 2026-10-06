package projectmodel

// layerBypassContainsDir mirrors codesignal's layerContainsDir: "." matches
// every directory, otherwise a prefix matches dir itself or any "/"-
// separated descendant of it.
func layerBypassContainsDir(layer BypassLayer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if prefix == "." || dir == prefix || (len(dir) > len(prefix) && dir[:len(prefix)+1] == prefix+"/") {
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
