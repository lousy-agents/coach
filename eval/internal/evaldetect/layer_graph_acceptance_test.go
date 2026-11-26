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
			"go.mod":                    "module example.com/fx\n\ngo 1.26.0\n",
			".lousy-agents/coach.json":  goLayerPolicy,
			"pkg/domain/domain.go":      "package domain\n\nfunc D() {}\n",
			"pkg/app/app.go":            "package app\n\nimport \"example.com/fx/pkg/domain\"\n\nfunc A() { domain.D() }\n",
			"tools/gen/gen.go":          "package gen\n\nimport \"example.com/fx/pkg/app\"\n\nfunc G() { app.A() }\n",
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

var _ = Describe("the TypeScript layer graph", func() {
	tsBase := func(domainSource string) string {
		return writeTree(GinkgoT().TempDir(), map[string]string{
			".lousy-agents/coach.json": tsLayerPolicy,
			"tsconfig.json":            `{"compilerOptions":{"baseUrl":".","paths":{"@app/*":["src/app/*"]},"allowJs":false},"include":["src"]}`,
			"src/app/a.ts":             "import { D } from \"../domain/d\";\nexport const A = () => D();\n",
			"src/domain/d.ts":          domainSource,
		})
	}

	When("a base tree imports only along allowed edges", func() {
		It("reports nothing", func() {
			root := tsBase("export const D = () => 1;\n")

			findings, err := evaldetect.TSLayerViolations(root, loadPolicy(root), typescriptDir())
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		})
	})

	DescribeTable("a domain file reaches the app layer",
		func(domainSource string, line int) {
			root := tsBase(domainSource)

			findings, err := evaldetect.TSLayerViolations(root, loadPolicy(root), typescriptDir())
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(evaldetect.Finding{
				RuleID:  layerViolation,
				Path:    "src/domain/d.ts",
				Subject: layerViolation + ":src/domain->src/app",
				Line:    line,
			}))
		},
		Entry("by a value import", "export const D = () => 1;\nimport { A } from \"../app/a\";\nexport const use = A;\n", 2),
		Entry("by a type-only import", "export const D = () => 1;\nimport type { A } from \"../app/a\";\nexport type T = typeof A;\n", 2),
		Entry("through a tsconfig paths alias", "export const D = () => 1;\nimport { A } from \"@app/a\";\nexport const use = A;\n", 2),
		Entry("by a dynamic import()", "export const D = () => 1;\nexport const lazy = () => import(\"../app/a\");\n", 2),
		Entry("by a require() call", "declare const require: (id: string) => unknown;\nexport const D = () => 1;\nexport const app = require(\"../app/a\");\n", 3),
	)
})
