package validationtasks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type layerRule struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

type layerPolicy struct {
	Layers []layerRule `json:"layers"`
}

// contains reports whether any prefix of the layer contains dir.
func (l layerRule) contains(dir string) bool {
	for _, prefix := range l.Prefixes {
		if domain.LayerPrefixContains(prefix, dir) {
			return true
		}
	}
	return false
}

// layerNamesFor lists the distinct layers whose prefixes contain dir.
func (p layerPolicy) layerNamesFor(dir string) []string {
	var names []string
	for _, layer := range p.Layers {
		if layer.contains(dir) {
			names = append(names, layer.Name)
		}
	}
	return names
}

// packagesNotInExactlyOneLayer describes every directory the policy leaves out
// or places in more than one layer.
func packagesNotInExactlyOneLayer(policy layerPolicy, dirs []string) []string {
	var problems []string
	for _, dir := range dirs {
		switch layers := policy.layerNamesFor(dir); len(layers) {
		case 1:
		case 0:
			problems = append(problems, dir+": in no layer")
		default:
			problems = append(problems, dir+": in "+strings.Join(layers, " and "))
		}
	}
	sort.Strings(problems)
	return problems
}

func readLayerPolicy(path string) layerPolicy {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	var policy layerPolicy
	Expect(json.Unmarshal(raw, &policy)).To(Succeed())
	return policy
}

// goPackageDirs is the repository-relative directory of every Go package in
// the module. -find names the packages without resolving their imports, so the
// listing needs neither the module cache nor the network.
func goPackageDirs(moduleRoot string) []string {
	command := exec.Command("go", "list", "-find", "-f", "{{.Dir}}", "./...")
	command.Dir = moduleRoot
	output, err := command.Output()
	Expect(err).NotTo(HaveOccurred(), "go list ./... must run from the module root")
	root, err := filepath.Abs(moduleRoot)
	Expect(err).NotTo(HaveOccurred())
	var dirs []string
	for _, absolute := range strings.Fields(string(output)) {
		relative, err := filepath.Rel(root, absolute)
		Expect(err).NotTo(HaveOccurred())
		dirs = append(dirs, filepath.ToSlash(relative))
	}
	Expect(dirs).NotTo(BeEmpty())
	return dirs
}

var _ = Describe("the layer policy this repository scans itself with", func() {
	moduleRoot := filepath.Join("..", "..")
	policyPath := filepath.Join(moduleRoot, ".lousy-agents", "coach.json")

	When("a package directory is missing from every layer or is in two", func() {
		It("is reported with the layers it is in", func() {
			var policy layerPolicy
			Expect(json.Unmarshal([]byte(`{"layers":[
				{"name":"entities","prefixes":["pkg/domain"]},
				{"name":"adapters","prefixes":["internal","pkg/domain/shared"]}]}`), &policy)).To(Succeed())

			problems := packagesNotInExactlyOneLayer(policy, []string{"pkg/domain", "pkg/domain/shared", "internal/x", "cmd/new-tool"})

			Expect(problems).To(Equal([]string{
				"cmd/new-tool: in no layer",
				"pkg/domain/shared: in entities and adapters",
			}))
		})
	})

	When("a Go package exists anywhere in the module", func() {
		It("places it in exactly one layer, so a new cmd/<tool> cannot land outside every layer rule unnoticed", func() {
			problems := packagesNotInExactlyOneLayer(readLayerPolicy(policyPath), goPackageDirs(moduleRoot))

			Expect(problems).To(BeEmpty(), "add each directory to a layer in .lousy-agents/coach.json (and project.json):\n%s", strings.Join(problems, "\n"))
		})
	})

	When("the deprecated project.json is still at the repository root", func() {
		It("is byte-equal to .lousy-agents/coach.json, because the two are maintained by hand", func() {
			deprecated, err := os.ReadFile(filepath.Join(moduleRoot, "project.json"))
			if os.IsNotExist(err) {
				Skip("project.json is gone, so there is no second copy to keep equal")
			}
			Expect(err).NotTo(HaveOccurred())
			current, err := os.ReadFile(policyPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(deprecated)).To(Equal(string(current)), "copy .lousy-agents/coach.json over project.json, or delete project.json")
		})
	})
})
