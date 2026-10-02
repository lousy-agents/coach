package codesignal_test

import (
	"encoding/json"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

type sideAttribution struct {
	side, revision string
}

func projectChangeAt(key, ruleID, path string) codesignal.ProjectChange {
	change := projectChange(key, ruleID)
	change.PrimaryAnchor.Path = path
	return change
}

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
		It("still reports schema 2 with an all-zero ProjectSummary and present-but-empty project_changes/project_coverage keys", func() {
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
			Expect(fields).To(HaveKey("project_changes"))
			Expect(string(fields["project_changes"])).To(Equal("[]"))
			Expect(fields).To(HaveKey("project_coverage"))
			Expect(string(fields["project_coverage"])).NotTo(Equal("null"))
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
			Expect(report.ProjectChanges[0].Evidence).To(Equal("pkg/a -> pkg/b -> pkg/a"), "a determinate resolved finding must not carry the indeterminate-lifecycle note")
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

		// A finding's own Evidence is the only place a reader looks while
		// scanning findings one at a time; the indeterminate diagnostic
		// explaining why lives in a separate top-level list. Without a
		// pointer on the finding itself, "lifecycle: unknown" reads as
		// unexplained noise.
		It("points from a degraded finding's own Evidence to the project_lifecycle_indeterminate diagnostic that explains why", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(report.ProjectChanges[0].Evidence).To(ContainSubstring("pkg/a -> pkg/b -> pkg/a"), "the producer's own evidence must survive, not be replaced")
			Expect(report.ProjectChanges[0].Evidence).To(ContainSubstring("project_lifecycle_indeterminate"), "a reader must be able to find why lifecycle degraded without separately cross-referencing the diagnostics list")
		})

		It("leaves Evidence exactly as the producer supplied it when lifecycle is a normal, determinate claim", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(report.ProjectChanges[0].Evidence).To(Equal("pkg/a -> pkg/b -> pkg/a"))
		})

		// The base-only branch (a semantic key present only in
		// BaseProjectChanges, a resolved-claim candidate) applies the same
		// Evidence pointer as the head-side branch above -- it must not be
		// left silently unexplained just because it's on the other side of
		// the comparison.
		It("points from a base-only finding's Evidence to the project_lifecycle_indeterminate diagnostic when base coverage is incomplete", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(report.ProjectChanges[0].Evidence).To(ContainSubstring("pkg/a -> pkg/b -> pkg/a"), "the producer's own evidence must survive, not be replaced")
			Expect(report.ProjectChanges[0].Evidence).To(ContainSubstring("project_lifecycle_indeterminate"), "a base-only finding needs the same pointer as a head-side one")
		})

		// A project change's own diagnostic is anchored at that change's
		// path so a reader can locate it, not because that path was itself
		// unanalyzed or skipped -- so it must not be counted as such.
		It("does not count a degraded project change's own diagnostic as an unanalyzed or skipped file", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
			})
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_change_lifecycle_indeterminate")))
			Expect(report.Summary.FilesWithDiagnostics).To(Equal(0), "pkg/a/a.go is a project change's anchor, not a file the diagnostics pipeline analyzed or skipped")
			Expect(report.Summary.FilesUnanalyzed).To(Equal(0), "pkg/a/a.go is a project change's anchor, not a file the diagnostics pipeline analyzed or skipped")
		})

		// A diagnostic naming this specific degraded change's own
		// repository-relative path must identify -- in machine-readable
		// Side/Revision fields, with text naming the same side and
		// revision -- which comparison side was incomplete, at both the
		// head-key classification site and the base-only (resolved
		// candidate) site. A mutation swapping "head"/"base" in either
		// branch must fail one of these rows.
		DescribeTable("attributes each degraded change's diagnostic to the comparison side(s) actually responsible",
			func(buildInput func() codesignal.Input, wantPath string, want []sideAttribution) {
				report := build(codesignal.Options{ProjectEnabled: true}, buildInput())

				var matches []codesignal.Diagnostic
				for _, d := range report.Diagnostics {
					if d.Kind == "project_change_lifecycle_indeterminate" && d.Path == wantPath {
						matches = append(matches, d)
					}
				}
				Expect(matches).To(HaveLen(len(want)), "expected exactly one diagnostic per implicated side naming %s", wantPath)

				for _, exp := range want {
					found := false
					for i, m := range matches {
						if m.Side == exp.side && m.Revision == exp.revision {
							Expect(m.Message).To(ContainSubstring(exp.side))
							Expect(m.Message).To(ContainSubstring(exp.revision))
							matches = append(matches[:i], matches[i+1:]...)
							found = true
							break
						}
					}
					Expect(found).To(BeTrue(), "expected a diagnostic with side %q revision %q naming %s", exp.side, exp.revision, wantPath)
				}
			},
			Entry("head coverage incomplete, at the head-key site", func() codesignal.Input {
				return codesignal.Input{
					Scope:           codesignal.Scope{Revision: "head-sha"},
					ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
				}
			}, "pkg/a/a.go", []sideAttribution{{"head", "head-sha"}}),
			Entry("base coverage incomplete, at the head-key site", func() codesignal.Input {
				return codesignal.Input{
					Scope:               codesignal.Scope{Revision: "head-sha", Base: "base-sha"},
					ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
					BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
				}
			}, "pkg/a/a.go", []sideAttribution{{"base", "base-sha"}}),
			Entry("base observations supplied without a completed base analysis, at the head-key site", func() codesignal.Input {
				return codesignal.Input{
					Scope:               codesignal.Scope{Revision: "head-sha", Base: "base-sha"},
					ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					BaseProjectChanges:  []codesignal.ProjectChange{projectChangeAt("cycle:pkg/c<->pkg/d", "project.import_cycle", "pkg/c/c.go")},
					ProjectBaseAnalyzed: false,
					ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				}
			}, "pkg/a/a.go", []sideAttribution{{"base", "base-sha"}}),
			Entry("head and base both incomplete emits one diagnostic per side for the same path", func() codesignal.Input {
				return codesignal.Input{
					Scope:               codesignal.Scope{Revision: "head-sha", Base: "base-sha"},
					ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: false},
					BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: false},
				}
			}, "pkg/a/a.go", []sideAttribution{{"head", "head-sha"}, {"base", "base-sha"}}),
			Entry("a base-only key classified at the resolved-candidate site is attributed the same as a head-key one", func() codesignal.Input {
				return codesignal.Input{
					Scope:               codesignal.Scope{Revision: "head-sha"},
					BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: false},
					BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
				}
			}, "pkg/a/a.go", []sideAttribution{{"head", "head-sha"}}),
		)
	})

	// The control run below (base coverage complete) proves the
	// incompleteness in the second run is what produces "unknown", rather
	// than the fixture happening to produce it for an unrelated reason.
	Describe("AC-VER-3: indeterminate lifecycle and counters instead of a false improvement claim", func() {
		It("keeps a change unknown and uncounted, instead of existing, when base coverage is genuinely incomplete", func() {
			fixture := func(baseCoverageComplete bool) codesignal.Input {
				return codesignal.Input{
					ProjectChanges:      []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
					ProjectBaseAnalyzed: true,
					ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
					BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: baseCoverageComplete},
				}
			}

			control := build(codesignal.Options{ProjectEnabled: true}, fixture(true))
			Expect(control.ProjectChanges).To(HaveLen(1))
			Expect(control.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")), "control: a complete comparison must classify this key existing")
			Expect(control.ProjectSummary.ExistingChanges).To(Equal(1))
			Expect(control.ProjectSummary.IntroducedChanges).To(Equal(0))
			Expect(control.ProjectSummary.ResolvedChanges).To(Equal(0))

			report := build(codesignal.Options{ProjectEnabled: true}, fixture(false))
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")), "an incomplete base comparison must never claim a determinate lifecycle")
			Expect(report.ProjectSummary.ExistingChanges).To(Equal(0), "an incomplete comparison must not claim the change is still existing")
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0), "an incomplete comparison must not claim the change was introduced")
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(0), "an incomplete comparison must not claim the change was resolved")
		})
	})

	// F-003: active project observations must appear on the shared signals
	// surface and normal summary counters; facts-only stay outside both.
	Describe("project observations on the shared signals surface", func() {
		It("maps an active project observation into signals and summary counters while facts stay facts-only", func() {
			active := projectChange("cycle:pkg/a<->pkg/b", "architecture.layer_violation")
			// Deliberately unsorted producer order so the assertions below can
			// distinguish "mirrored verbatim" from "mirrored canonicalized"
			// (RelatedLocations sorts by path, so "pkg/z/z.go" before
			// "pkg/b/b.go" would pass a same-order-as-input assertion but fail
			// a same-order-as-canonicalized one).
			active.RelatedLocations = []codesignal.ProjectLocation{
				{Path: "pkg/z/z.go", Location: semantics.Location{StartRow: 9}},
				{Path: "pkg/b/b.go", Location: semantics.Location{StartRow: 4}},
			}
			active.PathSteps = []codesignal.ProjectPathStep{
				{NodeID: "func:Z", DisplayName: "Z", Resolution: "static", Confidence: codesignal.Confidence("medium")},
			}
			active.CoverageRefs = []string{"z_ref", "a_ref"}
			active.MachineEvidence = map[string]string{
				"importer": "pkg/a",
				"importee": "pkg/b",
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

			// signals[] must carry the same structured evidence as
			// project_changes[] -- consumers reading only signals get full parity.
			// The mirrored arrays are asserted against the canonicalized
			// project_changes[0] values (not the raw producer-order active.*
			// fixture) so this also locks that signalFromProjectChange mirrors
			// post-canonicalization output, not pre-canonicalization input.
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(sig.MachineEvidence).To(Equal(active.MachineEvidence))
			Expect(sig.RelatedLocations).To(Equal(report.ProjectChanges[0].RelatedLocations))
			Expect(sig.PathSteps).To(Equal(report.ProjectChanges[0].PathSteps))
			Expect(sig.CoverageRefs).To(Equal(report.ProjectChanges[0].CoverageRefs))
			Expect(sig.RelatedLocations).NotTo(Equal(active.RelatedLocations), "canonicalization must sort RelatedLocations by path, not preserve producer order")
			Expect(sig.CoverageRefs).To(Equal([]string{"a_ref", "z_ref"}), "canonicalization must sort CoverageRefs")

			// Struct-field equality above can't catch a JSON tag rename; lock the
			// wire keys and canonicalized (sorted) values at the raw-bytes level.
			sigRaw, err := json.Marshal(sig)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(sigRaw)).To(ContainSubstring(`"related_locations":[{"path":"pkg/b/b.go","location":{"start_byte":0,"end_byte":0,"start_row":4,"start_col":0,"end_row":0,"end_col":0}},{"path":"pkg/z/z.go","location":{"start_byte":0,"end_byte":0,"start_row":9,"start_col":0,"end_row":0,"end_col":0}}]`))
			Expect(string(sigRaw)).To(ContainSubstring(`"path_steps":[{"node_id":"func:Z","display_name":"Z","resolution":"static","confidence":"medium"}]`))
			Expect(string(sigRaw)).To(ContainSubstring(`"coverage_refs":["a_ref","z_ref"]`))

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

		// F-2: every active project Signal requires a repository-relative primary
		// path; anchorless observations stay facts/coverage only — not Signals,
		// not project_changes, and not ProjectSummary active counters.
		It("does not promote a ProjectChange with an empty primary anchor path to a Signal", func() {
			anchorless := projectChange("cycle:pkg/a<->pkg/b", "architecture.layer_violation")
			anchorless.PrimaryAnchor = codesignal.ProjectLocation{}

			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{anchorless},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			Expect(report.Signals).To(BeEmpty())
			Expect(report.Summary.ActiveSignals).To(Equal(0))
			Expect(report.Summary.BaselineSignals).To(Equal(0))
			Expect(report.ProjectChanges).To(BeEmpty(), "anchorless observations must not appear as project findings")
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(0))
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_observation_missing_primary_path")))
			for _, sig := range report.Signals {
				Expect(sig.Path).NotTo(BeEmpty())
			}

			// Control: the same observation with a real anchor still maps once.
			anchored := projectChange("cycle:pkg/a<->pkg/b", "architecture.layer_violation")
			control := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{anchored},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(control.Signals).To(HaveLen(1))
			Expect(control.Signals[0].Path).To(Equal("pkg/a/a.go"))
			Expect(control.Summary.ActiveSignals).To(Equal(1))
		})
	})

	Describe("project observation input hardening", func() {
		It("keeps the first ProjectChange per SemanticKey and emits project_duplicate_semantic_key", func() {
			first := projectChange("cycle:pkg/a<->pkg/b", "architecture.layer_violation")
			first.Evidence = "first"
			second := projectChange("cycle:pkg/a<->pkg/b", "architecture.layer_violation")
			second.Evidence = "second"
			second.PrimaryAnchor.Path = "pkg/other/o.go"

			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{first, second},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Evidence).To(Equal("first"))
			Expect(report.ProjectChanges[0].PrimaryAnchor.Path).To(Equal("pkg/a/a.go"))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_duplicate_semantic_key")))
		})

		It("serializes structured machine_evidence with stable key order for layer-violation consumers", func() {
			change := projectChange("layer:domain->infra", "architecture.layer_violation")
			change.Kind = "project_layer_violation"
			change.MachineEvidence = map[string]string{
				"policy_rule":    "domain_must_not_import_infrastructure",
				"importer":       "pkg/domain",
				"importee":       "pkg/infra",
				"importer_layer": "domain",
				"importee_layer": "infrastructure",
				"edge_kind":      "internal",
			}

			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{change},
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("importer", "pkg/domain"))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("policy_rule", "domain_must_not_import_infrastructure"))

			raw, err := json.Marshal(report.ProjectChanges[0])
			Expect(err).NotTo(HaveOccurred())
			// encoding/json sorts map keys; freeze the epic #208 field set order in the wire bytes.
			Expect(string(raw)).To(ContainSubstring(`"machine_evidence":{"edge_kind":"internal","importee":"pkg/infra","importee_layer":"infrastructure","importer":"pkg/domain","importer_layer":"domain","policy_rule":"domain_must_not_import_infrastructure"}`))

			Expect(report.Signals).To(HaveLen(1))
			sigRaw, err := json.Marshal(report.Signals[0])
			Expect(err).NotTo(HaveOccurred())
			// signals[] must carry the same stable-ordered machine_evidence bytes.
			Expect(string(sigRaw)).To(ContainSubstring(`"machine_evidence":{"edge_kind":"internal","importee":"pkg/infra","importee_layer":"infrastructure","importer":"pkg/domain","importer_layer":"domain","policy_rule":"domain_must_not_import_infrastructure"}`))
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
			Expect(fields).To(HaveKey("project_changes"))
			Expect(string(fields["project_changes"])).To(Equal("[]"))

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

	// AC-VER-5/AC-VER-6 (#334, epic #280): reachability ProjectFacts describe
	// current HEAD only and must never receive ProjectChange's lifecycle
	// vocabulary (introduced/existing/resolved/baseline) or improvement
	// framing when a fact's presence differs across a --base comparison.
	Describe("reachability facts carry no lifecycle or improvement semantics (AC-VER-5, AC-VER-6)", func() {
		It("has no Lifecycle-style field on ProjectFact", func() {
			factType := reflect.TypeOf(codesignal.ProjectFact{})
			forbidden := []string{"lifecycle", "introduced", "resolved", "improv", "baseline", "existing"}
			for i := 0; i < factType.NumField(); i++ {
				field := factType.Field(i)
				lowerName := strings.ToLower(field.Name)
				lowerTag := strings.ToLower(field.Tag.Get("json"))
				for _, word := range forbidden {
					Expect(lowerName).NotTo(ContainSubstring(word), "ProjectFact field %q must not carry lifecycle-style naming", field.Name)
					Expect(lowerTag).NotTo(ContainSubstring(word), "ProjectFact JSON tag %q must not carry lifecycle-style naming", field.Tag.Get("json"))
				}
			}
		})

		It("carries facts through the pipeline on exactly one head-only field, with no base-side, resolved, removed, or previous fact field on Input, Report, or the project backend result", func() {
			// A regression that added e.g. Input.BaseProjectFacts, or a
			// Report.ResolvedProjectFacts/RemovedProjectFacts section, or
			// ProjectBackendResult.BaseFacts would let base-side or
			// lifecycle-classified facts flow into the pipeline without ever
			// touching ProjectFact's own fields (checked above) or the
			// with/without-fact rendering fixture below (which only ever
			// builds head-only reports). Requiring exactly one
			// "Fact"-named field per type, with the fixed name each type
			// already uses, closes that gap regardless of what name or
			// prefix a regression would pick.
			assertOnlyFactField := func(t reflect.Type, allowedFieldName string) {
				for i := 0; i < t.NumField(); i++ {
					field := t.Field(i)
					if !strings.Contains(field.Name, "Fact") {
						continue
					}
					Expect(field.Name).To(Equal(allowedFieldName), "%s must carry facts on exactly one field named %q; found an additional fact-bearing field %q", t.Name(), allowedFieldName, field.Name)
					for _, forbiddenPrefix := range []string{"Base", "Resolved", "Removed", "Previous"} {
						Expect(field.Name).NotTo(HavePrefix(forbiddenPrefix), "%s.%s must not carry a %s-prefixed fact field", t.Name(), field.Name, forbiddenPrefix)
					}
				}
			}

			assertOnlyFactField(reflect.TypeOf(codesignal.Input{}), "ProjectFacts")
			assertOnlyFactField(reflect.TypeOf(codesignal.Report{}), "ProjectFacts")
			assertOnlyFactField(reflect.TypeOf(codesignalcli.ProjectBackendResult{}), "Facts")
		})

		It("renders a reachability fact's appearance/disappearance across two otherwise-identical head reports with no lifecycle or improvement wording, even alongside a genuinely resolved ProjectChange", func() {
			fact := codesignal.ProjectFact{
				Kind:        "possible_call_reachability",
				SemanticKey: "reach:handler->query",
				Evidence:    "Handler may reach Query",
				Provenance:  codesignal.Provenance{Producer: "projectmodel"},
			}
			// A base-only ProjectChange that legitimately classifies "resolved"
			// (see "surfaces a base-only key resolved when IncludeResolved is
			// set" above) -- present in both reports below so the assertions
			// can distinguish real lifecycle wording (which must appear, for
			// the change) from wording that leaks onto the fact's own
			// presence/absence (which must not).
			resolvedInput := codesignal.Input{
				BaseProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			}
			options := codesignal.Options{ProjectEnabled: true, IncludeResolved: true}

			withFactInput := resolvedInput
			withFactInput.ProjectFacts = []codesignal.ProjectFact{fact}
			reportWithFact := build(options, withFactInput)
			Expect(reportWithFact.ProjectChanges).To(HaveLen(1))
			Expect(reportWithFact.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")), "fixture must exercise a real resolved ProjectChange alongside the fact")
			Expect(reportWithFact.ProjectFacts).To(HaveLen(1))

			withoutFactInput := resolvedInput // identical head-only input, but with the fact absent
			reportWithoutFact := build(options, withoutFactInput)
			Expect(reportWithoutFact.ProjectFacts).To(BeEmpty())
			Expect(reportWithoutFact.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))

			// JSON: the fact's own serialized bytes must never carry
			// ProjectChange's lifecycle vocabulary or improvement wording.
			factRaw, err := json.Marshal(reportWithFact.ProjectFacts[0])
			Expect(err).NotTo(HaveOccurred())
			for _, word := range []string{"lifecycle", "introduced", "resolved", "improv", "baseline", "existing", "no_longer"} {
				Expect(strings.ToLower(string(factRaw))).NotTo(ContainSubstring(word))
			}

			// TEXT: removing exactly the fact's own rendered block from the
			// with-fact report's text must reproduce the without-fact report's
			// text byte-for-byte. An implementation that reacted to the fact's
			// disappearance by adding compensating prose (e.g. "no longer
			// reachable, resolved") anywhere else in the report -- verdict,
			// summary, or elsewhere -- would break this equality even though
			// it never touches ProjectFact's own fields.
			textWithFact := codesignalcli.RenderText(reportWithFact)
			textWithoutFact := codesignalcli.RenderText(reportWithoutFact)
			expectedFactBlock := "\nFacts:\nkind: possible_call_reachability\nsemantic_key: reach:handler->query\nevidence: Handler may reach Query\n"
			Expect(textWithFact).To(ContainSubstring(expectedFactBlock))
			Expect(textWithoutFact).NotTo(ContainSubstring("Facts:"))
			Expect(strings.Replace(textWithFact, expectedFactBlock, "", 1)).To(Equal(textWithoutFact), "a fact's presence/absence must change only its own rendered block, never any other wording in the report")

			for _, word := range []string{"no longer reachable", "improved", "fixed"} {
				Expect(strings.ToLower(textWithFact)).NotTo(ContainSubstring(word))
				Expect(strings.ToLower(textWithoutFact)).NotTo(ContainSubstring(word))
			}
		})
	})

	Describe("TypeScript reachability facts mapped into ProjectFact", func() {
		It("maps a resolved TS possible-call-reachability fact with projectmodel/kind provenance and produces no active Signal/Change through the shared pipeline", func() {
			result := projectmodel.ReachabilityResult{
				Facts: []projectmodel.ReachabilityFact{{
					ID:         "reach:file:src/app.ts#getUsers->(PrismaClient).findMany@ts-source-sink-registry@1",
					Kind:       projectmodel.KindPossibleCallReachability,
					Confidence: projectmodel.ReachabilityConfidenceResolvedDirect,
					Source:     "file:src/app.ts#getUsers",
					Sink:       "(PrismaClient).findMany",
					Path: []projectmodel.ReachabilityStep{
						{NodeID: "file:src/app.ts#getUsers"},
						{NodeID: "(PrismaClient).findMany"},
					},
					AlgorithmVersion: "ts-source-sink-registry@1",
				}},
				Sources:   []string{"file:src/app.ts#getUsers"},
				Algorithm: "ts-source-sink-registry@1",
				Coverage:  projectmodel.Coverage{Phase: "ts_sidecar_build", Complete: true},
			}

			facts := codesignal.ReachabilityProjectFacts(result, "typescript")
			Expect(facts).To(HaveLen(1))
			fact := facts[0]
			Expect(fact.Kind).To(Equal(projectmodel.KindPossibleCallReachability))
			Expect(fact.Provenance.Producer).To(Equal("projectmodel"))
			Expect(fact.Provenance.FindingKind).To(Equal(projectmodel.KindPossibleCallReachability))
			Expect(fact.Provenance.Language).To(Equal("typescript"))
			Expect(fact.SemanticKey).To(Equal("possible_call_reachability:file:src/app.ts#getUsers->(PrismaClient).findMany"))
			Expect(fact.PathSteps).To(HaveLen(2))
			Expect(fact.PathSteps[0].NodeID).To(Equal("file:src/app.ts#getUsers"))
			Expect(fact.PathSteps[0].Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(fact.PathSteps[1].NodeID).To(Equal("(PrismaClient).findMany"))
			Expect(fact.PathSteps[1].Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(fact.Evidence).To(ContainSubstring("possible"), "expected shared possible-call wording per AC-1")

			// AC-4/AC-11 regression guard: routing this ProjectFact through the
			// existing generic Input.ProjectFacts -> Report.ProjectFacts pipeline
			// must never promote it to a ProjectChange, a Signal, or an active
			// ProjectSummary counter.
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectFacts:    facts,
				ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			Expect(report.ProjectFacts).To(HaveLen(1))
			Expect(report.ProjectFacts[0].Provenance.Producer).To(Equal("projectmodel"))
			Expect(report.ProjectChanges).To(BeEmpty())
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(*report.ProjectSummary).To(Equal(codesignal.ProjectSummary{}), "expected an all-zero ProjectSummary: a fact must never inflate active/introduced/existing/resolved/baseline counters")
			Expect(report.Signals).To(BeEmpty())
			Expect(report.Summary.ActiveSignals).To(Equal(0))
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
