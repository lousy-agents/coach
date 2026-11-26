package evalpatterns

import "strings"

// checkoutPolicy is the part of a checkout's .lousy-agents/coach.json the
// layer measure reads.
type checkoutPolicy struct {
	Layers []struct {
		Name     string   `json:"name"`
		Prefixes []string `json:"prefixes"`
	} `json:"layers"`
	ForbiddenImports []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"forbidden_imports"`
}

// layerOf is the layer whose longest prefix holds dir, or "".
func (policy checkoutPolicy) layerOf(dir string) string {
	layer, longest := "", -1
	for _, candidate := range policy.Layers {
		if length := longestPrefix(candidate.Prefixes, dir); length > longest {
			layer, longest = candidate.Name, length
		}
	}
	return layer
}

// longestPrefix is the length of the longest prefix that holds dir, a whole
// path segment at a time, or -1.
func longestPrefix(prefixes []string, dir string) int {
	longest := -1
	for _, prefix := range prefixes {
		if dir == prefix || strings.HasPrefix(dir, prefix+"/") {
			longest = max(longest, len(prefix))
		}
	}
	return longest
}

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

// isPermutation reports whether the ranks are exactly 0 to n-1: every pair
// of layers is forbidden one way, so the forbidden pairs order them.
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
