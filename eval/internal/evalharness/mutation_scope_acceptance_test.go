package evalharness_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

func scopedProgram(tree map[string]string) evalgen.Program {
	files := map[string][]byte{}
	for name, content := range tree {
		files[name] = []byte(content)
	}
	return evalgen.Program{
		Tree: files,
		Manifest: evalgen.Manifest{Template: evalgen.TemplateInfo{
			APISurface: []evalgen.APISymbol{{Symbol: "example.com/shop/internal/api/entry.Run", Entry: "internal/api/entry.Run"}},
		}},
	}
}

func apiSurfaceDirectories(generated evalgen.Program) []string {
	var directories []string
	for _, symbol := range generated.Manifest.Template.APISurface {
		qualified := strings.TrimPrefix(symbol.Symbol, moduleOf(generated)+"/")
		directories = append(directories, qualified[:strings.LastIndex(qualified, ".")])
	}
	return directories
}

const scopeModule = "module example.com/shop\n\ngo 1.22\n"

var _ = Describe("the packages the mutation gate judges (design §6, step 6: the import graph from api_surface)", func() {
	It("holds the entry's package and every package reachable from it by imports, however far (AC-24)", func() {
		program := scopedProgram(map[string]string{
			"go.mod":                          scopeModule,
			"internal/api/entry/entry.go":     "package entry\n\nimport \"example.com/shop/internal/service\"\n\nfunc Run() { service.Do() }\n",
			"internal/service/service.go":     "package service\n\nimport \"example.com/shop/internal/shared\"\n\nfunc Do() { shared.Help() }\n",
			"internal/shared/shared.go":       "package shared\n\nimport (\n\t\"fmt\"\n\t\"example.com/shop/internal/leaf\"\n)\n\nfunc Help() { fmt.Println(leaf.Name) }\n",
			"internal/leaf/leaf.go":           "package leaf\n\nconst Name = \"x\"\n",
			"internal/unrelated/unrelated.go": "package unrelated\n\nfunc Other() {}\n",
		})

		scope, err := evalharness.MutationScope(program)

		Expect(err).NotTo(HaveOccurred())
		Expect(scope).To(ConsistOf("internal/api/entry", "internal/service", "internal/shared", "internal/leaf"),
			"shared and leaf are imported only transitively and are in; unrelated is imported by nobody and is out")
	})

	It("does not follow an import that only a test file makes (AC-24)", func() {
		program := scopedProgram(map[string]string{
			"go.mod":                           scopeModule,
			"internal/api/entry/entry.go":      "package entry\n\nfunc Run() {}\n",
			"internal/api/entry/entry_test.go": "package entry\n\nimport _ \"example.com/shop/internal/unrelated\"\n",
			"internal/unrelated/unrelated.go":  "package unrelated\n",
		})

		scope, err := evalharness.MutationScope(program)

		Expect(err).NotTo(HaveOccurred())
		Expect(scope).To(ConsistOf("internal/api/entry"))
	})

	It("refuses a Go file it cannot read the imports of, instead of judging a smaller scope (AC-24)", func() {
		program := scopedProgram(map[string]string{
			"go.mod":                      scopeModule,
			"internal/api/entry/entry.go": "package entry\n\nimport (\n",
		})

		_, err := evalharness.MutationScope(program)

		Expect(err).To(HaveOccurred())
	})

	DescribeTable("is the api surface's own packages for each of four sampled generated cells, which import nothing beyond their route (AC-24)",
		func(cell int) {
			generated := program(cell)

			scope, err := evalharness.MutationScope(generated)

			Expect(err).NotTo(HaveOccurred())
			Expect(scope).To(ConsistOf(apiSurfaceDirectories(generated)))
		},
		Entry("the first cell", 0),
		Entry("a http handler cell", 1002),
		Entry("a write cell", 1500),
		Entry("the last cell", 1727),
	)
})
