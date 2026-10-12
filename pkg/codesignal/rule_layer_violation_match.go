package codesignal

import (
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
)

type layerPairKey struct {
	importer string
	importee string
}

func buildForbiddenLayerPairSet(policy LayerPolicy) map[[2]string]struct{} {
	forbidden := make(map[[2]string]struct{}, len(policy.ForbiddenImports))
	for _, f := range policy.ForbiddenImports {
		forbidden[[2]string{f.From, f.To}] = struct{}{}
	}
	return forbidden
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

func sortedLayerPairKeys(groups map[layerPairKey][]domain.ImportEdge) []layerPairKey {
	keys := make([]layerPairKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].importer != keys[j].importer {
			return keys[i].importer < keys[j].importer
		}
		return keys[i].importee < keys[j].importee
	})
	return keys
}
