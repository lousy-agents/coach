package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// layerBypassSourcePath/layerBypassSourceLine is a synthetic-but-realistic
// repository-relative position the shared layerBypassWitness helper attaches
// to a witness's Source step, standing in for the real file/line data
// BuildGoLayerBypass resolves from SSA function declarations (see
// pkg/projectmodel/go_layer_bypass_acceptance_test.go for the real-source
// variant, which pins exact values against a testdata fixture). Every other
// step -- including the sink, always a stdlib function with no local
// declaration -- is left unpositioned, matching what BuildGoLayerBypass
// actually produces.
const (
	layerBypassSourcePath = "pkg/handlers/handlers.go"
	layerBypassSourceLine = 3
)

// layerBypassWitness builds a synthetic LayerBypassWitness the way
// BuildGoLayerBypass would (see pkg/projectmodel/go_layer_bypass.go), without
// needing real Go source: Task A already proved the evaluator itself.
func layerBypassWitness(source, sink, requiredLayer string, path []string) projectmodel.LayerBypassWitness {
	steps := make([]projectmodel.LayerBypassStep, len(path))
	for i, nodeID := range path {
		steps[i] = projectmodel.LayerBypassStep{NodeID: nodeID}
	}
	if len(steps) > 0 {
		steps[0].Path = layerBypassSourcePath
		steps[0].Line = layerBypassSourceLine
	}
	return projectmodel.LayerBypassWitness{
		ID:               "bypass:" + requiredLayer + ":" + source + "->" + sink + "@" + projectmodel.LayerBypassAlgorithm,
		Source:           source,
		Sink:             sink,
		RequiredLayer:    requiredLayer,
		Path:             steps,
		Confidence:       projectmodel.LayerBypassConfidenceHigh,
		AlgorithmVersion: projectmodel.LayerBypassAlgorithm,
	}
}

// layerBypassResult wraps witnesses in a LayerBypassResult with a complete
// Coverage, mirroring what a genuinely finished BuildGoLayerBypass search
// reports -- most specs in this file are not exercising Coverage itself, so
// they opt into "search completed" explicitly rather than relying on
// Coverage's own zero value (Complete: false), which would otherwise trip
// the project_layer_bypass_coverage_incomplete diagnostic on every spec.
func layerBypassResult(witnesses ...projectmodel.LayerBypassWitness) projectmodel.LayerBypassResult {
	return projectmodel.LayerBypassResult{
		Witnesses: witnesses,
		Algorithm: projectmodel.LayerBypassAlgorithm,
		Coverage:  projectmodel.Coverage{Phase: "go_layer_bypass", Complete: true},
	}
}

var _ = Describe("EvaluateGoLayerBypass", func() {
	When("a single high-confidence witness is supplied", func() {
		It("emits exactly one architecture.layer_bypass ProjectChange anchored on the witness's real source position", func() {
			witness := projectmodel.LayerBypassWitness{
				ID:            "bypass:service:example.com/app/handlers.Handler->(*database/sql.DB).Query@" + projectmodel.LayerBypassAlgorithm,
				Source:        "example.com/app/handlers.Handler",
				Sink:          "(*database/sql.DB).Query",
				RequiredLayer: "service",
				Path: []projectmodel.LayerBypassStep{
					{NodeID: "example.com/app/handlers.Handler", Path: "pkg/handlers/handlers.go", Line: 3},
					{NodeID: "example.com/app/direct.Query", Path: "pkg/direct/direct.go", Line: 10},
					{NodeID: "(*database/sql.DB).Query"},
				},
				Confidence:       projectmodel.LayerBypassConfidenceHigh,
				AlgorithmVersion: projectmodel.LayerBypassAlgorithm,
			}
			result := layerBypassResult(witness)

			changes, diagnostics := codesignal.EvaluateGoLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			change := changes[0]

			Expect(change.RuleID).To(Equal("architecture.layer_bypass"))
			Expect(change.Kind).To(Equal("architecture.layer_bypass"))
			Expect(change.Category).To(Equal(codesignal.Category("architecture")))
			Expect(change.Severity).To(Equal(codesignal.Severity("advisory")))
			Expect(change.Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(change.RuleVersion).To(Equal("1"))
			Expect(change.BackendVersion).To(Equal("backend-1"))
			Expect(change.ConfigDigest).To(Equal("digest-1"))
			Expect(change.AlgorithmVersion).To(Equal(projectmodel.LayerBypassAlgorithm))
			Expect(change.SemanticKey).To(Equal(
				"architecture.layer_bypass:service:example.com/app/handlers.Handler->(*database/sql.DB).Query",
			))

			// False-green control: these values are tied to the specific input
			// witness, not merely "MachineEvidence is non-empty". The SSA node
			// identity intentionally still lives here even though it is no longer
			// on PrimaryAnchor -- see EvaluateGoLayerBypass's doc comment.
			Expect(change.MachineEvidence).To(Equal(map[string]string{
				"source":         "example.com/app/handlers.Handler",
				"sink":           "(*database/sql.DB).Query",
				"required_layer": "service",
				"path":           "example.com/app/handlers.Handler->example.com/app/direct.Query->(*database/sql.DB).Query",
			}))

			Expect(change.PathSteps).To(Equal([]codesignal.ProjectPathStep{
				{
					NodeID:          "example.com/app/handlers.Handler",
					Confidence:      codesignal.Confidence("high"),
					SourceLocations: []codesignal.ProjectLocation{{Path: "pkg/handlers/handlers.go", Location: semantics.Location{StartRow: 2}}},
				},
				{
					NodeID:          "example.com/app/direct.Query",
					Confidence:      codesignal.Confidence("high"),
					SourceLocations: []codesignal.ProjectLocation{{Path: "pkg/direct/direct.go", Location: semantics.Location{StartRow: 9}}},
				},
				{
					NodeID:     "(*database/sql.DB).Query",
					Confidence: codesignal.Confidence("high"),
				},
			}))

			// Anchored on the witness's real source position (Task A's SSA
			// declaration position, converted 1-based Line -> 0-based StartRow,
			// exactly as parseSiteLocation does for architecture.layer_violation),
			// never the SSA function identity string -- see Implementer Report.
			Expect(change.PrimaryAnchor).To(Equal(codesignal.ProjectLocation{
				Path:     "pkg/handlers/handlers.go",
				Location: semantics.Location{StartRow: 2},
			}))

			Expect(change.CausalEvidenceDigest).NotTo(BeEmpty())
			Expect(change.WhyItMatters).NotTo(BeEmpty())
			Expect(change.Recommendation).NotTo(BeEmpty())
			Expect(change.Provenance).To(Equal(codesignal.Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_bypass"}))
		})
	})

	When("the source step's position is unresolvable but a later step's is not", func() {
		It("anchors on the first path step with a resolvable position instead of fabricating one", func() {
			witness := projectmodel.LayerBypassWitness{
				ID:            "bypass:service:example.com/app/handlers.Handler->(*database/sql.DB).Query@" + projectmodel.LayerBypassAlgorithm,
				Source:        "example.com/app/handlers.Handler",
				Sink:          "(*database/sql.DB).Query",
				RequiredLayer: "service",
				Path: []projectmodel.LayerBypassStep{
					{NodeID: "example.com/app/handlers.Handler"}, // unresolvable, e.g. a synthetic wrapper
					{NodeID: "example.com/app/direct.Query", Path: "pkg/direct/direct.go", Line: 10},
					{NodeID: "(*database/sql.DB).Query"},
				},
				Confidence:       projectmodel.LayerBypassConfidenceHigh,
				AlgorithmVersion: projectmodel.LayerBypassAlgorithm,
			}

			changes, diagnostics := codesignal.EvaluateGoLayerBypass(layerBypassResult(witness), "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].PrimaryAnchor).To(Equal(codesignal.ProjectLocation{
				Path:     "pkg/direct/direct.go",
				Location: semantics.Location{StartRow: 9},
			}))
		})
	})

	When("no path step has a resolvable position", func() {
		It("leaves PrimaryAnchor empty rather than fabricating one, so Build's shared anchorless filter drops the change", func() {
			witness := projectmodel.LayerBypassWitness{
				ID:            "bypass:service:example.com/app/handlers.Handler->(*database/sql.DB).Query@" + projectmodel.LayerBypassAlgorithm,
				Source:        "example.com/app/handlers.Handler",
				Sink:          "(*database/sql.DB).Query",
				RequiredLayer: "service",
				Path: []projectmodel.LayerBypassStep{
					{NodeID: "example.com/app/handlers.Handler"},
					{NodeID: "(*database/sql.DB).Query"},
				},
				Confidence:       projectmodel.LayerBypassConfidenceHigh,
				AlgorithmVersion: projectmodel.LayerBypassAlgorithm,
			}

			changes, diagnostics := codesignal.EvaluateGoLayerBypass(layerBypassResult(witness), "1", "backend-1", "digest-1")
			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].PrimaryAnchor).To(Equal(codesignal.ProjectLocation{}))

			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:  changes,
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			Expect(report.ProjectChanges).To(BeEmpty())
			found := false
			for _, d := range report.Diagnostics {
				if d.Kind == "project_observation_missing_primary_path" {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected a project_observation_missing_primary_path diagnostic, got %+v", report.Diagnostics)
		})
	})

	When("BuildGoLayerBypass's search coverage was incomplete", func() {
		It("still emits the witness's ProjectChange but also a coverage-incomplete diagnostic", func() {
			witness := layerBypassWitness(
				"example.com/app/handlers.Handler",
				"(*database/sql.DB).Query",
				"service",
				[]string{"example.com/app/handlers.Handler", "(*database/sql.DB).Query"},
			)
			result := projectmodel.LayerBypassResult{
				Witnesses: []projectmodel.LayerBypassWitness{witness},
				Algorithm: projectmodel.LayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "go_layer_bypass", Complete: false},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(changes).To(HaveLen(1))
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal("project_layer_bypass_coverage_incomplete"))
		})
	})

	When("BuildGoLayerBypass's search coverage was incomplete and found zero witnesses", func() {
		It("still emits the coverage-incomplete diagnostic, so the absence is not read as a sound 'no bypass' claim", func() {
			result := projectmodel.LayerBypassResult{
				Algorithm: projectmodel.LayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "go_layer_bypass", Complete: false},
			}

			changes, diagnostics := codesignal.EvaluateGoLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal("project_layer_bypass_coverage_incomplete"))
		})
	})

	When("no witnesses are supplied", func() {
		It("emits zero ProjectChanges (negative control)", func() {
			result := layerBypassResult()

			changes, diagnostics := codesignal.EvaluateGoLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	When("a witness's Confidence is not LayerBypassConfidenceHigh", func() {
		It("emits zero ProjectChanges even though the witness would otherwise map cleanly (defensive re-check, not decoration)", func() {
			witness := layerBypassWitness(
				"example.com/app/handlers.Handler",
				"(*database/sql.DB).Query",
				"service",
				[]string{"example.com/app/handlers.Handler", "(*database/sql.DB).Query"},
			)
			witness.Confidence = "medium" // BuildGoLayerBypass never produces this; constructed directly to exercise the guard.
			result := layerBypassResult(witness)

			changes, _ := codesignal.EvaluateGoLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
		})
	})

	When("two witnesses under the same required layer cover different source/sink pairs, supplied out of order", func() {
		It("emits two ProjectChanges deterministically ordered by (source, sink)", func() {
			witnessZ := layerBypassWitness(
				"example.com/app/handlers.ZHandler",
				"(*database/sql.DB).Query",
				"service",
				[]string{"example.com/app/handlers.ZHandler", "(*database/sql.DB).Query"},
			)
			witnessA := layerBypassWitness(
				"example.com/app/handlers.AHandler",
				"(*database/sql.DB).Exec",
				"service",
				[]string{"example.com/app/handlers.AHandler", "(*database/sql.DB).Exec"},
			)
			result := layerBypassResult(witnessZ, witnessA)

			changes, diagnostics := codesignal.EvaluateGoLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(2))
			Expect(changes[0].SemanticKey).To(Equal(
				"architecture.layer_bypass:service:example.com/app/handlers.AHandler->(*database/sql.DB).Exec",
			))
			Expect(changes[1].SemanticKey).To(Equal(
				"architecture.layer_bypass:service:example.com/app/handlers.ZHandler->(*database/sql.DB).Query",
			))
		})
	})

	Describe("lifecycle classification via the shared Build entrypoint", func() {
		coverage := func() *projectmodel.Coverage {
			return &projectmodel.Coverage{Phase: "full", Complete: true}
		}

		When("the identical witness (same path) is present on both head and base", func() {
			It("classifies existing with Changed false", func() {
				witness := layerBypassWitness(
					"example.com/app/handlers.Handler",
					"(*database/sql.DB).Query",
					"service",
					[]string{"example.com/app/handlers.Handler", "example.com/app/direct.Query", "(*database/sql.DB).Query"},
				)
				result := layerBypassResult(witness)
				changes, _ := codesignal.EvaluateGoLayerBypass(result, "1", "backend-1", "digest-1")

				report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
					ProjectChanges:      changes,
					BaseProjectChanges:  changes,
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     coverage(),
					BaseProjectCoverage: coverage(),
				})

				Expect(report.ProjectChanges).To(HaveLen(1))
				Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
				Expect(report.ProjectChanges[0].Changed).To(BeFalse())
			})
		})

		When("the same (source, sink, required layer) has a different intermediate path on head vs. base", func() {
			It("keeps identity/fingerprint stable but marks Changed true", func() {
				headWitness := layerBypassWitness(
					"example.com/app/handlers.Handler",
					"(*database/sql.DB).Query",
					"service",
					[]string{"example.com/app/handlers.Handler", "example.com/app/direct.Query", "(*database/sql.DB).Query"},
				)
				baseWitness := layerBypassWitness(
					"example.com/app/handlers.Handler",
					"(*database/sql.DB).Query",
					"service",
					[]string{"example.com/app/handlers.Handler", "example.com/app/other.Route", "(*database/sql.DB).Query"},
				)
				headChanges, _ := codesignal.EvaluateGoLayerBypass(layerBypassResult(headWitness), "1", "backend-1", "digest-1")
				baseChanges, _ := codesignal.EvaluateGoLayerBypass(layerBypassResult(baseWitness), "1", "backend-1", "digest-1")

				combined := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
					ProjectChanges:      headChanges,
					BaseProjectChanges:  baseChanges,
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     coverage(),
					BaseProjectCoverage: coverage(),
				})
				Expect(combined.ProjectChanges).To(HaveLen(1))
				Expect(combined.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
				Expect(combined.ProjectChanges[0].Changed).To(BeTrue())

				// Identity (Fingerprint) does not depend on the concrete path, so
				// classifying each side independently must yield the same value --
				// asserted alongside Changed==true above so neither half can pass alone.
				headOnly := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
					ProjectChanges: headChanges, ProjectCoverage: coverage(),
				})
				baseOnly := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
					ProjectChanges: baseChanges, ProjectCoverage: coverage(),
				})
				Expect(headOnly.ProjectChanges[0].Fingerprint).To(Equal(baseOnly.ProjectChanges[0].Fingerprint))
			})
		})

		When("a witness is present only on head", func() {
			It("classifies introduced", func() {
				witness := layerBypassWitness("example.com/app/handlers.Handler", "(*database/sql.DB).Query", "service",
					[]string{"example.com/app/handlers.Handler", "(*database/sql.DB).Query"})
				headChanges, _ := codesignal.EvaluateGoLayerBypass(layerBypassResult(witness), "1", "backend-1", "digest-1")

				report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
					ProjectChanges:      headChanges,
					BaseProjectChanges:  nil,
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     coverage(),
					BaseProjectCoverage: coverage(),
				})

				Expect(report.ProjectChanges).To(HaveLen(1))
				Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			})
		})

		When("a witness is present only on base", func() {
			It("classifies resolved", func() {
				witness := layerBypassWitness("example.com/app/handlers.Handler", "(*database/sql.DB).Query", "service",
					[]string{"example.com/app/handlers.Handler", "(*database/sql.DB).Query"})
				baseChanges, _ := codesignal.EvaluateGoLayerBypass(layerBypassResult(witness), "1", "backend-1", "digest-1")

				report := build(codesignal.Options{ProjectEnabled: true, IncludeResolved: true}, codesignal.Input{
					BaseProjectChanges:  baseChanges,
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     coverage(),
					BaseProjectCoverage: coverage(),
				})

				Expect(report.ProjectChanges).To(HaveLen(1))
				Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
			})
		})

		When("there are no witnesses on either side", func() {
			It("reports zero active project changes (zero-witness negative control)", func() {
				noWitnesses := layerBypassResult()
				changes, diagnostics := codesignal.EvaluateGoLayerBypass(noWitnesses, "1", "backend-1", "digest-1")
				Expect(diagnostics).To(BeEmpty())
				Expect(changes).To(BeEmpty())

				report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
					ProjectChanges:  changes,
					ProjectCoverage: coverage(),
				})

				Expect(report.ProjectChanges).To(BeEmpty())
				Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
				Expect(report.ProjectSummary.ActiveChanges).To(Equal(0))
			})
		})

		When("a witness is present in a baseline run", func() {
			It("classifies baseline with zero base comparison, not existing/introduced/resolved", func() {
				witness := layerBypassWitness("example.com/app/handlers.Handler", "(*database/sql.DB).Query", "service",
					[]string{"example.com/app/handlers.Handler", "(*database/sql.DB).Query"})
				changes, diagnostics := codesignal.EvaluateGoLayerBypass(layerBypassResult(witness), "1", "backend-1", "digest-1")
				Expect(diagnostics).To(BeEmpty())
				Expect(changes).To(HaveLen(1))

				report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
					ProjectChanges:  changes,
					ProjectCoverage: coverage(),
				})

				Expect(report.ProjectChanges).To(HaveLen(1))
				Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
				Expect(report.ProjectChanges[0].Changed).To(BeFalse())
				Expect(report.ProjectSummary.BaselineChanges).To(Equal(1))
				Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
			})
		})
	})
})

var _ = Describe("EvaluateTypeScriptLayerBypass", func() {
	When("a single high-confidence TS witness is supplied", func() {
		It("emits exactly one architecture.layer_bypass ProjectChange tagged with language typescript", func() {
			witness := projectmodel.LayerBypassWitness{
				ID:            "bypass:service:file:src/handlers/app.ts#getUsers->(PrismaClient).findMany@" + projectmodel.TSLayerBypassAlgorithm,
				Source:        "file:src/handlers/app.ts#getUsers",
				Sink:          "(PrismaClient).findMany",
				RequiredLayer: "service",
				Path: []projectmodel.LayerBypassStep{
					{NodeID: "file:src/handlers/app.ts#getUsers", Path: "src/handlers/app.ts"},
					{NodeID: "(PrismaClient).findMany"},
				},
				Confidence:       projectmodel.LayerBypassConfidenceHigh,
				AlgorithmVersion: projectmodel.TSLayerBypassAlgorithm,
			}
			result := projectmodel.LayerBypassResult{
				Witnesses: []projectmodel.LayerBypassWitness{witness},
				Algorithm: projectmodel.TSLayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_layer_bypass", Complete: true},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(HaveLen(1))
			change := changes[0]

			Expect(change.RuleID).To(Equal("architecture.layer_bypass"))
			Expect(change.Kind).To(Equal("architecture.layer_bypass"))
			Expect(change.Category).To(Equal(codesignal.Category("architecture")))
			Expect(change.Severity).To(Equal(codesignal.Severity("advisory")))
			Expect(change.Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(change.AlgorithmVersion).To(Equal(projectmodel.TSLayerBypassAlgorithm))
			Expect(change.SemanticKey).To(Equal(
				"architecture.layer_bypass:service:file:src/handlers/app.ts#getUsers->(PrismaClient).findMany",
			))

			// False-green control: MachineEvidence["language"] is TS-only --
			// EvaluateGoLayerBypass's own spec above asserts an exact
			// MachineEvidence map with no "language" key at all, so a shared
			// builder that always added the key (rather than only when
			// non-empty) would fail that Go spec, not this one.
			Expect(change.MachineEvidence).To(Equal(map[string]string{
				"source":         "file:src/handlers/app.ts#getUsers",
				"sink":           "(PrismaClient).findMany",
				"required_layer": "service",
				"path":           "file:src/handlers/app.ts#getUsers->(PrismaClient).findMany",
				"language":       "typescript",
			}))

			Expect(change.PrimaryAnchor).To(Equal(codesignal.ProjectLocation{Path: "src/handlers/app.ts"}))
			Expect(change.Provenance).To(Equal(codesignal.Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_bypass"}))
		})
	})

	When("a TS witness's Confidence is not LayerBypassConfidenceHigh", func() {
		It("emits zero ProjectChanges (defensive re-check, not decoration)", func() {
			witness := projectmodel.LayerBypassWitness{
				ID:               "bypass:service:file:src/handlers/app.ts#getUsers->(PrismaClient).findMany@" + projectmodel.TSLayerBypassAlgorithm,
				Source:           "file:src/handlers/app.ts#getUsers",
				Sink:             "(PrismaClient).findMany",
				RequiredLayer:    "service",
				Path:             []projectmodel.LayerBypassStep{{NodeID: "file:src/handlers/app.ts#getUsers"}, {NodeID: "(PrismaClient).findMany"}},
				Confidence:       "medium", // BuildTypeScriptLayerBypass never produces this; constructed directly to exercise the guard.
				AlgorithmVersion: projectmodel.TSLayerBypassAlgorithm,
			}
			result := projectmodel.LayerBypassResult{
				Witnesses: []projectmodel.LayerBypassWitness{witness},
				Algorithm: projectmodel.TSLayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_layer_bypass", Complete: true},
			}

			changes, _ := codesignal.EvaluateTypeScriptLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
		})
	})

	When("BuildTypeScriptLayerBypass's search coverage was incomplete", func() {
		It("still emits the witness's ProjectChange but also a coverage-incomplete diagnostic", func() {
			witness := projectmodel.LayerBypassWitness{
				ID:               "bypass:service:file:src/handlers/app.ts#getUsers->(PrismaClient).findMany@" + projectmodel.TSLayerBypassAlgorithm,
				Source:           "file:src/handlers/app.ts#getUsers",
				Sink:             "(PrismaClient).findMany",
				RequiredLayer:    "service",
				Path:             []projectmodel.LayerBypassStep{{NodeID: "file:src/handlers/app.ts#getUsers"}, {NodeID: "(PrismaClient).findMany"}},
				Confidence:       projectmodel.LayerBypassConfidenceHigh,
				AlgorithmVersion: projectmodel.TSLayerBypassAlgorithm,
			}
			result := projectmodel.LayerBypassResult{
				Witnesses: []projectmodel.LayerBypassWitness{witness},
				Algorithm: projectmodel.TSLayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_layer_bypass", Complete: false},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(changes).To(HaveLen(1))
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal("project_layer_bypass_coverage_incomplete"))
		})
	})

	When("the required layer was ambiguous, so BuildTypeScriptLayerBypass found zero witnesses", func() {
		It("emits zero ProjectChanges and a coverage-incomplete diagnostic, matching an ambiguous-layer run's Coverage.Complete false", func() {
			result := projectmodel.LayerBypassResult{
				Algorithm: projectmodel.TSLayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_layer_bypass", Complete: false},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(changes).To(BeEmpty())
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal("project_layer_bypass_coverage_incomplete"))
		})
	})

	When("no witnesses are supplied", func() {
		It("emits zero ProjectChanges (negative control)", func() {
			result := projectmodel.LayerBypassResult{
				Algorithm: projectmodel.TSLayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_layer_bypass", Complete: true},
			}

			changes, diagnostics := codesignal.EvaluateTypeScriptLayerBypass(result, "1", "backend-1", "digest-1")

			Expect(diagnostics).To(BeEmpty())
			Expect(changes).To(BeEmpty())
		})
	})

	Describe("lifecycle classification via the shared Build entrypoint", func() {
		coverage := func() *projectmodel.Coverage {
			return &projectmodel.Coverage{Phase: "full", Complete: true}
		}

		tsWitness := func(source, sink, requiredLayer string, path []string) projectmodel.LayerBypassWitness {
			steps := make([]projectmodel.LayerBypassStep, len(path))
			for i, nodeID := range path {
				steps[i] = projectmodel.LayerBypassStep{NodeID: nodeID}
			}
			if len(steps) > 0 {
				steps[0].Path = layerBypassSourcePath
			}
			return projectmodel.LayerBypassWitness{
				ID:               "bypass:" + requiredLayer + ":" + source + "->" + sink + "@" + projectmodel.TSLayerBypassAlgorithm,
				Source:           source,
				Sink:             sink,
				RequiredLayer:    requiredLayer,
				Path:             steps,
				Confidence:       projectmodel.LayerBypassConfidenceHigh,
				AlgorithmVersion: projectmodel.TSLayerBypassAlgorithm,
			}
		}
		tsResult := func(witnesses ...projectmodel.LayerBypassWitness) projectmodel.LayerBypassResult {
			return projectmodel.LayerBypassResult{
				Witnesses: witnesses,
				Algorithm: projectmodel.TSLayerBypassAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_layer_bypass", Complete: true},
			}
		}

		When("a TS witness is present only on head", func() {
			It("classifies introduced, sharing the ruleLayerBypassID vocabulary with the Go evaluator", func() {
				witness := tsWitness("file:src/handlers/app.ts#getUsers", "(PrismaClient).findMany", "service",
					[]string{"file:src/handlers/app.ts#getUsers", "(PrismaClient).findMany"})
				headChanges, _ := codesignal.EvaluateTypeScriptLayerBypass(tsResult(witness), "1", "backend-1", "digest-1")

				report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
					ProjectChanges:      headChanges,
					BaseProjectChanges:  nil,
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     coverage(),
					BaseProjectCoverage: coverage(),
				})

				Expect(report.ProjectChanges).To(HaveLen(1))
				Expect(report.ProjectChanges[0].RuleID).To(Equal("architecture.layer_bypass"))
				Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			})
		})

		When("the identical TS witness is present on both head and base", func() {
			It("classifies existing with Changed false", func() {
				witness := tsWitness("file:src/handlers/app.ts#getUsers", "(PrismaClient).findMany", "service",
					[]string{"file:src/handlers/app.ts#getUsers", "(PrismaClient).findMany"})
				result := tsResult(witness)
				changes, _ := codesignal.EvaluateTypeScriptLayerBypass(result, "1", "backend-1", "digest-1")

				report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
					ProjectChanges:      changes,
					BaseProjectChanges:  changes,
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     coverage(),
					BaseProjectCoverage: coverage(),
				})

				Expect(report.ProjectChanges).To(HaveLen(1))
				Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
				Expect(report.ProjectChanges[0].Changed).To(BeFalse())
			})
		})
	})
})
