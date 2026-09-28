package codesignal

import (
	"sort"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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

// parseSiteLocation splits an ImportEdge.Site ("<file>:<1-based line>") on
// the last colon (file paths never contain one, but this stays defensive)
// and converts the 1-based line to the 0-based StartRow ProjectLocation
// uses elsewhere in this package.
func parseSiteLocation(site string) ProjectLocation {
	idx := strings.LastIndex(site, ":")
	if idx < 0 {
		return ProjectLocation{Path: site}
	}
	path := site[:idx]
	row, err := strconv.Atoi(site[idx+1:])
	if err != nil || row <= 0 {
		return ProjectLocation{Path: path}
	}
	return ProjectLocation{Path: path, Location: semantics.Location{StartRow: uint(row - 1)}}
}
