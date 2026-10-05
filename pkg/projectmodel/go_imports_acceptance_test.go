package projectmodel_test

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func testMeta() projectmodel.SnapshotMeta {
	return projectmodel.SnapshotMeta{
		Revision:      "revision-sha",
		TreeID:        "tree-sha",
		ConfigDigest:  "config-digest",
		BackendDigest: "backend-digest",
	}
}

func edgeByTo(edges []projectmodel.ImportEdge, to string) (projectmodel.ImportEdge, bool) {
	for _, e := range edges {
		if e.To == to {
			return e, true
		}
	}
	return projectmodel.ImportEdge{}, false
}

var _ = Describe("BuildGoModel", func() {
	When("built from the go_multiworkspace fixture", func() {
		It("classifies every import edge kind and freezes From/To/Site conventions", func() {
			snapshot := os.DirFS("testdata/go_multiworkspace")
			model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(model.Workspaces).To(HaveLen(1))
			Expect(model.Workspaces[0].ID).To(Equal("workspace:."))
			Expect(model.Workspaces[0].Root).To(Equal("."))
			Expect(model.Workspaces[0].Projects).To(ConsistOf("module:modulea", "module:moduleb"))

			Expect(model.Modules).To(ConsistOf(
				projectmodel.Module{ID: "module:modulea", Path: "modulea", Language: "go", Files: []string{"modulea/pkg/a.go"}},
				projectmodel.Module{ID: "module:moduleb", Path: "moduleb", Language: "go", Files: []string{"moduleb/greet/greet.go"}},
			))

			Expect(model.Packages).To(ConsistOf(
				projectmodel.Package{ID: "package:modulea/pkg", Path: "modulea/pkg", Language: "go", Files: []string{"modulea/pkg/a.go"}},
				projectmodel.Package{ID: "package:moduleb/greet", Path: "moduleb/greet", Language: "go", Files: []string{"moduleb/greet/greet.go"}},
			))

			Expect(model.Files).To(ConsistOf(
				projectmodel.File{ID: "file:modulea/pkg/a.go", Path: "modulea/pkg/a.go", Language: "go"},
				projectmodel.File{ID: "file:moduleb/greet/greet.go", Path: "moduleb/greet/greet.go", Language: "go"},
			))

			Expect(model.ImportEdges).To(HaveLen(6))

			internal, ok := edgeByTo(model.ImportEdges, "package:moduleb/greet")
			Expect(ok).To(BeTrue())
			Expect(internal).To(Equal(projectmodel.ImportEdge{
				From: "package:modulea/pkg", To: "package:moduleb/greet", Kind: "internal", Site: "modulea/pkg/a.go:6",
			}))

			stdlib, ok := edgeByTo(model.ImportEdges, "fmt")
			Expect(ok).To(BeTrue())
			Expect(stdlib).To(Equal(projectmodel.ImportEdge{
				From: "package:modulea/pkg", To: "fmt", Kind: "stdlib", Site: "modulea/pkg/a.go:4",
			}))

			excluded, ok := edgeByTo(model.ImportEdges, "github.com/excluded/pkg")
			Expect(ok).To(BeTrue())
			Expect(excluded).To(Equal(projectmodel.ImportEdge{
				From: "package:modulea/pkg", To: "github.com/excluded/pkg", Kind: "excluded", Site: "modulea/pkg/a.go:7",
			}))

			external, ok := edgeByTo(model.ImportEdges, "github.com/external/pkg")
			Expect(ok).To(BeTrue())
			Expect(external).To(Equal(projectmodel.ImportEdge{
				From: "package:modulea/pkg", To: "github.com/external/pkg", Kind: "external", Site: "modulea/pkg/a.go:8",
			}))

			replaced, ok := edgeByTo(model.ImportEdges, "github.com/replaced/pkg")
			Expect(ok).To(BeTrue())
			Expect(replaced).To(Equal(projectmodel.ImportEdge{
				From: "package:modulea/pkg", To: "github.com/replaced/pkg", Kind: "replaced", Site: "modulea/pkg/a.go:9",
			}))

			unresolved, ok := edgeByTo(model.ImportEdges, "github.com/unresolved/pkg")
			Expect(ok).To(BeTrue())
			Expect(unresolved).To(Equal(projectmodel.ImportEdge{
				From: "package:modulea/pkg", To: "github.com/unresolved/pkg", Kind: "unresolved", Site: "modulea/pkg/a.go:10",
			}))
		})

		It("pins the Model.Coverage counts/budgets vocabulary", func() {
			body_goImportsAcceptanceTest_pinsTheModelCoverageCountsBudgetsVocabulary_99()
		})
	})

	When("a source file has syntax errors", func() {
		It("keeps the file's identity and records a coverage diagnostic instead of failing the build", func() {
			body_goImportsAcceptanceTest_keepsTheFileSIdentityAndRecordsACoverageDiagnost_118()
		})
	})

	When("a budget bounds the source-file read/analyze phase", func() {
		It("truncates deterministically, marks Complete false, and records files_skipped", func() {
			body_goImportsAcceptanceTest_truncatesDeterministicallyMarksCompleteFalseAndR_149()
		})
	})

	When("a module has .go files under testdata/, vendor/, and a dot-prefixed subdirectory", func() {
		It("excludes those files from Model.Files and Model.Packages, matching the go tool's own convention", func() {
			snapshot := os.DirFS("testdata/go_module_excluded_dirs")
			model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(model.Files).To(ConsistOf(
				projectmodel.File{ID: "file:main.go", Path: "main.go", Language: "go"},
			))
			Expect(model.Modules).To(ConsistOf(
				projectmodel.Module{ID: "module:.", Path: ".", Language: "go", Files: []string{"main.go"}},
			))
			Expect(model.Packages).To(ConsistOf(
				projectmodel.Package{ID: "package:.", Path: ".", Language: "go", Files: []string{"main.go"}},
			))
		})
	})

	When("GoBuildOptions.Roots is the repository-root path \".\" on a multi-module workspace", func() {
		It("includes every module under the snapshot, not an empty hollow workspace", func() {
			snapshot := os.DirFS("testdata/go_multiworkspace")
			model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{
				Roots: []string{"."},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(model.Modules).To(ConsistOf(
				projectmodel.Module{ID: "module:modulea", Path: "modulea", Language: "go", Files: []string{"modulea/pkg/a.go"}},
				projectmodel.Module{ID: "module:moduleb", Path: "moduleb", Language: "go", Files: []string{"moduleb/greet/greet.go"}},
			))
			Expect(model.Workspaces).To(HaveLen(1))
			Expect(model.Workspaces[0].Projects).To(ConsistOf("module:modulea", "module:moduleb"))

			_, ok := edgeByTo(model.ImportEdges, "package:moduleb/greet")
			Expect(ok).To(BeTrue(), "expected the internal cross-module edge to survive Roots:[\".\"], got %+v", model.ImportEdges)
		})
	})

	When("a workspace's modules declare dotless module paths (e.g. from `go mod init myapp`)", func() {
		It("classifies a same-module import as internal, not stdlib, alongside every other import kind", func() {
			snapshot := os.DirFS("testdata/go_dotless_module")
			model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())

			internal, ok := edgeByTo(model.ImportEdges, "package:moduleb/sub")
			Expect(ok).To(BeTrue(), "expected an edge resolving to package:moduleb/sub, got %+v", model.ImportEdges)
			Expect(internal.Kind).To(Equal("internal"),
				"a dotless module path (\"moduleab/greet\") must not be misclassified as stdlib merely because its first path segment has no dot")

			stdlib, ok := edgeByTo(model.ImportEdges, "fmt")
			Expect(ok).To(BeTrue())
			Expect(stdlib.Kind).To(Equal("stdlib"),
				"a genuine stdlib import matching no declared module path must still classify as stdlib")

			excluded, ok := edgeByTo(model.ImportEdges, "github.com/excluded/pkg")
			Expect(ok).To(BeTrue())
			Expect(excluded.Kind).To(Equal("excluded"))

			external, ok := edgeByTo(model.ImportEdges, "github.com/external/pkg")
			Expect(ok).To(BeTrue())
			Expect(external.Kind).To(Equal("external"))

			replaced, ok := edgeByTo(model.ImportEdges, "github.com/replaced/pkg")
			Expect(ok).To(BeTrue())
			Expect(replaced.Kind).To(Equal("replaced"))

			unresolved, ok := edgeByTo(model.ImportEdges, "github.com/unresolved/pkg")
			Expect(ok).To(BeTrue())
			Expect(unresolved.Kind).To(Equal("unresolved"))
		})

		It("resolves an import to the longest matching module path when more than one declared module genuinely matches", func() {
			snapshot := os.DirFS("testdata/go_dotless_module")
			model, err := projectmodel.BuildGoModel(snapshot, testMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())

			// Both "moduleab" (8 chars, package dir moduleb/greet) and
			// "moduleab/greet" (14 chars, package dir moduleb/sub) are
			// declared module paths that genuinely match the import path
			// "moduleab/greet" -- the first by a slash-bounded prefix, the
			// second by exact equality. Both candidate package directories
			// exist, so only the len(modPath) tiebreak in classifyGoImport
			// decides between them.
			edge, ok := edgeByTo(model.ImportEdges, "package:moduleb/sub")
			Expect(ok).To(BeTrue(), "expected the import to resolve against the longer module path \"moduleab/greet\", got %+v", model.ImportEdges)
			Expect(edge.Kind).To(Equal("internal"))
			Expect(edge.From).To(Equal("package:modulea/pkg"))

			_, shorterMatched := edgeByTo(model.ImportEdges, "package:moduleb/greet")
			Expect(shorterMatched).To(BeFalse(),
				"the shorter module path \"moduleab\" must lose the longest-match tiebreak to \"moduleab/greet\", not also resolve the import")
		})
	})

	When("two modules declare the same module path and an import could resolve to either", func() {
		It("always resolves the import to the same, sorted-first module across repeated builds", func() {
			body_goImportsAcceptanceTest_alwaysResolvesTheImportToTheSameSortedFirstModul_288()
		})
	})

	When("SnapshotMeta carries a repository identity and GoBuildOptions selects specific roots", func() {
		It("populates Model.Repository and Snapshot.SelectedRoots from the caller's inputs", func() {
			snapshot := os.DirFS("testdata/go_multiworkspace")
			meta := testMeta()
			meta.Repository = "github.com/lousy-agents/coach"

			model, err := projectmodel.BuildGoModel(snapshot, meta, projectmodel.GoBuildOptions{
				Roots: []string{"moduleb", "modulea"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Repository).To(Equal("github.com/lousy-agents/coach"))
			Expect(model.Snapshot.SelectedRoots).To(Equal([]string{"modulea", "moduleb"}))
		})
	})

	When("the same snapshot is mounted from two different absolute temporary roots", func() {
		It("produces byte-identical canonical Model JSON", func() {
			left := copyFixtureToTempDir("testdata/go_multiworkspace")
			right := copyFixtureToTempDir("testdata/go_multiworkspace")
			Expect(left).NotTo(Equal(right))

			leftModel, err := projectmodel.BuildGoModel(os.DirFS(left), testMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())
			rightModel, err := projectmodel.BuildGoModel(os.DirFS(right), testMeta(), projectmodel.GoBuildOptions{})
			Expect(err).NotTo(HaveOccurred())

			leftJSON, err := json.Marshal(leftModel)
			Expect(err).NotTo(HaveOccurred())
			rightJSON, err := json.Marshal(rightModel)
			Expect(err).NotTo(HaveOccurred())
			Expect(leftJSON).To(Equal(rightJSON))
			Expect(string(leftJSON)).NotTo(ContainSubstring(left))
			Expect(string(leftJSON)).NotTo(ContainSubstring(right))
		})
	})
})

func copyFixtureToTempDir(fixture string) string {
	dir := GinkgoT().TempDir()
	Expect(os.CopyFS(dir, os.DirFS(fixture))).To(Succeed())
	return dir
}
