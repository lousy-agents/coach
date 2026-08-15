package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func twoLayerPolicy() codesignal.LayerPolicy {
	return codesignal.LayerPolicy{
		Layers: []codesignal.ArchitectureLayer{
			{Name: "handlers", Prefixes: []string{"pkg/handlers"}},
			{Name: "db", Prefixes: []string{"pkg/db"}},
		},
		ForbiddenImports: []codesignal.ForbiddenLayerImport{
			{From: "handlers", To: "db"},
		},
	}
}

var _ = Describe("EvaluateGoLayerViolations", func() {
	When("an internal edge crosses a forbidden layer boundary", func() {
		It("emits exactly one architecture.layer_violation ProjectChange with the frozen field shape", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/h.go:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

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
			Expect(change.SemanticKey).To(Equal("architecture.layer_violation:pkg/handlers->pkg/db"))
			Expect(change.MachineEvidence).To(Equal(map[string]string{
				"importer":   "pkg/handlers",
				"importee":   "pkg/db",
				"layer_from": "handlers",
				"layer_to":   "db",
				"rule":       "handlers->db",
			}))
			Expect(change.PrimaryAnchor).To(Equal(codesignal.ProjectLocation{
				Path:     "pkg/handlers/h.go",
				Location: semantics.Location{StartRow: 9},
			}))
			Expect(change.RelatedLocations).To(BeEmpty())
			Expect(change.WhyItMatters).NotTo(BeEmpty())
			Expect(change.Recommendation).NotTo(BeEmpty())
			Expect(change.Provenance).To(Equal(codesignal.Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_violation"}))
		})
	})

	When("the import direction is allowed by policy", func() {
		It("emits zero ProjectChanges (negative control: allowed inter-layer import)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/db", To: "package:pkg/handlers", Kind: "internal", Site: "pkg/db/d.go:5"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("an edge is not Kind internal", func() {
		It("never produces a violation even when From/To would otherwise match a forbidden pair (negative control)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "external", Site: "pkg/handlers/h.go:10"},
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "unresolved", Site: "pkg/handlers/h.go:11"},
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "excluded", Site: "pkg/handlers/h.go:12"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("an endpoint's package directory is not covered by any configured layer", func() {
		It("emits zero ProjectChanges and zero diagnostics (negative control: unmapped package)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/handlers", To: "package:pkg/other", Kind: "internal", Site: "pkg/handlers/h.go:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("multiple import edges violate the same layer pair between the same packages", func() {
		It("collapses them into one ProjectChange with the lexicographically-first site as PrimaryAnchor and the rest as RelatedLocations", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/z.go:3"},
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/a.go:7"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			change := changes[0]
			Expect(change.PrimaryAnchor).To(Equal(codesignal.ProjectLocation{
				Path:     "pkg/handlers/a.go",
				Location: semantics.Location{StartRow: 6},
			}))
			Expect(change.RelatedLocations).To(Equal([]codesignal.ProjectLocation{
				{Path: "pkg/handlers/z.go", Location: semantics.Location{StartRow: 2}},
			}))
		})
	})

	When("multiple violating pairs are supplied out of sorted order", func() {
		It("emits ProjectChanges deterministically ordered by (importerPkgDir, importeePkgDir)", func() {
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
					{From: "package:pkg/api", To: "package:pkg/store", Kind: "internal", Site: "pkg/api/a.go:1"},
					{From: "package:pkg/handlers", To: "package:pkg/store", Kind: "internal", Site: "pkg/handlers/h.go:2"},
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/h.go:3"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, policy, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(3))
			semanticKeys := make([]string, len(changes))
			for i, change := range changes {
				semanticKeys[i] = change.SemanticKey
			}
			Expect(semanticKeys).To(Equal([]string{
				"architecture.layer_violation:pkg/api->pkg/store",
				"architecture.layer_violation:pkg/handlers->pkg/db",
				"architecture.layer_violation:pkg/handlers->pkg/store",
			}))
		})
	})

	When("the importer package directory is a descendant of a configured layer prefix", func() {
		It("matches the layer via the descendant rule and emits one violation", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/handlers/http", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/http/h.go:1"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].SemanticKey).To(Equal("architecture.layer_violation:pkg/handlers/http->pkg/db"))
		})
	})

	When("the importer package directory is a sibling that merely shares a prefix string with a configured layer", func() {
		It("does not match the layer and emits zero ProjectChanges (negative control: sibling-prefix false match)", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/handlersx", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlersx/h.go:1"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("the policy has no layers or no forbidden imports configured", func() {
		It("returns nil, nil immediately regardless of how many internal edges the model has", func() {
			model := projectmodel.Model{
				ImportEdges: []projectmodel.ImportEdge{
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/h.go:10"},
					{From: "package:pkg/db", To: "package:pkg/handlers", Kind: "internal", Site: "pkg/db/d.go:5"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, codesignal.LayerPolicy{}, "1", "backend-1", "digest-1")

			Expect(changes).To(BeNil())
			Expect(diagnostics).To(BeNil())
		})
	})

	// "." is validated as the universal ancestor of every other prefix
	// (internal/codesignalcli hasDuplicateOrOverlappingPaths). matchLayer
	// must treat it the same way; otherwise a valid config with
	// prefixes:["."] only matches the root package directory "." and
	// silently drops nested-package edges (complete:true + zero findings).
	When(`a layer's only prefix is the universal repository-root ancestor "."`, func() {
		It("matches nested package directories so a forbidden edge between them is emitted rather than silently dropped", func() {
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
					{From: "package:pkg/handlers", To: "package:pkg/db", Kind: "internal", Site: "pkg/handlers/h.go:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerViolations(model, policy, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1), `prefix "." must match nested dirs pkg/handlers and pkg/db, not only the root package "."`)
			Expect(changes[0].RuleID).To(Equal("architecture.layer_violation"))
			Expect(changes[0].SemanticKey).To(Equal("architecture.layer_violation:pkg/handlers->pkg/db"))
			Expect(changes[0].MachineEvidence).To(Equal(map[string]string{
				"importer":   "pkg/handlers",
				"importee":   "pkg/db",
				"layer_from": "app",
				"layer_to":   "app",
				"rule":       "app->app",
			}))
		})
	})
})
