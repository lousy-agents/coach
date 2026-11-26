package evaldetect_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const leakToApp = "package leak\n\nimport \"example.com/fx/pkg/app\"\n\nvar _ = app.A\n"

// goLayerTree writes the Go base tree under root, overlaid with files.
func goLayerTree(root string, files map[string]string) string {
	tree := map[string]string{
		"go.mod":                   "module example.com/fx\n\ngo 1.26.0\n",
		".lousy-agents/coach.json": goLayerPolicy,
		"pkg/app/app.go":           "package app\n\nfunc A() {}\n",
	}
	for path, content := range files {
		tree[path] = content
	}
	return writeTree(root, tree)
}

func goViolations(root string) []evaldetect.Finding {
	findings, err := evaldetect.GoLayerViolations(root, loadPolicy(root))
	Expect(err).NotTo(HaveOccurred())
	return findings
}

var _ = Describe("the Go layer graph's boundaries", func() {
	When("a directory's name only starts with a layer prefix", func() {
		It("places it outside that layer", func() {
			root := writeTree(GinkgoT().TempDir(), map[string]string{
				"go.mod":                   "module example.com/fx\n\ngo 1.26.0\n",
				".lousy-agents/coach.json": `{"schema_version":"1","roots":["."],"layers":[{"name":"db","prefixes":["pkg/db"]},{"name":"app","prefixes":["pkg/app"]}],"forbidden_imports":[{"from":"db","to":"app"}]}`,
				"pkg/app/app.go":           "package app\n\nfunc A() {}\n",
				"pkg/dbx/x.go":             "package dbx\n\nimport \"example.com/fx/pkg/app\"\n\nvar _ = app.A\n",
			})

			Expect(goViolations(root)).To(BeEmpty())
		})
	})

	When("an import's module path only starts with a module under the root", func() {
		It("does not resolve it into that module", func() {
			root := writeTree(GinkgoT().TempDir(), map[string]string{
				"go.mod":                   "module example.com/fx\n\ngo 1.26.0\n",
				".lousy-agents/coach.json": `{"schema_version":"1","roots":["."],"layers":[{"name":"low","prefixes":["pkg/low"]},{"name":"high","prefixes":["other/pkg/high"]}],"forbidden_imports":[{"from":"low","to":"high"}]}`,
				"other/pkg/high/h.go":      "package high\n\nfunc H() {}\n",
				"pkg/low/l.go":             "package low\n\nimport \"example.com/fxother/pkg/high\"\n\nvar _ = high.H\n",
			})

			Expect(goViolations(root)).To(BeEmpty())
		})
	})

	DescribeTable("a forbidden import in a tree no grader reads is not reported",
		func(dir string) {
			root := goLayerTree(GinkgoT().TempDir(), map[string]string{"pkg/domain/" + dir + "/leak.go": leakToApp})

			Expect(goViolations(root)).To(BeEmpty())
		},
		Entry("vendor", "vendor"),
		Entry("testdata", "testdata"),
		Entry("node_modules", "node_modules"),
		Entry("a hidden directory", ".cache"),
	)

	When("the root directory's own name starts with a dot", func() {
		It("still walks the root", func() {
			root := goLayerTree(filepath.Join(GinkgoT().TempDir(), ".fx"), map[string]string{"pkg/domain/leak.go": leakToApp})

			Expect(goViolations(root)).To(ConsistOf(HaveField("Subject", layerViolation+":pkg/domain->pkg/app")))
		})
	})

	When("the go.mod module line carries a trailing comment", func() {
		It("still resolves imports into that module", func() {
			root := goLayerTree(GinkgoT().TempDir(), map[string]string{
				"go.mod":             "module example.com/fx // the fixture module\n\ngo 1.26.0\n",
				"pkg/domain/leak.go": leakToApp,
			})

			Expect(goViolations(root)).To(ConsistOf(HaveField("Subject", layerViolation+":pkg/domain->pkg/app")))
		})
	})
})
