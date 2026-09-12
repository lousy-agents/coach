package projectmodel_test

import (
	"encoding/json"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// Issue #331 AC-13 / Task 9: project_scope must be derivable from a Model
// plus a layer policy alone -- no context, snapshot, or sidecar options.
var _ = Describe("ProjectScopeFromModel", func() {
	It("has exactly the two-parameter (model, policy) signature", func() {
		fn := reflect.TypeOf(projectmodel.ProjectScopeFromModel)
		Expect(fn.Kind()).To(Equal(reflect.Func))
		Expect(fn.NumIn()).To(Equal(2), "ProjectScopeFromModel must take exactly model and policy -- no context or sidecar options")
		Expect(fn.In(0)).To(Equal(reflect.TypeOf(projectmodel.Model{})), "first parameter must be the package's own Model")
		Expect(fn.In(1)).To(Equal(reflect.TypeOf(projectmodel.ProjectScopePolicy{})), "second parameter must be a layer policy, not a wider option/context type")
		Expect(fn.NumOut()).To(Equal(2))
		Expect(fn.Out(0)).To(Equal(reflect.TypeOf(projectmodel.ProjectScope{})))
		Expect(fn.Out(1)).To(Equal(reflect.TypeOf((*error)(nil)).Elem()))
	})

	When("the model reports two roots, including a nested root, and the policy names 3 layers", func() {
		policy := projectmodel.ProjectScopePolicy{
			Roots: []string{".", "services/payments"},
			Layers: []projectmodel.ProjectScopePolicyLayer{
				{Name: "handlers", Prefixes: []string{"handlers"}},
				{Name: "db", Prefixes: []string{"services/payments/db"}},
				{Name: "queue", Prefixes: []string{"queue"}},
			},
		}

		modelA := projectmodel.Model{
			RootScopes: []projectmodel.RootScope{
				{Root: "services/payments", CandidateFiles: 12, AnalyzedFiles: 12, AnalyzedPaths: []string{"services/payments/db/client.ts"}},
				{Root: ".", CandidateFiles: 42, AnalyzedFiles: 40, AnalyzedPaths: []string{"handlers/app.ts"}},
			},
		}

		It("produces a ProjectScope with roots in policy declaration order, independent per-root counts, and correct matched/unmatched layer partitioning", func() {
			scope, err := projectmodel.ProjectScopeFromModel(modelA, policy)
			Expect(err).NotTo(HaveOccurred())

			Expect(scope.Roots).To(Equal([]projectmodel.ProjectScopeRoot{
				{Root: ".", CandidateFiles: 42, AnalyzedFiles: 40},
				{Root: "services/payments", CandidateFiles: 12, AnalyzedFiles: 12},
			}), "roots must be ordered by policy declaration order (\".\" then \"services/payments\"), not model insertion order, and each root's counts must be its own -- never summed across nested roots")

			Expect(scope.MatchedLayers).To(Equal([]string{"handlers", "db"}), "both handlers and db have an analyzed file under their prefix")
			Expect(scope.UnmatchedLayers).To(Equal([]string{"queue"}), "queue has no analyzed file under its prefix")

			Expect(scope.InclusionRule).To(Equal("tsconfig_includes_no_test_classification"))
			Expect(scope.PatternSet).To(Equal(projectmodel.TSReachabilityAlgorithm))
		})

		It("produces a byte-for-byte identical ProjectScope when RootScopes arrive in a different insertion order", func() {
			modelB := projectmodel.Model{
				RootScopes: []projectmodel.RootScope{
					{Root: ".", CandidateFiles: 42, AnalyzedFiles: 40, AnalyzedPaths: []string{"handlers/app.ts"}},
					{Root: "services/payments", CandidateFiles: 12, AnalyzedFiles: 12, AnalyzedPaths: []string{"services/payments/db/client.ts"}},
				},
			}

			scopeA, errA := projectmodel.ProjectScopeFromModel(modelA, policy)
			Expect(errA).NotTo(HaveOccurred())
			scopeB, errB := projectmodel.ProjectScopeFromModel(modelB, policy)
			Expect(errB).NotTo(HaveOccurred())

			Expect(reflect.DeepEqual(scopeA, scopeB)).To(BeTrue(), "output must be driven by policy declaration order, not model insertion order")

			jsonA, err := json.Marshal(scopeA)
			Expect(err).NotTo(HaveOccurred())
			jsonB, err := json.Marshal(scopeB)
			Expect(err).NotTo(HaveOccurred())
			Expect(jsonA).To(MatchJSON(jsonB))
		})
	})

	When("the same analyzed file is reported in two overlapping roots' AnalyzedPaths", func() {
		policy := projectmodel.ProjectScopePolicy{
			Roots: []string{".", "services/payments"},
			Layers: []projectmodel.ProjectScopePolicyLayer{
				{Name: "db", Prefixes: []string{"services/payments/db"}},
			},
		}
		model := projectmodel.Model{
			RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 5, AnalyzedFiles: 1, AnalyzedPaths: []string{"services/payments/db/client.ts"}},
				{Root: "services/payments", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"services/payments/db/client.ts"}},
			},
		}

		It("still reports the layer matched exactly once, deduplicating the path shared by both roots", func() {
			scope, err := projectmodel.ProjectScopeFromModel(model, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(scope.MatchedLayers).To(Equal([]string{"db"}))
		})
	})

	When("the model reports a RootScope for a root the policy never selected", func() {
		policy := projectmodel.ProjectScopePolicy{
			Roots: []string{"."},
			Layers: []projectmodel.ProjectScopePolicyLayer{
				{Name: "vendor", Prefixes: []string{"vendor"}},
			},
		}
		model := projectmodel.Model{
			RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"handlers/app.ts"}},
				{Root: "vendor", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"vendor/lib.ts"}},
			},
		}

		It("reports the layer unmatched, since the policy never selected the vendor root", func() {
			scope, err := projectmodel.ProjectScopeFromModel(model, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(scope.UnmatchedLayers).To(Equal([]string{"vendor"}), "vendor/lib.ts belongs to a RootScope outside policy.Roots and must not make the layer match")
			Expect(scope.Roots).To(Equal([]projectmodel.ProjectScopeRoot{
				{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1},
			}), "Roots must list only the policy-selected root, not every RootScope the model happens to carry")
		})
	})

	When("a root's analyzed file has no import/export/require statement of its own", func() {
		policy := projectmodel.ProjectScopePolicy{
			Roots: []string{"."},
			Layers: []projectmodel.ProjectScopePolicyLayer{
				{Name: "leaf", Prefixes: []string{"leaf"}},
			},
		}
		model := projectmodel.Model{
			RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"leaf/constants.ts"}},
			},
		}

		It("reports the layer as matched, since RootScope.AnalyzedPaths (not ImportEdges) proves the file was analyzed", func() {
			scope, err := projectmodel.ProjectScopeFromModel(model, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(scope.MatchedLayers).To(Equal([]string{"leaf"}), "leaf/constants.ts has no ImportEdge of its own but is present in AnalyzedPaths, so leaf must match")
			Expect(scope.UnmatchedLayers).To(Equal([]string{}))
		})
	})

	When("a layer's prefix appears only as an ImportEdge's To endpoint (present but unread by ProjectScopeFromModel), never in any root's AnalyzedPaths", func() {
		policy := projectmodel.ProjectScopePolicy{
			Roots: []string{"."},
			Layers: []projectmodel.ProjectScopePolicyLayer{
				{Name: "vendorish", Prefixes: []string{"generated"}},
			},
		}
		model := projectmodel.Model{
			RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 2, AnalyzedFiles: 2, AnalyzedPaths: []string{"handlers/app.ts"}},
			},
			ImportEdges: []projectmodel.ImportEdge{
				{From: "file:handlers/app.ts", To: "file:generated/schema.ts", Kind: "import"},
			},
		}

		It("reports the layer as unmatched, since no root's AnalyzedPaths contains a file under the prefix", func() {
			scope, err := projectmodel.ProjectScopeFromModel(model, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(scope.UnmatchedLayers).To(Equal([]string{"vendorish"}), "generated/schema.ts is only ever an edge To endpoint, never present in any root's AnalyzedPaths")
			Expect(scope.MatchedLayers).To(Equal([]string{}))
		})
	})

	When("every policy layer matches an analyzed file", func() {
		policy := projectmodel.ProjectScopePolicy{
			Roots: []string{"."},
			Layers: []projectmodel.ProjectScopePolicyLayer{
				{Name: "handlers", Prefixes: []string{"handlers"}},
			},
		}
		model := projectmodel.Model{
			RootScopes: []projectmodel.RootScope{
				{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1, AnalyzedPaths: []string{"handlers/app.ts"}},
			},
		}

		It("still renders both matched_layers and unmatched_layers keys in the JSON, with unmatched_layers as an empty array rather than omitted", func() {
			scope, err := projectmodel.ProjectScopeFromModel(model, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(scope.MatchedLayers).To(Equal([]string{"handlers"}))
			Expect(scope.UnmatchedLayers).To(Equal([]string{}))

			raw, err := json.Marshal(scope)
			Expect(err).NotTo(HaveOccurred())
			var decoded map[string]json.RawMessage
			Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
			Expect(decoded).To(HaveKey("matched_layers"))
			Expect(decoded).To(HaveKey("unmatched_layers"), "unmatched_layers must always be present, never omitted, even when it is empty")
			Expect(string(decoded["unmatched_layers"])).To(Equal("[]"), "unmatched_layers must render as [] rather than null when empty")
		})
	})

	When("the policy names a root the model has no root_scopes entry for", func() {
		It("returns a non-nil error", func() {
			policy := projectmodel.ProjectScopePolicy{Roots: []string{"services/missing"}}
			model := projectmodel.Model{
				RootScopes: []projectmodel.RootScope{
					{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1},
				},
			}
			_, err := projectmodel.ProjectScopeFromModel(model, policy)
			Expect(err).To(HaveOccurred())
		})
	})

	When("a policy root is written with a leading \"./\" or trailing slash but the model's root_scopes entry is already normalized", func() {
		It("still matches, since both sides are normalized the same way before lookup", func() {
			policy := projectmodel.ProjectScopePolicy{Roots: []string{"./services/payments/"}}
			model := projectmodel.Model{
				RootScopes: []projectmodel.RootScope{
					{Root: "services/payments", CandidateFiles: 3, AnalyzedFiles: 3},
				},
			}
			scope, err := projectmodel.ProjectScopeFromModel(model, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(scope.Roots).To(Equal([]projectmodel.ProjectScopeRoot{
				{Root: "services/payments", CandidateFiles: 3, AnalyzedFiles: 3},
			}))
		})
	})
})
