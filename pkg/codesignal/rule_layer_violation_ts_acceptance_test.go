package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/semantics"
)

var _ = Describe("EvaluateTypeScriptLayerViolations", func() {
	When("a value-level import edge crosses a forbidden layer boundary", func() {
		It("emits exactly one architecture.layer_violation ProjectChange tagged with language typescript", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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

	When("the edge is type-only and the policy does not opt in to runtime-only evaluation", func() {
		It("emits exactly one architecture.layer_violation, because a type-only import is still a compile-time dependency", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].RuleID).To(Equal("architecture.layer_violation"))
			Expect(changes[0].MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
			Expect(changes[0].MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
		})
	})

	When("the policy sets ExcludeTypeOnlyEdges", func() {
		excludingPolicy := func() codesignal.LayerPolicy {
			policy := twoLayerPolicy()
			policy.ExcludeTypeOnlyEdges = true
			return policy
		}

		It("emits zero ProjectChanges and zero diagnostics for a type-only edge", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "type_only", Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, excludingPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})

		It("still emits a ProjectChange for a value-level edge across the same forbidden pair", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, _ := codesignal.EvaluateTypeScriptLayerViolations(model, excludingPolicy(), "1", "backend-1", "digest-1")

			Expect(changes).To(HaveLen(1))
		})

		It("drops only the type-only edge of a pair that also has a value-level edge, so the finding anchors on the value-level site", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "type_only", Resolution: "snapshot", Site: "pkg/handlers/h.ts:1"},
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
				},
			}

			excluded, _ := codesignal.EvaluateTypeScriptLayerViolations(model, excludingPolicy(), "1", "backend-1", "digest-1")
			Expect(excluded).To(HaveLen(1))
			Expect(excluded[0].PrimaryAnchor.Location.StartRow).To(Equal(uint(9)))
			Expect(excluded[0].RelatedLocations).To(BeEmpty())

			included, _ := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")
			Expect(included).To(HaveLen(1))
			Expect(included[0].PrimaryAnchor.Location.StartRow).To(Equal(uint(0)), "control: without the option the type-only site is part of the finding")
		})
	})

	When("every edge in a violating pair is type-only", func() {
		It("records that the finding is removed if exclude_type_only_edges is set", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "type_only", Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, _ := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(changes).To(HaveLen(1))
			Expect(changes[0].MachineEvidence).To(HaveKeyWithValue("removed_if_set", "exclude_type_only_edges"))
		})
	})

	When("a violating pair has a value-level edge alongside a type-only edge", func() {
		It("does not claim the finding is removed by the option", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "type_only", Resolution: "snapshot", Site: "pkg/handlers/h.ts:1"},
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, _ := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(changes).To(HaveLen(1))
			Expect(changes[0].MachineEvidence).NotTo(HaveKey("removed_if_set"))
		})
	})

	When("the edge is a commonjs_require or a dynamic_import that resolves within the snapshot", func() {
		DescribeTable("emits exactly one architecture.layer_violation, evaluated like a static import (coach#561)",
			func(kind string) {
				model := domain.Model{
					ImportEdges: []domain.ImportEdge{
						{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: kind, Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
					},
				}

				changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

				Expect(diagnostics).To(BeEmpty())
				Expect(changes).To(HaveLen(1))
				Expect(changes[0].RuleID).To(Equal("architecture.layer_violation"))
				Expect(changes[0].MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
				Expect(changes[0].MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
				Expect(changes[0].PrimaryAnchor.Location.StartRow).To(Equal(uint(9)))
			},
			Entry("a commonjs_require", "commonjs_require"),
			Entry("a dynamic_import", "dynamic_import"),
		)
	})

	When("an edge resolves to an unresolved target", func() {
		It("emits no ProjectChange and one project_import_unresolved diagnostic naming the file, the specifier, and the site", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "unresolved:pkg/db/d", Kind: "dynamic_import", Resolution: "unresolved", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal(codesignal.DiagKindProjectImportUnresolved))
			Expect(diagnostics[0].Path).To(Equal("pkg/handlers/h.ts"))
			Expect(diagnostics[0].Message).To(ContainSubstring(`"pkg/db/d"`))
			Expect(diagnostics[0].Message).To(ContainSubstring("did not resolve"))
			Expect(diagnostics[0].Location).NotTo(BeNil())
			Expect(diagnostics[0].Location.StartRow).To(Equal(uint(9)))
		})

		DescribeTable("treats every evaluated edge kind alike",
			func(kind string) {
				model := domain.Model{
					ImportEdges: []domain.ImportEdge{
						{From: "file:pkg/handlers/h.ts", To: "unresolved:../missing/m", Kind: kind, Resolution: "unresolved", Site: "pkg/handlers/h.ts:3"},
					},
				}

				_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

				Expect(diagnostics).To(HaveLen(1))
				Expect(diagnostics[0].Kind).To(Equal(codesignal.DiagKindProjectImportUnresolved))
			},
			Entry("an import", "import"),
			Entry("a reexport", "reexport"),
			Entry("a type-only import", "type_only"),
			Entry("a commonjs_require", "commonjs_require"),
			Entry("a dynamic_import", "dynamic_import"),
		)

		It("orders several diagnostics by file, then line, whatever order the edges arrive in", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/z.ts", To: "unresolved:a", Kind: "import", Resolution: "unresolved", Site: "pkg/handlers/z.ts:2"},
					{From: "file:pkg/handlers/a.ts", To: "unresolved:b", Kind: "import", Resolution: "unresolved", Site: "pkg/handlers/a.ts:9"},
					{From: "file:pkg/handlers/a.ts", To: "unresolved:c", Kind: "import", Resolution: "unresolved", Site: "pkg/handlers/a.ts:3"},
				},
			}

			_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			var order []string
			for _, diagnostic := range diagnostics {
				order = append(order, diagnostic.Path)
			}
			Expect(order).To(Equal([]string{"pkg/handlers/a.ts", "pkg/handlers/a.ts", "pkg/handlers/z.ts"}))
			Expect(diagnostics[0].Location.StartRow).To(Equal(uint(2)))
			Expect(diagnostics[1].Location.StartRow).To(Equal(uint(8)))
		})

		It("emits nothing for an importer that sits outside every declared layer, because no rule applies to it", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/other/o.ts", To: "unresolved:gone", Kind: "import", Resolution: "unresolved", Site: "pkg/other/o.ts:1"},
					{From: "file:pkg/other/o.ts", To: "external:lodash", Kind: "import", Resolution: "external", Site: "pkg/other/o.ts:2"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
			Expect(diagnostics).To(BeEmpty())
		})

		It("emits nothing for a type-only edge when the policy excludes type-only edges", func() {
			policy := twoLayerPolicy()
			policy.ExcludeTypeOnlyEdges = true
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "unresolved:gone", Kind: "type_only", Resolution: "unresolved", Site: "pkg/handlers/h.ts:1"},
					{From: "file:pkg/handlers/h.ts", To: "external:zod", Kind: "type_only", Resolution: "external", Site: "pkg/handlers/h.ts:2"},
				},
			}

			_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, policy, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
		})
	})

	When("an edge is a dynamic load whose specifier is not a literal (coach#568)", func() {
		DescribeTable("emits no ProjectChange and one project_import_non_literal diagnostic naming the file, the callee, the expression, and the line",
			func(kind, wantCallee string) {
				model := domain.Model{
					ImportEdges: []domain.ImportEdge{
						{From: "file:pkg/handlers/h.ts", To: "nonliteral:modulePath", Kind: kind, Resolution: "non_literal", Site: "pkg/handlers/h.ts:7"},
					},
				}

				changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

				Expect(changes).To(BeEmpty())
				Expect(diagnostics).To(HaveLen(1))
				Expect(diagnostics[0].Kind).To(Equal(codesignal.DiagKindProjectImportNonLiteral))
				Expect(diagnostics[0].Path).To(Equal("pkg/handlers/h.ts"))
				Expect(diagnostics[0].Message).To(ContainSubstring(wantCallee))
				Expect(diagnostics[0].Message).To(ContainSubstring(`("modulePath")`))
				Expect(diagnostics[0].Message).To(ContainSubstring("did not evaluate it"))
				Expect(diagnostics[0].Location).NotTo(BeNil())
				Expect(diagnostics[0].Location.StartRow).To(Equal(uint(6)))
			},
			Entry("a dynamic_import", "dynamic_import", "import()"),
			Entry("a commonjs_require", "commonjs_require", "require()"),
		)

		It("never puts a raw control or bidirectional-override character from the source into the message, because a terminal would act on it", func() {
			hostile := "name + /* \x1b[1A\x1b[2K No active CodeSignal findings. \u202e */ other"
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "nonliteral:" + hostile, Kind: "commonjs_require", Resolution: "non_literal", Site: "pkg/handlers/h.ts:2"},
				},
			}

			_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Message).NotTo(ContainSubstring("\x1b"))
			Expect(diagnostics[0].Message).NotTo(ContainSubstring("\u202e"))
			Expect(diagnostics[0].Message).To(ContainSubstring(`\x1b[1A`), "the expression must still be readable, escaped")
		})

		It("emits nothing for an importer that sits outside every declared layer, because no rule applies to it", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/other/o.ts", To: "nonliteral:name", Kind: "commonjs_require", Resolution: "non_literal", Site: "pkg/other/o.ts:2"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
			Expect(diagnostics).To(BeEmpty())
		})
	})

	When("an edge targets a relative file that is not TypeScript source", func() {
		DescribeTable("counts it as not evaluated and never diagnoses it as unresolved, because it cannot hide a forbidden code edge",
			func(specifier string) {
				model := domain.Model{
					ImportEdges: []domain.ImportEdge{
						{From: "file:pkg/handlers/h.ts", To: "unresolved:" + specifier, Kind: "import", Resolution: "unresolved", Site: "pkg/handlers/h.ts:3"},
					},
				}

				_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

				Expect(diagnostics).To(HaveLen(1))
				Expect(diagnostics[0].Kind).To(Equal(codesignal.DiagKindProjectExternalImportsNotEvaluated))
				Expect(diagnostics[0].Message).To(Equal("1 import edge to a non-source file was not evaluated by layer rules"))
			},
			Entry("a stylesheet", "./style.css"),
			Entry("a preprocessed stylesheet", "../theme/main.SCSS"),
			Entry("a JSON file", "./data.json"),
			Entry("an image", "./logo.png"),
			Entry("a font", "../fonts/inter.woff2"),
			Entry("an asset with a bundler query", "./icon.svg?react"),
		)

		It("still diagnoses a relative specifier that could be a broken path to code", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "unresolved:./gone.js", Kind: "import", Resolution: "unresolved", Site: "pkg/handlers/h.ts:3"},
					{From: "file:pkg/handlers/h.ts", To: "unresolved:../missing/m", Kind: "import", Resolution: "unresolved", Site: "pkg/handlers/h.ts:4"},
				},
			}

			_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(HaveLen(2))
			for _, diagnostic := range diagnostics {
				Expect(diagnostic.Kind).To(Equal(codesignal.DiagKindProjectImportUnresolved))
			}
		})

		It("counts external packages and non-source files together in one diagnostic", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "external:zod", Kind: "import", Resolution: "external", Site: "pkg/handlers/h.ts:1"},
					{From: "file:pkg/handlers/h.ts", To: "unresolved:./style.css", Kind: "import", Resolution: "unresolved", Site: "pkg/handlers/h.ts:2"},
				},
			}

			_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Message).To(Equal("2 import edges to external packages or non-source files were not evaluated by layer rules"))
		})
	})

	When("edges resolve to external packages", func() {
		It("emits no ProjectChange and one aggregated informational diagnostic for a single edge", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "external:some-lib", Kind: "import", Resolution: "external", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal(codesignal.DiagKindProjectExternalImportsNotEvaluated))
			Expect(diagnostics[0].Message).To(Equal("1 import edge to an external package was not evaluated by layer rules"))
		})

		It("counts every external edge in one diagnostic, not one per edge, across kinds and files", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "external:zod", Kind: "import", Resolution: "external", Site: "pkg/handlers/h.ts:1"},
					{From: "file:pkg/handlers/h.ts", To: "external:react", Kind: "dynamic_import", Resolution: "external", Site: "pkg/handlers/h.ts:2"},
					{From: "file:pkg/db/d.ts", To: "external:pg", Kind: "commonjs_require", Resolution: "external", Site: "pkg/db/d.ts:1"},
				},
			}

			_, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, twoLayerPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Message).To(Equal("3 import edges to external packages were not evaluated by layer rules"))
		})
	})

	When("the import direction is allowed by policy", func() {
		It("emits zero ProjectChanges (negative control: allowed inter-layer import)", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
		It("emits zero ProjectChanges and only the informational external count (the file: prefix check, not layer matching, is what excludes non-file-addressed importees)", func() {
			policy := codesignal.LayerPolicy{
				Layers: []codesignal.ArchitectureLayer{
					{Name: "app", Prefixes: []string{"."}},
				},
				ForbiddenImports: []codesignal.ForbiddenLayerImport{
					{From: "app", To: "app"},
				},
			}
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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

			Expect(changes).To(BeEmpty())
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal(codesignal.DiagKindProjectExternalImportsNotEvaluated))
		})
	})

	When("the importer lacks the file: prefix but the importee carries it", func() {
		It("emits zero ProjectChanges and zero diagnostics (the importer-side file: prefix check alone excludes this edge)", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
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

	When("a layer prefix names a single TypeScript file rather than a directory", func() {
		fileLevelPolicy := func() codesignal.LayerPolicy {
			return codesignal.LayerPolicy{
				Layers: []codesignal.ArchitectureLayer{
					{Name: "entry", Prefixes: []string{"packages/core/src/index.ts"}},
					{Name: "db", Prefixes: []string{"packages/db"}},
				},
				ForbiddenImports: []codesignal.ForbiddenLayerImport{
					{From: "entry", To: "db"},
				},
			}
		}

		It("treats exactly that file as a member of the layer", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:packages/core/src/index.ts", To: "file:packages/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "packages/core/src/index.ts:3"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, fileLevelPolicy(), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].MachineEvidence).To(HaveKeyWithValue("importer", "packages/core/src/index.ts"))
			Expect(changes[0].MachineEvidence).To(HaveKeyWithValue("layer_from", "entry"))
		})

		DescribeTable("does not extend the layer to another file that merely begins with the same path",
			func(importer string) {
				model := domain.Model{
					ImportEdges: []domain.ImportEdge{
						{From: "file:" + importer, To: "file:packages/db/d.ts", Kind: "import", Resolution: "snapshot", Site: importer + ":3"},
					},
				}

				changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, fileLevelPolicy(), "1", "backend-1", "digest-1")

				Expect(diagnostics).To(BeEmpty())
				Expect(changes).To(BeEmpty())
			},
			Entry("a tsx file with the same stem", "packages/core/src/index.tsx"),
			Entry("a source map beside it", "packages/core/src/index.ts.map"),
		)
	})

	When("the policy has no layers or no forbidden imports configured", func() {
		It("returns nil, nil immediately regardless of how many edges the model has", func() {
			model := domain.Model{
				ImportEdges: []domain.ImportEdge{
					{From: "file:pkg/handlers/h.ts", To: "file:pkg/db/d.ts", Kind: "import", Resolution: "snapshot", Site: "pkg/handlers/h.ts:10"},
				},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerViolations(model, codesignal.LayerPolicy{}, "1", "backend-1", "digest-1")

			Expect(changes).To(BeNil())
			Expect(diagnostics).To(BeNil())
		})
	})
})
