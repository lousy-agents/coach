package evaldetect_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const layerViolation = "architecture.layer_violation"

func loadPolicy(root string) evaldetect.Policy {
	policy, err := evaldetect.LoadPolicy(filepath.Join(root, ".lousy-agents", "coach.json"))
	Expect(err).NotTo(HaveOccurred())
	return policy
}

var _ = Describe("the Go layer graph", func() {
	base := func(extra map[string]string) string {
		files := map[string]string{
			"go.mod":                   "module example.com/fx\n\ngo 1.26.0\n",
			".lousy-agents/coach.json": goLayerPolicy,
			"pkg/domain/domain.go":     "package domain\n\nfunc D() {}\n",
			"pkg/app/app.go":           "package app\n\nimport \"example.com/fx/pkg/domain\"\n\nfunc A() { domain.D() }\n",
			"tools/gen/gen.go":         "package gen\n\nimport \"example.com/fx/pkg/app\"\n\nfunc G() { app.A() }\n",
		}
		for path, content := range extra {
			files[path] = content
		}
		return writeTree(GinkgoT().TempDir(), files)
	}

	When("a base tree imports only along allowed edges", func() {
		It("reports nothing, including for importers outside every layer", func() {
			root := base(nil)

			findings, err := evaldetect.GoLayerViolations(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		})
	})

	When("a package imports across a forbidden edge", func() {
		It("reports exactly that edge, at the importing file and line", func() {
			root := base(map[string]string{
				"pkg/domain/leak.go": "package domain\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/fx/pkg/app\"\n)\n\nfunc Leak() { fmt.Println(); app.A() }\n",
			})

			findings, err := evaldetect.GoLayerViolations(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(evaldetect.Finding{
				RuleID:  layerViolation,
				Path:    "pkg/domain/leak.go",
				Subject: layerViolation + ":pkg/domain->pkg/app",
				Line:    6,
			}))
		})
	})

	When("one layer's prefix sits inside another's", func() {
		It("places each directory in the layer with the longest matching prefix", func() {
			root := writeTree(GinkgoT().TempDir(), map[string]string{
				"go.mod":                   "module example.com/fx\n\ngo 1.26.0\n",
				".lousy-agents/coach.json": `{"schema_version":"1","roots":["."],"layers":[{"name":"app","prefixes":["pkg/app"]},{"name":"core","prefixes":["pkg"]}],"forbidden_imports":[{"from":"core","to":"app"}]}`,
				"pkg/app/app.go":           "package app\n\nfunc A() {}\n",
				"pkg/domain/domain.go":     "package domain\n\nimport \"example.com/fx/pkg/app\"\n\nfunc D() { app.A() }\n",
			})

			findings, err := evaldetect.GoLayerViolations(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Subject", layerViolation+":pkg/domain->pkg/app")))
		})
	})

	When("the forbidden import is in a nested module", func() {
		It("resolves the nested module's path and reports the edge", func() {
			root := writeTree(GinkgoT().TempDir(), map[string]string{
				"go.mod":                   "module example.com/fx\n\ngo 1.26.0\n",
				"sub/go.mod":               "module example.com/fx/sub\n\ngo 1.26.0\n",
				".lousy-agents/coach.json": `{"schema_version":"1","roots":["."],"layers":[{"name":"low","prefixes":["sub/low"]},{"name":"high","prefixes":["sub/high"]}],"forbidden_imports":[{"from":"low","to":"high"}]}`,
				"sub/high/high.go":         "package high\n\nfunc H() {}\n",
				"sub/low/low.go":           "package low\n\nimport \"example.com/fx/sub/high\"\n\nfunc L() { high.H() }\n",
			})

			findings, err := evaldetect.GoLayerViolations(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Subject", layerViolation+":sub/low->sub/high")))
		})
	})
})
