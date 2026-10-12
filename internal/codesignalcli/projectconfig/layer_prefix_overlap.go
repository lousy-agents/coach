package projectconfig

import (
	"sort"
	"strings"
)

// HasDuplicateOrOverlappingPaths reports exact duplicates or ancestor/descendant
// path pairs. Used for layer prefixes, which must partition policy membership.
// Complexity is O(n log n) via sort + adjacent/ancestor checks rather than a
// nested all-pairs scan.
func HasDuplicateOrOverlappingPaths(paths []string) bool {
	if len(paths) < 2 {
		return false
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	for i := 0; i < len(sorted); i++ {
		// "." is an ancestor of every other non-empty prefix.
		if sorted[i] == "." && len(sorted) > 1 {
			return true
		}
		if sorted[i] == "." {
			continue
		}

		if adjacentPathsOverlap(sorted, i) {
			return true
		}
	}
	return false
}

func adjacentPathsOverlap(sorted []string, i int) bool {
	if i+1 >= len(sorted) {
		return false
	}
	left, right := sorted[i], sorted[i+1]
	return left == right || strings.HasPrefix(right, left+"/")
}
