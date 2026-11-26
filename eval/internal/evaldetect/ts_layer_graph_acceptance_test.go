package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

var _ = Describe("the TypeScript layer graph", func() {
	tsBase := func(domainSource string) string {
		return writeTree(GinkgoT().TempDir(), map[string]string{
			".lousy-agents/coach.json": tsLayerPolicy,
			"tsconfig.json":            `{"compilerOptions":{"paths":{"@app/*":["src/app/*"]},"allowJs":false},"include":["src"]}`,
			"src/app/a.ts":             "import { D } from \"../domain/d\";\nexport const A = () => D();\n",
			"src/domain/d.ts":          domainSource,
		})
	}

	When("a base tree imports only along allowed edges", func() {
		It("reports nothing", func() {
			root := tsBase("export const D = () => 1;\n")

			findings, err := evaldetect.TSLayerViolations(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		})
	})

	DescribeTable("a domain file reaches the app layer",
		func(domainSource string, line int) {
			root := tsBase(domainSource)

			findings, err := evaldetect.TSLayerViolations(root, loadPolicy(root))
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
