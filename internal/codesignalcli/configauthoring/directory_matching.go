package configauthoring

import (
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
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
		if directoryHasPrefix(dir, prefix) {
			return true
		}
	}
	return false
}

// directoryHasPrefix reports whether prefix matches dir the same way a real
// layer-violation evaluation would (pkg/codesignal/rule_layer_violation_match.go's
// layerContainsDir): dir equals prefix, dir is nested under prefix, or prefix
// is ".", the universal repository-root ancestor.
func directoryHasPrefix(dir, prefix string) bool {
	return prefix == "." || dir == prefix || strings.HasPrefix(dir, prefix+"/")
}
