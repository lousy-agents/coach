package evalpatterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	raw, err := readRegularFile(root, policyFile)
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

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
