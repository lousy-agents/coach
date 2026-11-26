package evalpatterns

// ranks counts, for each layer, the other layers it may not import, each
// once however often the pair is listed: the outermost layer may import
// every other one, so it ranks 0.
func (policy checkoutPolicy) ranks(names []string) map[string]int {
	present := map[string]bool{}
	for _, name := range names {
		present[name] = true
	}
	ranks := map[string]int{}
	for _, name := range names {
		ranks[name] = 0
	}
	counted := map[[2]string]bool{}
	for _, pair := range policy.ForbiddenImports {
		key := [2]string{pair.From, pair.To}
		if present[pair.From] && present[pair.To] && pair.From != pair.To && !counted[key] {
			counted[key] = true
			ranks[pair.From]++
		}
	}
	return ranks
}

// forbidsBothWays reports whether two of the named layers each forbid the
// other.
func (policy checkoutPolicy) forbidsBothWays(names []string) bool {
	present := map[string]bool{}
	for _, name := range names {
		present[name] = true
	}
	forbidden := map[[2]string]bool{}
	for _, pair := range policy.ForbiddenImports {
		forbidden[[2]string{pair.From, pair.To}] = present[pair.From] && present[pair.To] && pair.From != pair.To
	}
	for pair := range forbidden {
		if forbidden[[2]string{pair[1], pair[0]}] {
			return true
		}
	}
	return false
}

// isPermutation reports whether the ranks are exactly 0 to n-1. With no
// pair forbidden both ways, that means every pair of layers is forbidden
// exactly one way, so the forbidden pairs order them.
func isPermutation(ranks map[string]int) bool {
	seen := make([]bool, len(ranks))
	for _, rank := range ranks {
		if rank >= len(ranks) || seen[rank] {
			return false
		}
		seen[rank] = true
	}
	return true
}
