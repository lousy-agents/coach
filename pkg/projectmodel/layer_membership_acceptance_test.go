package projectmodel_test

import (
	"context"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("layer membership: how many project files under the configured roots sit outside every declared layer", func() {
	layers := []projectmodel.ProjectScopePolicyLayer{
		{Name: "handlers", Prefixes: []string{"handlers"}},
		{Name: "db", Prefixes: []string{"services/payments/db"}},
	}

	Describe("ProjectScopeFromModel for TypeScript", func() {
		scopeOf := func(model projectmodel.Model, roots []string, layers []projectmodel.ProjectScopePolicyLayer) projectmodel.ProjectScope {
			scope, err := projectmodel.ProjectScopeFromModel(model, projectmodel.ProjectScopePolicy{Roots: roots, Layers: layers})
			Expect(err).NotTo(HaveOccurred())
			return scope
		}

		It("counts the project files under the roots that no layer prefix contains", func() {
			model := projectmodel.Model{RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 3, AnalyzedFiles: 3, AnalyzedPaths: []string{"handlers/app.ts", "services/payments/db/client.ts", "util/misc.ts"}},
			}}

			scope := scopeOf(model, []string{"."}, layers)

			Expect(scope.LayerMembership).To(Equal(&domain.LayerMembership{AnalyzedFiles: 3, FilesOutsideLayers: 1}))
		})

		It("counts a file once when nested roots both analyze it", func() {
			model := projectmodel.Model{RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 2, AnalyzedFiles: 2, AnalyzedPaths: []string{"services/payments/db/client.ts", "util/misc.ts"}},
				{Root: "services/payments", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"services/payments/db/client.ts"}},
			}}

			scope := scopeOf(model, []string{".", "services/payments"}, layers)

			Expect(scope.LayerMembership).To(Equal(&domain.LayerMembership{AnalyzedFiles: 2, FilesOutsideLayers: 1}),
				"the same file reported by two nested roots is one analyzed file, not two")
		})

		It("reports a real zero when a single layer's prefix is . and something was analyzed", func() {
			model := projectmodel.Model{RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 2, AnalyzedFiles: 2, AnalyzedPaths: []string{"a.ts", "b/c.ts"}},
			}}

			scope := scopeOf(model, []string{"."}, []projectmodel.ProjectScopePolicyLayer{{Name: "all", Prefixes: []string{"."}}})

			Expect(scope.LayerMembership).To(Equal(&domain.LayerMembership{AnalyzedFiles: 2, FilesOutsideLayers: 0}))
		})

		It("does not compute a count when the analyzer reported counts without the matching paths", func() {
			model := projectmodel.Model{RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 40, AnalyzedFiles: 40, AnalyzedPaths: []string{"handlers/app.ts"}},
			}}

			scope := scopeOf(model, []string{"."}, layers)

			Expect(scope.LayerMembership).To(BeNil(), "deriving the count from one path of forty would report a false zero")
		})

		It("does not compute a count when the analyzer reported more paths than analyzed files", func() {
			model := projectmodel.Model{RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"handlers/app.ts", "util/misc.ts"}},
			}}

			scope := scopeOf(model, []string{"."}, layers)

			Expect(scope.LayerMembership).To(BeNil(), "a path list that disagrees with the count in either direction cannot be trusted")
		})

		When("the client's input budget truncated the files sent to the analyzer", func() {
			snapshot := fstest.MapFS{
				"handlers/app.ts": &fstest.MapFile{Data: []byte("export const app = 1;\n")},
				"util/a.ts":       &fstest.MapFile{Data: []byte("export const a = 1;\n")},
				"util/b.ts":       &fstest.MapFile{Data: []byte("export const b = 1;\n")},
			}
			buildWith := func(budgets projectmodel.GoBudgets) projectmodel.Model {
				opts := sidecarOptsWithMode("root_scope_from_files")
				opts.Budgets = budgets
				model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), snapshot, testMeta(), opts)
				Expect(err).NotTo(HaveOccurred())
				return model
			}

			It("counts every file when nothing was truncated", func() {
				scope := scopeOf(buildWith(projectmodel.GoBudgets{}), []string{"."}, layers)

				Expect(scope.LayerMembership).To(Equal(&domain.LayerMembership{AnalyzedFiles: 3, FilesOutsideLayers: 2}))
			})

			It("does not compute a count, because the analyzer's counts and paths agree yet both omit the skipped tail", func() {
				model := buildWith(projectmodel.GoBudgets{MaxInputFiles: 1})
				Expect(model.Coverage.Complete).To(BeFalse(), "the budget must actually truncate, or this spec proves nothing")

				scope := scopeOf(model, []string{"."}, layers)

				Expect(scope.LayerMembership).To(BeNil(), "counting the one admitted handlers file would report a false zero for util/a.ts and util/b.ts")
			})
		})

		It("does not compute a count when the policy declares no layers", func() {
			model := projectmodel.Model{RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"a.ts"}},
			}}

			scope := scopeOf(model, []string{"."}, nil)

			Expect(scope.LayerMembership).To(BeNil())
		})
	})

	Describe("GoLayerMembership", func() {
		goModel := func(paths ...string) projectmodel.Model {
			files := make([]projectmodel.File, len(paths))
			for i, p := range paths {
				files[i] = projectmodel.File{Path: p}
			}
			return projectmodel.Model{Files: files, Coverage: projectmodel.Coverage{Complete: true}}
		}

		DescribeTable("matches the importing package directory the way the layer rule does",
			func(paths []string, layers []projectmodel.ProjectScopePolicyLayer, want domain.LayerMembership) {
				got := projectmodel.GoLayerMembership(goModel(paths...), layers)

				Expect(got).To(Equal(&want))
			},
			Entry("files in one package directory are in or out together",
				[]string{"pkg/db/db.go", "pkg/db/query.go", "pkg/other/o.go"},
				[]projectmodel.ProjectScopePolicyLayer{{Name: "db", Prefixes: []string{"pkg/db"}}},
				domain.LayerMembership{AnalyzedFiles: 3, FilesOutsideLayers: 1}),
			Entry("a prefix contains its subdirectories",
				[]string{"pkg/db/sub/x.go"},
				[]projectmodel.ProjectScopePolicyLayer{{Name: "db", Prefixes: []string{"pkg/db"}}},
				domain.LayerMembership{AnalyzedFiles: 1, FilesOutsideLayers: 0}),
			Entry("a prefix that only shares a name stem does not contain the directory",
				[]string{"pkg/dbx/x.go"},
				[]projectmodel.ProjectScopePolicyLayer{{Name: "db", Prefixes: []string{"pkg/db"}}},
				domain.LayerMembership{AnalyzedFiles: 1, FilesOutsideLayers: 1}),
			Entry("the prefix . contains every directory, including the root",
				[]string{"main.go", "pkg/db/db.go"},
				[]projectmodel.ProjectScopePolicyLayer{{Name: "all", Prefixes: []string{"."}}},
				domain.LayerMembership{AnalyzedFiles: 2, FilesOutsideLayers: 0}),
			Entry("a prefix that names a file never contains a package directory",
				[]string{"pkg/db/db.go"},
				[]projectmodel.ProjectScopePolicyLayer{{Name: "db", Prefixes: []string{"pkg/db/db.go"}}},
				domain.LayerMembership{AnalyzedFiles: 1, FilesOutsideLayers: 1}),
			Entry("a root-level file is outside a layer that does not use .",
				[]string{"main.go"},
				[]projectmodel.ProjectScopePolicyLayer{{Name: "db", Prefixes: []string{"pkg/db"}}},
				domain.LayerMembership{AnalyzedFiles: 1, FilesOutsideLayers: 1}),
		)

		It("does not compute a count when the policy declares no layers", func() {
			Expect(projectmodel.GoLayerMembership(goModel("a.go"), nil)).To(BeNil())
		})

		It("does not compute a count when the model is incomplete", func() {
			model := goModel("pkg/db/db.go")
			model.Coverage.Complete = false

			Expect(projectmodel.GoLayerMembership(model, layers)).To(BeNil(), "a truncated model holds only the analyzed head of the files")
		})
	})
})
