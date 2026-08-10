package codesignal_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func projectChange(key, ruleID string) codesignal.ProjectChange {
	return codesignal.ProjectChange{
		SemanticKey: key,
		RuleID:      ruleID,
		RuleVersion: "1",
		Kind:        "cycle",
		Category:    codesignal.Category("architecture"),
		Severity:    codesignal.Severity("medium"),
		Confidence:  codesignal.Confidence("high"),
		PrimaryAnchor: codesignal.ProjectLocation{
			Path:     "pkg/a/a.go",
			Location: semantics.Location{StartRow: 1},
		},
		Evidence:   "pkg/a -> pkg/b -> pkg/a",
		Provenance: codesignal.Provenance{Producer: "projectmodel"},
	}
}

var _ = Describe("Project-analysis report generation", func() {
	When("ProjectEnabled is false (the default)", func() {
		It("produces a byte-identical schema-1 report even when project data is supplied on Input", func() {
			input := codesignal.Input{
				Files:              []codesignal.FileChange{{Path: "a.go", Status: "modified", Head: cleanResult("a.go", mutation("Update", 1))}},
				ProjectChanges:     []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				BaseProjectChanges: []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage:    &projectmodel.Coverage{Phase: "full", Complete: true},
			}

			withoutProjectData := build(codesignal.Options{}, codesignal.Input{Files: input.Files})
			withProjectData := build(codesignal.Options{}, input)

			Expect(withProjectData.SchemaVersion).To(Equal("1"))

			leftJSON, err := json.Marshal(withoutProjectData)
			Expect(err).NotTo(HaveOccurred())
			rightJSON, err := json.Marshal(withProjectData)
			Expect(err).NotTo(HaveOccurred())
			Expect(rightJSON).To(Equal(leftJSON))

			var raw map[string]json.RawMessage
			Expect(json.Unmarshal(rightJSON, &raw)).To(Succeed())
			Expect(raw).NotTo(HaveKey("project_changes"))
			Expect(raw).NotTo(HaveKey("project_facts"))
			Expect(raw).NotTo(HaveKey("project_summary"))
			Expect(raw).NotTo(HaveKey("project_coverage"))
		})
	})

	When("ProjectEnabled is true with head-only project changes and no base supplied", func() {
		It("classifies changes unknown, assigns stable identity, and reports schema 2", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.ProjectChanges).To(HaveLen(1))
			change := report.ProjectChanges[0]
			Expect(change.ID).NotTo(BeEmpty())
			Expect(change.Fingerprint).NotTo(BeEmpty())
			Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))

			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0))
			Expect(report.ProjectSummary.ExistingChanges).To(Equal(0))
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(0))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
		})
	})

	When("ProjectEnabled is true and enabled with no project data at all", func() {
		It("still reports schema 2 with an all-zero ProjectSummary and no project_changes/project_coverage keys", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{})

			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.ProjectChanges).To(BeEmpty())
			Expect(report.ProjectCoverage).To(BeNil())
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(*report.ProjectSummary).To(Equal(codesignal.ProjectSummary{}))

			raw, err := json.Marshal(report)
			Expect(err).NotTo(HaveOccurred())
			var fields map[string]json.RawMessage
			Expect(json.Unmarshal(raw, &fields)).To(Succeed())
			Expect(fields).NotTo(HaveKey("project_changes"))
			Expect(fields).NotTo(HaveKey("project_coverage"))
			Expect(fields).To(HaveKey("project_summary"))
		})
	})

	Describe("Project change lifecycle classification", func() {
		It("marks a key present on both sides existing", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(report.ProjectSummary.ExistingChanges).To(Equal(1))
		})

		It("marks a key present only on head introduced when a base was supplied, while still counting the base-only key resolved", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/c<->pkg/d", "project.import_cycle")},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].SemanticKey).To(Equal("cycle:pkg/a<->pkg/b"))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(1))
		})

		It("hides a base-only key by default but still counts it resolved", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(BeEmpty())
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(1))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(0))
		})

		It("surfaces a base-only key resolved when IncludeResolved is set", func() {
			report := build(codesignal.Options{ProjectEnabled: true, IncludeResolved: true}, codesignal.Input{
				BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(1))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
		})

		It("marks a head-only change introduced when the base was analyzed and legitimately produced zero changes", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				BaseProjectChanges:  nil,
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))
		})

		It("marks head-only changes baseline when Options.Baseline is set and head coverage is complete", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(1))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
		})

		// Callers commonly initialize BaseProjectChanges as a non-nil empty
		// slice (make/append). That must not be treated as "base side present"
		// when ProjectBaseAnalyzed is false — otherwise baseline lifecycle is
		// silently suppressed for every complete baseline run.
		It("still claims baseline when BaseProjectChanges is a non-nil empty slice and no base was analyzed", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				BaseProjectChanges:  []codesignal.ProjectChange{},
				ProjectBaseAnalyzed: false,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(1))
			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})

		It("does not claim baseline when head coverage is nil", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges: []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})

		It("does not claim baseline when head coverage is incomplete", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})

		// Baseline has no base model. The indeterminate diagnostic must name
		// only the head-side coverage failure, not invent "base coverage
		// unavailable" for a side that was never expected.
		It("names only head coverage in the indeterminate diagnostic for an incomplete baseline run", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
			})
			var message string
			for _, d := range report.Diagnostics {
				if d.Kind == "project_lifecycle_indeterminate" {
					message = d.Message
					break
				}
			}
			Expect(message).NotTo(BeEmpty())
			Expect(message).To(ContainSubstring("head coverage incomplete"))
			Expect(message).NotTo(ContainSubstring("base coverage"))
		})

		It("does not claim introduced/existing when base coverage is incomplete", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(report.ProjectSummary.ExistingChanges).To(Equal(0))
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})
	})

	// F-003: active project observations must appear on the shared signals
	// surface and normal summary counters; facts-only stay outside both.
	Describe("project observations on the shared signals surface", func() {
		It("maps an active project observation into signals and summary counters while facts stay facts-only", func() {
			active := projectChange("cycle:pkg/a<->pkg/b", "architecture.layer_violation")
			active.RelatedLocations = []codesignal.ProjectLocation{
				{Path: "pkg/b/b.go", Location: semantics.Location{StartRow: 4}},
			}
			fact := codesignal.ProjectFact{
				Kind:        "possible_call_reachability",
				SemanticKey: "reach:handler->query",
				Evidence:    "Handler may reach Query",
				Provenance:  codesignal.Provenance{Producer: "projectmodel"},
			}
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{active},
				ProjectFacts:    []codesignal.ProjectFact{fact},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			Expect(report.Signals).To(HaveLen(1))
			sig := report.Signals[0]
			Expect(sig.RuleID).To(Equal("architecture.layer_violation"))
			Expect(sig.Path).To(Equal("pkg/a/a.go"))
			Expect(sig.Subject).To(Equal("cycle:pkg/a<->pkg/b"))
			Expect(sig.Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(report.Summary.ActiveSignals).To(Equal(1))
			Expect(report.Summary.BaselineSignals).To(Equal(1))

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectFacts).To(HaveLen(1))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(1))

			// Facts must not inflate active signal counters.
			factsOnly := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectFacts:    []codesignal.ProjectFact{fact},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(factsOnly.Signals).To(BeEmpty())
			Expect(factsOnly.Summary.ActiveSignals).To(Equal(0))
			Expect(factsOnly.ProjectFacts).To(HaveLen(1))
		})
	})

	// F-004: same-kind facts with missing/duplicate keys and reversed coverage
	// diagnostics must still marshal byte-identically.
	Describe("schema-2 canonical ordering at the report boundary", func() {
		It("produces byte-identical JSON for reverse-ordered facts and coverage diagnostics", func() {
			factA := codesignal.ProjectFact{
				Kind:       "possible_call_reachability",
				Evidence:   "a",
				Provenance: codesignal.Provenance{Producer: "projectmodel"},
			}
			factB := codesignal.ProjectFact{
				Kind:       "possible_call_reachability",
				Evidence:   "b",
				Provenance: codesignal.Provenance{Producer: "projectmodel"},
			}
			covLeft := &projectmodel.Coverage{
				Phase:    "full",
				Complete: false,
				Diagnostics: []projectmodel.Diagnostic{
					{Code: "z_code", Message: "z", Path: "z.go"},
					{Code: "a_code", Message: "a", Path: "a.go"},
				},
			}
			covRight := &projectmodel.Coverage{
				Phase:    "full",
				Complete: false,
				Diagnostics: []projectmodel.Diagnostic{
					{Code: "a_code", Message: "a", Path: "a.go"},
					{Code: "z_code", Message: "z", Path: "z.go"},
				},
			}
			left := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectFacts:    []codesignal.ProjectFact{factB, factA},
				ProjectCoverage: covLeft,
			})
			right := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectFacts:    []codesignal.ProjectFact{factA, factB},
				ProjectCoverage: covRight,
			})
			leftJSON, err := json.Marshal(left)
			Expect(err).NotTo(HaveOccurred())
			rightJSON, err := json.Marshal(right)
			Expect(err).NotTo(HaveOccurred())
			Expect(leftJSON).To(Equal(rightJSON))
			Expect(left.ProjectCoverage.Diagnostics[0].Code).To(Equal("a_code"))
		})
	})

	Describe("facts-only project observations", func() {
		It("serializes facts in project_facts without active project_changes or summary counters", func() {
			fact := codesignal.ProjectFact{
				Kind:        "possible_call_reachability",
				SemanticKey: "reach:handler->query",
				PathSteps: []codesignal.ProjectPathStep{{
					NodeID:      "func:Handler",
					DisplayName: "Handler",
					Resolution:  "static",
					Confidence:  codesignal.Confidence("medium"),
				}},
				CoverageRefs: []string{"call_graph"},
				Evidence:     "Handler may reach Query",
				Provenance:   codesignal.Provenance{Producer: "projectmodel", FindingKind: "possible_call_reachability"},
			}
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectFacts:    []codesignal.ProjectFact{fact},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			Expect(report.ProjectFacts).To(HaveLen(1))
			Expect(report.ProjectFacts[0].Kind).To(Equal("possible_call_reachability"))
			Expect(report.ProjectChanges).To(BeEmpty())
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(0))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
			Expect(report.Summary.ActiveSignals).To(Equal(0))

			raw, err := json.Marshal(report)
			Expect(err).NotTo(HaveOccurred())
			var fields map[string]json.RawMessage
			Expect(json.Unmarshal(raw, &fields)).To(Succeed())
			Expect(fields).To(HaveKey("project_facts"))
			Expect(fields).NotTo(HaveKey("project_changes"))

			first, err := json.Marshal(report)
			Expect(err).NotTo(HaveOccurred())
			second, err := json.Marshal(report)
			Expect(err).NotTo(HaveOccurred())
			Expect(first).To(Equal(second))
		})

		It("sorts facts deterministically by kind then semantic key", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectFacts: []codesignal.ProjectFact{
					{Kind: "possible_call_reachability", SemanticKey: "z", Provenance: codesignal.Provenance{Producer: "projectmodel"}},
					{Kind: "possible_call_reachability", SemanticKey: "a", Provenance: codesignal.Provenance{Producer: "projectmodel"}},
					{Kind: "other_fact", SemanticKey: "m", Provenance: codesignal.Provenance{Producer: "projectmodel"}},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectFacts).To(HaveLen(3))
			Expect(report.ProjectFacts[0].Kind).To(Equal("other_fact"))
			Expect(report.ProjectFacts[1].SemanticKey).To(Equal("a"))
			Expect(report.ProjectFacts[2].SemanticKey).To(Equal("z"))
		})
	})

	Describe("Project-analysis JSON round-trip", func() {
		It("preserves project fields across marshal/unmarshal", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectFacts:    []codesignal.ProjectFact{{Kind: "possible_call_reachability", SemanticKey: "reach:a->b", Provenance: codesignal.Provenance{Producer: "projectmodel"}}},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			raw, err := json.Marshal(report)
			Expect(err).NotTo(HaveOccurred())

			var roundTripped codesignal.Report
			Expect(json.Unmarshal(raw, &roundTripped)).To(Succeed())
			Expect(roundTripped.SchemaVersion).To(Equal("2"))
			Expect(roundTripped.ProjectChanges).To(HaveLen(1))
			Expect(roundTripped.ProjectChanges[0].SemanticKey).To(Equal("cycle:pkg/a<->pkg/b"))
			Expect(roundTripped.ProjectFacts).To(HaveLen(1))
			Expect(roundTripped.ProjectFacts[0].Kind).To(Equal("possible_call_reachability"))
			Expect(roundTripped.ProjectCoverage).NotTo(BeNil())
			Expect(roundTripped.ProjectCoverage.Phase).To(Equal("full"))
			Expect(roundTripped.ProjectSummary).NotTo(BeNil())
		})
	})
})
