package configauthoring

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/domain"
)

func matchingDiscoveredDirectories(layer projectconfig.Layer, dirs []string) []string {
	var matched []string
	for _, dir := range dirs {
		if layerMatchesDirectory(layer, dir) {
			matched = append(matched, dir)
		}
	}
	return matched
}

func layerMatchesDirectory(layer projectconfig.Layer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if domain.LayerPrefixContains(prefix, dir) {
			return true
		}
	}
	return false
}
