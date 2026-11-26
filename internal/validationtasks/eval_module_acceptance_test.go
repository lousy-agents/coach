package validationtasks

import (
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const evalModulePath = "github.com/lousy-agents/coach/eval"

var evalRingPrefixes = map[string][]string{
	"tooling":    {"eval/cmd/coach-eval", "eval/internal/evalharness"},
	"generation": {"eval/internal/evalgen"},
	"detection":  {"eval/internal/evaldetect"},
	"grading":    {"eval/internal/evalgrade"},
	"real-code":  {"eval/internal/evalhistory", "eval/internal/evalcensus", "eval/internal/evalpatterns"},
}

func allEvalRingPrefixes() []string {
	var prefixes []string
	for _, ringPrefixes := range evalRingPrefixes {
		prefixes = append(prefixes, ringPrefixes...)
	}
	return prefixes
}

// rootModuleImports lists the import paths from go list output that belong to
// the root module rather than to the eval module.
func rootModuleImports(goListOutput string) []string {
	var imports []string
	for _, path := range strings.Fields(goListOutput) {
		if strings.HasPrefix(path, "github.com/lousy-agents/coach/") && !strings.HasPrefix(path, evalModulePath) {
			imports = append(imports, path)
		}
	}
	return imports
}

func evalModulePackageDirs(moduleRoot string) []string {
	var dirs []string
	for _, dir := range goPackageDirs(filepath.Join(moduleRoot, "eval")) {
		dirs = append(dirs, "eval/"+dir)
	}
	return dirs
}

var _ = Describe("the eval module", func() {
	moduleRoot := filepath.Join("..", "..")
	policyPath := filepath.Join(moduleRoot, ".lousy-agents", "coach.json")

	When("its packages are listed", func() {
		It("has the package tree the design names, each package in exactly one ring", func() {
			dirs := evalModulePackageDirs(moduleRoot)

			Expect(dirs).To(ConsistOf(allEvalRingPrefixes()))
			Expect(packagesNotInExactlyOneLayer(readLayerPolicy(policyPath), dirs)).To(BeEmpty())
		})
	})

	When("the repository policy is read", func() {
		It("places each eval package in the ring the design gives it", func() {
			policy := readLayerPolicy(policyPath)

			for ring, prefixes := range evalRingPrefixes {
				for _, prefix := range prefixes {
					Expect(policy.layerNamesFor(prefix)).To(Equal([]string{ring}), "%s belongs in the %s ring", prefix, ring)
				}
			}
		})
	})

	When("its dependencies are listed", func() {
		It("imports no package of the root module, so graders reach Coach only through the pinned binary", func() {
			command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./...")
			command.Dir = filepath.Join(moduleRoot, "eval")
			output, err := command.Output()
			Expect(err).NotTo(HaveOccurred(), "go list -deps must run in the eval module")

			Expect(rootModuleImports(string(output))).To(BeEmpty())
		})
	})
})
