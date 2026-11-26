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
// path segment at a time, or -1. The prefix "." holds every dir, as in
// Coach's own layer matcher, and is the shortest prefix of all.
func longestPrefix(prefixes []string, dir string) int {
	longest := -1
	for _, prefix := range prefixes {
		if prefix == "." {
			longest = max(longest, 0)
		} else if dir == prefix || strings.HasPrefix(dir, prefix+"/") {
			longest = max(longest, len(prefix))
		}
	}
	return longest
}
