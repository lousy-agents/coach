package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const tsConfig = `{"compilerOptions":{"baseUrl":".","paths":{"@app/*":["src/app/*"]}},"include":["src"]}`

// tsTree writes a TS project whose app layer imports nothing from the domain
// layer, overlaid with files.
func tsTree(files map[string]string) string {
	tree := map[string]string{
		".lousy-agents/coach.json": tsLayerPolicy,
		"tsconfig.json":            tsConfig,
		"src/app/a.ts":             "export const A = () => 1;\n",
	}
	for path, content := range files {
		tree[path] = content
	}
	return writeTree(GinkgoT().TempDir(), tree)
}

func tsViolations(root string) ([]evaldetect.Finding, error) {
	return evaldetect.TSLayerViolations(root, loadPolicy(root))
}

var _ = Describe("the TypeScript import lexer", func() {
	DescribeTable("a specifier that is not an import reports nothing",
		func(domainSource string) {
			findings, err := tsViolations(tsTree(map[string]string{"src/domain/d.ts": domainSource}))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		},
		Entry("in a line comment", "// import { A } from \"../app/a\";\nexport const D = 1;\n"),
		Entry("in a block comment", "/* require(\"../app/a\") */\nexport const D = 1;\n"),
		Entry("in a string literal", "export const D = \"import('../app/a')\";\n"),
		Entry("in a template literal", "export const D = `import { A } from \"../app/a\"`;\n"),
		Entry("after a regular expression literal that holds a quote", "export const re = /\"/; const s = \"import('../app/a')\";\n"),
	)

	DescribeTable("an import the lexer must still see is reported",
		func(domainSource string, line int) {
			findings, err := tsViolations(tsTree(map[string]string{"src/domain/d.ts": domainSource}))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Line", line)))
		},
		Entry("inside a template substitution", "export const D = `${import(\"../app/a\")}`;\n", 1),
		Entry("as a side-effect import", "import \"../app/a\";\n", 1),
		Entry("as an export-from re-export", "export const D = 1;\nexport { A } from \"../app/a\";\n", 2),
		Entry("after a division that is not a regular expression", "export const D = 4 / 2; import { A } from \"../app/a\";\n", 1),
	)
})

var _ = Describe("the TypeScript import resolver", func() {
	DescribeTable("a supported specifier form resolves into the app layer",
		func(files map[string]string) {
			findings, err := tsViolations(tsTree(files))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Subject", layerViolation+":src/domain->src/app")))
		},
		Entry("a directory resolved to its index file", map[string]string{
			"src/app/index.ts": "export const I = 1;\n",
			"src/domain/d.ts":  "import { I } from \"../app\";\n",
		}),
		Entry("a .js specifier that names a .ts source", map[string]string{
			"src/domain/d.ts": "import { A } from \"../app/a.js\";\n",
		}),
		Entry("a paths alias inherited through extends", map[string]string{
			"tsconfig.base.json": `{"compilerOptions":{"baseUrl":".","paths":{"@base/*":["src/app/*"]}}}`,
			"tsconfig.json":      `{"extends":"./tsconfig.base.json","include":["src"]}`,
			"src/domain/d.ts":    "import { A } from \"@base/a\";\n",
		}),
		Entry("a baseUrl-relative specifier", map[string]string{
			"src/domain/d.ts": "import { A } from \"src/app/a\";\n",
		}),
	)

	DescribeTable("an unsupported form is refused with a named reason",
		func(files map[string]string, reason string) {
			_, err := tsViolations(tsTree(files))
			Expect(err).To(MatchError(evaldetect.ErrRefused))
			Expect(err).To(MatchError(ContainSubstring(reason)))
		},
		Entry("project references", map[string]string{
			"tsconfig.json": `{"compilerOptions":{"composite":true},"references":[{"path":"./src/app"}]}`,
		}, "project references"),
		Entry("a package.json imports specifier", map[string]string{
			"package.json":    `{"name":"fx","imports":{"#app/*":"./src/app/*.ts"}}`,
			"src/domain/d.ts": "import { A } from \"#app/a\";\n",
		}, "package.json imports"),
		Entry("a package.json exports self-reference", map[string]string{
			"package.json":    `{"name":"fx","exports":{"./app/*":"./src/app/*.ts"}}`,
			"src/domain/d.ts": "import { A } from \"fx/app/a\";\n",
		}, "package.json self-reference"),
		Entry("a paths pattern with more than one wildcard", map[string]string{
			"tsconfig.json": `{"compilerOptions":{"baseUrl":".","paths":{"@x/*/y/*":["src/*/z/*"]}}}`,
		}, "more than one wildcard"),
		Entry("a non-literal import()", map[string]string{
			"src/domain/d.ts": "const name = \"../app/a\";\nexport const lazy = () => import(name);\n",
		}, "non-literal import()"),
		Entry("a non-literal require()", map[string]string{
			"src/domain/d.ts": "declare const require: (id: string) => unknown;\nconst name = \"../app/a\";\nexport const app = require(name);\n",
		}, "non-literal require()"),
		Entry("an import() of a template with a substitution", map[string]string{
			"src/domain/d.ts": "const n = \"a\";\nexport const lazy = () => import(`../app/${n}`);\n",
		}, "non-literal import()"),
	)
})
