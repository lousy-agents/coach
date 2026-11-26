package antipatternfixtures_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// generatorRoles is the generator's role list for each layer count, outermost first.
var generatorRoles = map[int][]string{
	3: {"api", "service", "store"},
	4: {"api", "service", "domain", "store"},
	5: {"api", "service", "domain", "repository", "store"},
}

var layerIDs = []string{"dependency-rule-breach", "skipped-layer"}

var variants = []string{"bad", "good"}

// layered is a profile that records the given layer-count distribution, the packages each role was seen with, and
// the shapes listed as not covered, and observed no complexity shape.
func layered(layers map[string]int, roles map[string]map[string]int, notCovered ...string) []byte {
	GinkgoHelper()
	var profile map[string]any
	Expect(json.Unmarshal(profileWith(map[string]int{}, map[string]int{}, map[string]int{}, 0), &profile)).To(Succeed())
	profile["dimensions"].(map[string]any)["layers"] = layers
	observed := map[string]any{}
	for role, packages := range roles {
		observed[role] = map[string]any{"packages": packages}
	}
	profile["roles"] = observed
	items := []any{}
	for _, shape := range notCovered {
		items = append(items, map[string]any{"shape": shape, "reason": "out_of_range", "count": 1})
	}
	profile["not_covered"] = items
	raw, err := json.Marshal(profile)
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// evenly gives every role of a layer count the same package distribution.
func evenly(layers int, packages map[string]int) map[string]map[string]int {
	roles := map[string]map[string]int{}
	for _, role := range generatorRoles[layers] {
		roles[role] = packages
	}
	return roles
}

type policyFile struct {
	Layers []struct {
		Name     string   `json:"name"`
		Prefixes []string `json:"prefixes"`
	} `json:"layers"`
	ForbiddenImports []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"forbidden_imports"`
	RequiredLayer string `json:"required_layer"`
}

func policyOf(out, id, variant string) policyFile {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join(out, id, variant, ".lousy-agents", "coach.json.txt"))
	Expect(err).NotTo(HaveOccurred(), "the policy is stored as coach.json.txt, as #871's layer fixtures store it")
	var policy policyFile
	Expect(json.Unmarshal(raw, &policy)).To(Succeed())
	return policy
}

// packageDirs are the directories under internal/<role>/ that hold a Go file, by role.
func packageDirs(out, id, variant string) map[string][]string {
	GinkgoHelper()
	dirs := map[string][]string{}
	for name := range tree(filepath.Join(out, id, variant)) {
		parts := strings.Split(name, "/")
		if isPackageFile(parts) {
			dirs[parts[1]] = append(dirs[parts[1]], parts[2])
		}
	}
	for role := range dirs {
		slices.Sort(dirs[role])
		dirs[role] = slices.Compact(dirs[role])
	}
	return dirs
}

func isPackageFile(parts []string) bool {
	return len(parts) == 4 && parts[0] == "internal" && strings.HasSuffix(parts[3], ".go.txt")
}
