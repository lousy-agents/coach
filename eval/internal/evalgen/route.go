package evalgen

import (
	"slices"
	"sort"
)

// hop is one package on the feature route, from the entry package to the package that holds the entity.
type hop struct {
	layer, name string
	dir         string
}

// route picks depth+1 packages in strictly increasing (layer, package) order. It always passes through the
// second layer before any deeper one, so no step can skip the required layer or run against the layer order.
func route(layers []string, packages []string, depth int, s *stream) []hop {
	steps := layerSteps(len(layers), depth, s)
	used := map[int]int{}
	for _, layer := range steps {
		used[layer]++
	}
	chosen := map[int][]int{}
	for layer := 1; layer <= len(layers); layer++ {
		if used[layer] == 0 {
			continue
		}
		order := shuffled(s, indexes(len(packages)))[:used[layer]]
		sort.Ints(order)
		chosen[layer] = order
	}
	hops := make([]hop, 0, len(steps))
	for _, layer := range steps {
		name := packages[chosen[layer][0]]
		chosen[layer] = chosen[layer][1:]
		hops = append(hops, hop{layer: layers[layer-1], name: name, dir: "internal/" + layers[layer-1] + "/" + name})
	}
	return hops
}

func layerSteps(layerCount, depth int, s *stream) []int {
	slots := depth + 1
	steps := []int{}
	if slots >= layerCount {
		for layer := 1; layer <= layerCount; layer++ {
			steps = append(steps, layer)
		}
		for extra := 0; extra < slots-layerCount; extra++ {
			steps = append(steps, extra%layerCount+1)
		}
		slices.Sort(steps)
		return steps
	}
	deeper := shuffled(s, indexRange(3, layerCount-1))[:slots-3]
	steps = append([]int{1, 2}, deeper...)
	slices.Sort(steps)
	return append(steps, layerCount)
}

func indexes(n int) []int { return indexRange(0, n-1) }

func indexRange(low, high int) []int {
	out := make([]int, 0, high-low+1)
	for i := low; i <= high; i++ {
		out = append(out, i)
	}
	return out
}
