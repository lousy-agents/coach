package evalpatterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const policyFile = ".lousy-agents/coach.json"

// generatorRoles is the generator's role list for each layer count it
// builds, outermost first. It mirrors evalgen's vocabulary, which this ring
// may not import.
var generatorRoles = map[int][]string{
	3: {"api", "service", "store"},
	4: {"api", "service", "domain", "store"},
	5: {"api", "service", "domain", "repository", "store"},
}

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

// layerShape is how many packages each layer holds, outermost first when
// the policy orders the layers. A policy that names layers but leaves them
// without a total order is unordered.
type layerShape struct {
	packages   []int
	fromPolicy bool
	ordered    bool
}

// layersOf reads the checkout's layers: from its policy when it has one,
// counting only layers that hold a package, and otherwise one layer per
// top-level package directory.
func layersOf(root string, packageDirs []string) (layerShape, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(policyFile)))
	if errors.Is(err, fs.ErrNotExist) {
		return treeLayers(packageDirs), nil
	}
	if err != nil {
		return layerShape{}, err
	}
	var policy checkoutPolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return layerShape{}, fmt.Errorf("%s is not a valid policy: %v", policyFile, err)
	}
	return policy.layers(packageDirs), nil
}

func treeLayers(packageDirs []string) layerShape {
	counts := map[string]int{}
	for _, dir := range packageDirs {
		counts[strings.Split(dir, "/")[0]]++
	}
	var shape layerShape
	for _, name := range sortedKeys(counts) {
		shape.packages = append(shape.packages, counts[name])
	}
	return shape
}

func (policy checkoutPolicy) layers(packageDirs []string) layerShape {
	counts := map[string]int{}
	for _, dir := range packageDirs {
		if name := policy.layerOf(dir); name != "" {
			counts[name]++
		}
	}
	names := sortedKeys(counts)
	ranks := policy.ranks(names)
	sort.SliceStable(names, func(i, j int) bool { return ranks[names[i]] < ranks[names[j]] })
	shape := layerShape{fromPolicy: true, ordered: isPermutation(ranks)}
	for _, name := range names {
		shape.packages = append(shape.packages, counts[name])
	}
	return shape
}

// layerOf is the layer whose longest prefix holds dir, or "".
func (policy checkoutPolicy) layerOf(dir string) string {
	layer, longest := "", -1
	for _, candidate := range policy.Layers {
		for _, prefix := range candidate.Prefixes {
			if (dir == prefix || strings.HasPrefix(dir, prefix+"/")) && len(prefix) > longest {
				layer, longest = candidate.Name, len(prefix)
			}
		}
	}
	return layer
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

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
