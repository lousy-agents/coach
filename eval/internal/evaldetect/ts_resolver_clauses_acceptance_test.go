package evaldetect_test

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const aliasPaths = `"paths":{"@app/*":["src/app/*"]}`

// extendsChain writes a root tsconfig.json that reaches n further configs
// through relative extends, the last of which declares the @app alias.
func extendsChain(n int) map[string]string {
	files := map[string]string{"src/domain/d.ts": "import { A } from \"@app/a\";\n"}
	previous := "tsconfig.json"
	for i := 1; i <= n; i++ {
		name := "c" + strconv.Itoa(i) + ".json"
		files[previous] = `{"extends":"./` + name + `"}`
		previous = name
	}
	files[previous] = `{"compilerOptions":{"baseUrl":".",` + aliasPaths + `}}`
	return files
}

var _ = Describe("the TypeScript resolver's clauses", func() {
	DescribeTable("a specifier form that resolves into the app layer",
		func(files map[string]string) {
			findings, err := tsViolations(tsTree(files))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Subject", layerViolation+":src/domain->src/app")))
		},
		Entry("a .mjs specifier naming a .mts source", map[string]string{
			"src/app/m.mts": "export const M = 1;\n", "src/domain/d.ts": "import { M } from \"../app/m.mjs\";\n",
		}),
		Entry("a .cjs specifier naming a .cts source", map[string]string{
			"src/app/c.cts": "export const C = 1;\n", "src/domain/d.ts": "import { C } from \"../app/c.cjs\";\n",
		}),
		Entry("a declaration file", map[string]string{
			"src/app/types.d.ts": "export type T = 1;\n", "src/domain/d.ts": "import type { T } from \"../app/types\";\n",
		}),
		Entry("a directory resolved to its index.d.ts", map[string]string{
			"src/app/index.d.ts": "export type T = 1;\n", "src/domain/d.ts": "import type { T } from \"../app\";\n",
		}),
		Entry("a specifier that names the file with its extension", map[string]string{
			"src/domain/d.ts": "import { A } from \"../app/a.ts\";\n",
		}),
		Entry("an exact paths key", map[string]string{
			"tsconfig.json":   `{"compilerOptions":{"baseUrl":".","paths":{"@app":["src/app/a"]}}}`,
			"src/domain/d.ts": "import { A } from \"@app\";\n",
		}),
		Entry("a paths pattern with a suffix after its wildcard", map[string]string{
			"tsconfig.json":   `{"compilerOptions":{"baseUrl":".","paths":{"@app/*.js":["src/app/*.ts"]}}}`,
			"src/domain/d.ts": "import { A } from \"@app/a.js\";\n",
		}),
		Entry("alongside a specifier shorter than a pattern's prefix and suffix", map[string]string{
			"tsconfig.json":   `{"compilerOptions":{"baseUrl":".","paths":{"@a*a":["src/app/a"]}}}`,
			"src/domain/d.ts": "import \"@a\";\nimport { A } from \"../app/a\";\n",
		}),
		Entry("paths targets taken relative to baseUrl", map[string]string{
			"tsconfig.json":   `{"compilerOptions":{"baseUrl":"src","paths":{"@app/*":["app/*"]}}}`,
			"src/domain/d.ts": "import { A } from \"@app/a\";\n",
		}),
		Entry("a paths pattern whose first target is missing", map[string]string{
			"tsconfig.json":   `{"compilerOptions":{"baseUrl":".","paths":{"@app/*":["missing/*","src/app/*"]}}}`,
			"src/domain/d.ts": "import { A } from \"@app/a\";\n",
		}),
		Entry("a baseUrl relative to the config that declares it", map[string]string{
			"config/base.json": `{"compilerOptions":{"baseUrl":".."}}`,
			"tsconfig.json":    `{"extends":"./config/base.json"}`,
			"src/domain/d.ts":  "import { A } from \"src/app/a\";\n",
		}),
		Entry("paths without baseUrl, relative to the config that declares them", map[string]string{
			"config/base.json": `{"compilerOptions":{"paths":{"@app/*":["../src/app/*"]}}}`,
			"tsconfig.json":    `{"extends":"./config/base.json"}`,
			"src/domain/d.ts":  "import { A } from \"@app/a\";\n",
		}),
		Entry("an extends without its .json suffix", map[string]string{
			"tsconfig.base.json": `{"compilerOptions":{"baseUrl":".",` + aliasPaths + `}}`,
			"tsconfig.json":      `{"extends":"./tsconfig.base"}`,
			"src/domain/d.ts":    "import { A } from \"@app/a\";\n",
		}),
		Entry("an extends chain at the depth cap", extendsChain(16)),
		Entry("a tsconfig with a line comment", map[string]string{
			"tsconfig.json":   "{\n// the fixture\n\"compilerOptions\":{\"baseUrl\":\".\"," + aliasPaths + "}}",
			"src/domain/d.ts": "import { A } from \"@app/a\";\n",
		}),
		Entry("a tsconfig with a block comment", map[string]string{
			"tsconfig.json":   "{/* the fixture */\"compilerOptions\":{\"baseUrl\":\".\"," + aliasPaths + "}}",
			"src/domain/d.ts": "import { A } from \"@app/a\";\n",
		}),
		Entry("a tsconfig with a trailing comma in an object", map[string]string{
			"tsconfig.json":   "{\"compilerOptions\":{\"baseUrl\":\".\"," + aliasPaths + ",},}",
			"src/domain/d.ts": "import { A } from \"@app/a\";\n",
		}),
		Entry("a tsconfig with a trailing comma in an array", map[string]string{
			"tsconfig.json":   "{\"compilerOptions\":{\"baseUrl\":\".\",\"paths\":{\"@app/*\":[\"src/app/*\",]}}}",
			"src/domain/d.ts": "import { A } from \"@app/a\";\n",
		}),
		Entry("a tsconfig with a comment between a trailing comma and its closer", map[string]string{
			"tsconfig.json":   "{\"compilerOptions\":{\"baseUrl\":\".\"," + aliasPaths + ", // the alias\n}}",
			"src/domain/d.ts": "import { A } from \"@app/a\";\n",
		}),
	)

	When("the root has no tsconfig.json", func() {
		It("still resolves relative specifiers", func() {
			root := writeTree(GinkgoT().TempDir(), map[string]string{
				".lousy-agents/coach.json": tsLayerPolicy,
				"src/app/a.ts":             "export const A = 1;\n",
				"src/domain/d.ts":          "import { A } from \"../app/a\";\n",
			})

			findings, err := tsViolations(root)
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Subject", layerViolation+":src/domain->src/app")))
		})
	})

	When("a file imports its parent directory as ..", func() {
		It("resolves the parent's index file", func() {
			root := writeTree(GinkgoT().TempDir(), map[string]string{
				".lousy-agents/coach.json": `{"schema_version":"1","roots":["."],"layers":[{"name":"inner","prefixes":["src/core/inner"]},{"name":"outer","prefixes":["src/core"]}],"forbidden_imports":[{"from":"inner","to":"outer"}]}`,
				"src/core/index.ts":        "export const O = 1;\n",
				"src/core/inner/i.ts":      "import { O } from \"..\";\n",
			})

			findings, err := tsViolations(root)
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(HaveField("Subject", layerViolation+":src/core/inner->src/core")))
		})
	})

	When("a specifier matches a paths pattern's prefix but not its suffix", func() {
		It("is not resolved through that pattern", func() {
			findings, err := tsViolations(tsTree(map[string]string{
				"tsconfig.json":   `{"compilerOptions":{"baseUrl":".","paths":{"@app/*.js":["src/app/*.ts"]}}}`,
				"src/domain/d.ts": "import { A } from \"@app/a.ts\";\n",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		})
	})

	When("a bare specifier only starts with the package's own name", func() {
		It("is not refused as a self-reference", func() {
			findings, err := tsViolations(tsTree(map[string]string{
				"package.json":    `{"name":"fx"}`,
				"src/domain/d.ts": "import { X } from \"fxother/a\";\n",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		})
	})

	When("a package.json cannot be parsed", func() {
		It("fails rather than skipping the package's name", func() {
			_, err := tsViolations(tsTree(map[string]string{"package.json": "{"}))

			Expect(err).To(MatchError(ContainSubstring("parse")))
		})
	})

	DescribeTable("a config form the resolver does not model is refused by name",
		func(files map[string]string, reason string) {
			_, err := tsViolations(tsTree(files))
			Expect(err).To(MatchError(evaldetect.ErrRefused))
			Expect(err).To(MatchError(ContainSubstring(reason)))
		},
		Entry("an extends from a package", map[string]string{"tsconfig.json": `{"extends":"@tsconfig/node20/tsconfig.json"}`}, "not one relative path"),
		Entry("an extends array", map[string]string{"tsconfig.json": `{"extends":["./a.json","./b.json"]}`}, "not one relative path"),
		Entry("an extends cycle", map[string]string{"tsconfig.json": `{"extends":"./tsconfig.json"}`}, "deeper than"),
		Entry("an extends chain one past the depth cap", extendsChain(17), "deeper than"),
	)
})
