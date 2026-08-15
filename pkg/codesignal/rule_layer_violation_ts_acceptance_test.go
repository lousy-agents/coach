package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

var _ = Describe("EvaluateTypeScriptLayerViolations", func() {
	When("a value-level import edge crosses a forbidden layer boundary", func() {
		It("emits exactly one architecture.layer_violation ProjectChange tagged with language typescript", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "import",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			change := changes[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.Kind).To(Equal("architecture.layer_violation"))
			Expect(change.Category).To(Equal(codesignal.Category("architecture")))
			Expect(change.Severity).To(Equal(codesignal.Severity("advisory")))
			Expect(change.Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(change.RuleVersion).To(Equal("1"))
			Expect(change.BackendVersion).To(Equal("backend-1"))
			Expect(change.ConfigDigest).To(Equal("digest-1"))
			Expect(change.SemanticKey).To(Equal("architecture.layer_violation:pkg/handlers/h.ts->pkg/db/d.ts"))
			Expect(change.MachineEvidence).To(Equal(map[string]string{
				"importer":   "pkg/handlers/h.ts",
				"importee":   "pkg/db/d.ts",
				"layer_from": "handlers",
				"layer_to":   "db",
				"rule":       "handlers->db",
				"language":   "typescript",
			}))
			Expect(change.PrimaryAnchor).To(Equal(codesignal.ProjectLocation{
				Path:     "pkg/handlers/h.ts",
				Location: semantics.Location{StartRow: 9},
			}))
			Expect(change.WhyItMatters).NotTo(BeEmpty())
			Expect(change.Recommendation).NotTo(BeEmpty())
			Expect(change.Evidence).To(Equal("pkg/handlers/h.ts imports pkg/db/d.ts (layer handlers -> db is forbidden)"))
			Expect(change.Provenance).To(Equal(codesignal.Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_violation"}))
		})
	})

	When("a reexport edge crosses a forbidden layer boundary", func() {
		It("emits exactly one architecture.layer_violation ProjectChange", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "reexport",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
		})
	})

	When("the edge is type-only", func() {
		It("emits zero ProjectChanges and zero diagnostics (negative control: type-only import must not be a runtime violation)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "type_only",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("the edge is a commonjs_require", func() {
		It("emits zero ProjectChanges (negative control: commonjs_require is not eligible)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "commonjs_require",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("a dynamic_import edge crosses a forbidden layer boundary but resolves outside the snapshot", func() {
		It("emits zero ProjectChanges and zero diagnostics (silent coverage gap, matching Go's convention)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "unresolved:pkg/db/d",
						Kind:       "dynamic_import",
						Resolution: "unresolved",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeNil())
			Expect(changes).To(BeNil())
		})
	})

	When("a dynamic_import edge crosses a forbidden layer boundary and resolves within the snapshot", func() {
		It("emits zero ProjectChanges (negative control: dynamic_import is not eligible even when file-addressed)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "dynamic_import",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("an import edge resolves to an external package", func() {
		It("emits zero ProjectChanges and zero diagnostics (silent coverage gap: external: targets carry no file: prefix and are skipped)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "external:some-lib",
						Kind:       "import",
						Resolution: "external",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeNil())
			Expect(changes).To(BeNil())
		})
	})

	When("the import direction is allowed by policy", func() {
		It("emits zero ProjectChanges (negative control: allowed inter-layer import)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/db/d.ts",
						To:         "file:pkg/handlers/h.ts",
						Kind:       "import",
						Resolution: "snapshot",
						Site:       "pkg/db/d.ts:5",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("an endpoint's file is not covered by any configured layer", func() {
		It("emits zero ProjectChanges and zero diagnostics (negative control: unmapped file)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/other/o.ts",
						Kind:       "import",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("an edge would otherwise violate the forbidden layer pair but neither endpoint carries the file: prefix", func() {
		It("emits zero ProjectChanges and zero diagnostics (silent coverage gap: non-file-addressed edges are skipped)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "pkg/handlers/h.ts",
						To:         "pkg/db/d.ts",
						Kind:       "import",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When(`the policy has a universal repository-root layer ("prefixes: [\".\"]") and the importee lacks the file: prefix`, func() {
		It("emits zero ProjectChanges and zero diagnostics (the file: prefix check, not layer matching, is what excludes non-file-addressed importees)", func() {
			policy := codesignal.LayerPolicy{
				Layers: []codesignal.ArchitectureLayer{
					{Name: "app", Prefixes: []string{"."}},
				},
				ForbiddenImports: []codesignal.ForbiddenLayerImport{
					{From: "app", To: "app"},
				},
			}
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "external:lodash",
						Kind:       "import",
						Resolution: "external",
						Site:       "pkg/handlers/h.ts:1",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, policy, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("the importer lacks the file: prefix but the importee carries it", func() {
		It("emits zero ProjectChanges and zero diagnostics (the importer-side file: prefix check alone excludes this edge)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "import",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:1",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("a file-addressed import edge crosses a forbidden layer boundary but carries no Resolution value", func() {
		It("still emits a ProjectChange (Resolution is not part of the eligibility rule; file: addressing alone is sufficient)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From: "file:pkg/handlers/h.ts",
						To:   "file:pkg/db/d.ts",
						Kind: "import",
						Site: "pkg/handlers/h.ts:10",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].SemanticKey).To(Equal("architecture.layer_violation:pkg/handlers/h.ts->pkg/db/d.ts"))
		})
	})

	When("multiple eligible edges share the same (importer file, importee file) pair", func() {
		It("collapses them into one ProjectChange with the lexicographically-first site as PrimaryAnchor and the rest as RelatedLocations", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "reexport",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:7",
					},
					{
						From:       "file:pkg/handlers/h.ts",
						To:         "file:pkg/db/d.ts",
						Kind:       "import",
						Resolution: "snapshot",
						Site:       "pkg/handlers/h.ts:3",
					},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			change := changes[0]
			Expect(change.RuleVersion).To(Equal("1"))
			Expect(change.BackendVersion).To(Equal("backend-1"))
			Expect(change.ConfigDigest).To(Equal("digest-1"))
			Expect(change.PrimaryAnchor).To(Equal(codesignal.ProjectLocation{
				Path:     "pkg/handlers/h.ts",
				Location: semantics.Location{StartRow: 2},
			}))
			Expect(change.RelatedLocations).To(Equal([]codesignal.ProjectLocation{
				{Path: "pkg/handlers/h.ts", Location: semantics.Location{StartRow: 6}},
			}))
		})
	})

	When("multiple violating pairs are supplied out of sorted order", func() {
		It("emits ProjectChanges deterministically ordered by (importerFile, importeeFile)", func() {
			policy := codesignal.LayerPolicy{
				Layers: []codesignal.ArchitectureLayer{
					{Name: "handlers", Prefixes: []string{"pkg/handlers", "pkg/api"}},
					{Name: "db", Prefixes: []string{"pkg/db", "pkg/store"}},
				},
				ForbiddenImports: []codesignal.ForbiddenLayerImport{
					{From: "handlers", To: "db"},
				},
			}
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "file:pkg/api/a.ts", To: "file:pkg/store/s.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/api/a.ts:1"},
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/store/s.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:1"},
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:1"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, policy, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(3))
			semanticKeys := make([]string, len(changes))
			for i, change := range changes {
				semanticKeys[i] = change.SemanticKey
			}
			Expect(semanticKeys).To(Equal([]string{
				"architecture.layer_violation:pkg/api/a.ts->pkg/store/s.ts",
				"architecture.layer_violation:pkg/handlers/h.ts->pkg/db/d.ts",
				"architecture.layer_violation:pkg/handlers/h.ts->pkg/store/s.ts",
			}))
		})
	})

	When("the policy has no layers or no forbidden imports configured", func() {
		It("returns nil, nil immediately regardless of how many edges the model has", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, codesignal.LayerPolicy{}, "1", "backend-1", "digest-1")

			Expect(changes).To(BeNil())
			Expect(diagnostics).To(BeNil())
		})
	})
})
