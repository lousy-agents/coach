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
	for _, path := range paths {
		// "." is an ancestor of every other non-empty prefix.
		if path == "." {
			return true
		}
	}

	// The trailing separator keeps every descendant adjacent to its ancestor in
	// sort order. Without it a sibling such as "core-utils" ('-' sorts before
	// '/') lands between "core" and "core/src" and hides the pair.
	sorted := make([]string, len(paths))
	for i, path := range paths {
		sorted[i] = path + "/"
	}
	sort.Strings(sorted)
	for i := 0; i+1 < len(sorted); i++ {
		if strings.HasPrefix(sorted[i+1], sorted[i]) {
			return true
		}
	}
	return false
}
