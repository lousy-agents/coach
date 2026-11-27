package antipatternfixtures

import (
	"fmt"
	"strconv"
)

// layerRoles is the generator's role list for each layer count, outermost
// first: the vocabulary of its programs, never a reference repository's layer
// names.
var layerRoles = map[int][]string{
	3: {"api", "service", "store"},
	4: {"api", "service", "domain", "store"},
	5: {"api", "service", "domain", "repository", "store"},
}

const (
	minLayers = 3
	maxLayers = 5
	// maxPackagesPerRole is the packages layerPackages names: a layer holds at most one of each.
	maxPackagesPerRole = len(layerPackages)
)

// observeLayers reads the layer count the profile records in the generator's
// range and the packages its roles hold. A profile that lists layers as not
// covered gets neither layer fixture, whatever else it recorded.
func observeLayers(p profile) (observation, bool) {
	for _, item := range p.NotCovered {
		if item.Shape == "layers" {
			return observation{}, false
		}
	}
	layers, ok := layerCount(p.Dimensions.Layers)
	if !ok {
		return observation{}, false
	}
	packages, basis, ok := packagesPerLayer(p, layers)
	if !ok {
		return observation{}, false
	}
	return observation{measure: "layer count", value: layers, from: layers, threshold: minLayers, limit: maxLayers, packages: packages, basis: basis}, true
}

// layerCount is the most frequent count from 3 to 5, ties to the larger.
func layerCount(distribution map[string]int) (int, bool) {
	best, bestCount := 0, 0
	for layers := minLayers; layers <= maxLayers; layers++ {
		if count := distribution[strconv.Itoa(layers)]; count > 0 && count >= bestCount {
			best, bestCount = layers, count
		}
	}
	return best, best > 0
}

// packagesPerLayer is the most frequent package count the roles of the layer
// count observed, capped, with the account of it for the note. Roles are
// recorded only for layers an ordered policy places, so a profile with none
// has observed no ordered layers, and the layer count its package tree gave
// stands for nothing a fixture may model.
func packagesPerLayer(p profile, layers int) (int, string, bool) {
	merged := map[string]int{}
	for _, role := range layerRoles[layers] {
		for bucket, count := range p.Roles[role].Packages {
			merged[bucket] += count
		}
	}
	seen, ok := typical(merged, "packages per layer", 1, maxPackagesPerRole)
	if !ok {
		return 0, "", false
	}
	capped := ""
	if seen.from != seen.value {
		capped = fmt.Sprintf(", capped at %d from %d, so the fixture stays readable", seen.value, seen.from)
	}
	return seen.value, fmt.Sprintf("the most frequent layer count from 3 to 5, with packages per layer %d, the most frequent count its roles observed%s", seen.value, capped), true
}
