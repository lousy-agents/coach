package evalpatterns

import (
	"sort"
	"strings"
)

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

func (policy checkoutPolicy) layers(packageDirs []string) layerShape {
	if len(packageDirs) == 0 {
		return layerShape{}
	}
	counts := map[string]int{}
	for _, dir := range packageDirs {
		if name := policy.layerOf(dir); name != "" {
			counts[name]++
		}
	}
	names := sortedKeys(counts)
	if len(names) == 0 {
		return layerShape{fromPolicy: true, unplaced: policy.unplacedReason()}
	}
	ranks := policy.ranks(names)
	sort.SliceStable(names, func(i, j int) bool { return ranks[names[i]] < ranks[names[j]] })
	shape := layerShape{fromPolicy: true, ordered: isPermutation(ranks) && !policy.forbidsBothWays(names)}
	for _, name := range names {
		shape.packages = append(shape.packages, counts[name])
	}
	return shape
}

func (policy checkoutPolicy) unplacedReason() string {
	if len(policy.Layers) == 0 {
		return policyDeclaresNoLayers
	}
	return policyLayersMatchNoPackage
}
