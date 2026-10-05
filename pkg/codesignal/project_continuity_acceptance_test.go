package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func completeDiffInput(head, base []codesignal.ProjectChange, continuity ...codesignal.PathContinuity) codesignal.Input {
	return codesignal.Input{
		Scope:                  codesignal.Scope{Revision: "head-sha", Base: "base-sha"},
		ProjectChanges:         head,
		BaseProjectChanges:     base,
		ProjectBaseAnalyzed:    true,
		ProjectCoverage:        &domain.Coverage{Phase: "full", Complete: true},
		BaseProjectCoverage:    &domain.Coverage{Phase: "full", Complete: true},
		UndeterminedContinuity: continuity,
	}
}

func continuityDiagnostics(report *codesignal.Report) []codesignal.Diagnostic {
	var matches []codesignal.Diagnostic
	for _, d := range report.Diagnostics {
		if d.Kind == codesignal.DiagKindProjectChangeLifecycleIndeterminate {
			matches = append(matches, d)
		}
	}
	return matches
}

// AC-VER-3 (issue #334): a path-keyed project identity cannot tell a moved
// finding from a fixed one plus a new one, so a comparison across an
// undetermined rename/copy shall not claim either.
var _ = Describe("Project lifecycle across an undetermined rename/copy (AC-VER-3)", func() {
	moved := codesignal.PathContinuity{Path: "pkg/new/a.go", PreviousPath: "pkg/old/a.go"}

	When("the same finding is keyed by the old path at base and the new path at head", func() {
		It("classifies both sides unknown, counts neither, and names each path with its side and revision", func() {
			report := build(codesignal.Options{ProjectEnabled: true, IncludeResolved: true}, completeDiffInput(
				[]codesignal.ProjectChange{projectChangeAt("cycle:pkg/new<->pkg/b", "project.import_cycle", "pkg/new/a.go")},
				[]codesignal.ProjectChange{projectChangeAt("cycle:pkg/old<->pkg/b", "project.import_cycle", "pkg/old/a.go")},
				moved,
			))

			Expect(report.ProjectChanges).To(HaveLen(2))
			for _, change := range report.ProjectChanges {
				Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("unknown")), "%s", change.PrimaryAnchor.Path)
				Expect(change.Changed).To(BeFalse(), "%s", change.PrimaryAnchor.Path)
				Expect(change.Evidence).To(ContainSubstring(codesignal.DiagKindProjectChangeLifecycleIndeterminate))
			}
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0))
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(0))
			Expect(report.Summary.IntroducedSignals).To(Equal(0))
			Expect(report.Summary.ResolvedSignals).To(Equal(0))

			Expect(continuityDiagnostics(report)).To(ConsistOf(
				And(HaveField("Path", "pkg/new/a.go"), HaveField("Side", "head"), HaveField("Revision", "head-sha"),
					HaveField("Message", ContainSubstring("head revision head-sha"))),
				And(HaveField("Path", "pkg/old/a.go"), HaveField("Side", "base"), HaveField("Revision", "base-sha"),
					HaveField("Message", ContainSubstring("base revision base-sha"))),
			))
			Expect(report.Summary.FilesUnanalyzed).To(Equal(0))
		})
	})

	When("the moved path appears only in a related location", func() {
		It("still refuses the introduced claim and names the moved path", func() {
			change := projectChangeAt("cycle:pkg/a<->pkg/new", "project.import_cycle", "pkg/a/a.go")
			change.RelatedLocations = []codesignal.ProjectLocation{{Path: "pkg/new/a.go"}}
			report := build(codesignal.Options{ProjectEnabled: true}, completeDiffInput([]codesignal.ProjectChange{change}, nil, moved))

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(continuityDiagnostics(report)).To(ConsistOf(And(HaveField("Path", "pkg/new/a.go"), HaveField("Side", "head"))))
		})
	})

	When("only the importee moved, so no location points at the moved file", func() {
		DescribeTable("refuses both the resolved and the introduced claim and names each moved path",
			func(baseImportee, headImportee string, pair codesignal.PathContinuity) {
				layerChange := func(importee string) codesignal.ProjectChange {
					change := projectChangeAt("architecture.layer_violation:src/handlers->"+importee, "architecture.layer_violation", "src/handlers/h.ts")
					change.MachineEvidence = map[string]string{"importer": "src/handlers", "importee": importee}
					return change
				}
				report := build(codesignal.Options{ProjectEnabled: true, IncludeResolved: true}, completeDiffInput(
					[]codesignal.ProjectChange{layerChange(headImportee)},
					[]codesignal.ProjectChange{layerChange(baseImportee)},
					pair,
				))

				Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0))
				Expect(report.ProjectSummary.ResolvedChanges).To(Equal(0))
				Expect(continuityDiagnostics(report)).To(ConsistOf(
					And(HaveField("Path", pair.Path), HaveField("Side", "head")),
					And(HaveField("Path", pair.PreviousPath), HaveField("Side", "base")),
				))
			},
			Entry("a TypeScript importee file", "src/db/client.ts", "src/db/conn.ts",
				codesignal.PathContinuity{Path: "src/db/conn.ts", PreviousPath: "src/db/client.ts"}),
			Entry("a Go importee package directory", "pkg/db", "pkg/db/v2",
				codesignal.PathContinuity{Path: "pkg/db/v2/db.go", PreviousPath: "pkg/db/db.go"}),
		)
	})

	When("a file is copied, so the source still exists at head", func() {
		It("keeps the source finding existing and refuses only the destination's introduced claim", func() {
			copied := codesignal.PathContinuity{Path: "pkg/copy/a.go", PreviousPath: "pkg/src/a.go"}
			source := projectChangeAt("cycle:pkg/src<->pkg/b", "project.import_cycle", "pkg/src/a.go")
			report := build(codesignal.Options{ProjectEnabled: true}, completeDiffInput(
				[]codesignal.ProjectChange{source, projectChangeAt("cycle:pkg/copy<->pkg/b", "project.import_cycle", "pkg/copy/a.go")},
				[]codesignal.ProjectChange{source},
				copied,
			))

			Expect(report.ProjectChanges).To(ConsistOf(
				And(HaveField("PrimaryAnchor.Path", "pkg/src/a.go"), HaveField("Lifecycle", codesignal.Lifecycle("existing"))),
				And(HaveField("PrimaryAnchor.Path", "pkg/copy/a.go"), HaveField("Lifecycle", codesignal.Lifecycle("unknown"))),
			))
			Expect(continuityDiagnostics(report)).To(ConsistOf(And(HaveField("Path", "pkg/copy/a.go"), HaveField("Side", "head"))))
		})
	})

	When("the key survives the move on both sides", func() {
		It("keeps it existing, because continuity is not needed to match it", func() {
			key := "cycle:pkg/a<->pkg/b"
			report := build(codesignal.Options{ProjectEnabled: true}, completeDiffInput(
				[]codesignal.ProjectChange{projectChangeAt(key, "project.import_cycle", "pkg/new/a.go")},
				[]codesignal.ProjectChange{projectChangeAt(key, "project.import_cycle", "pkg/old/a.go")},
				moved,
			))

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(continuityDiagnostics(report)).To(BeEmpty())
		})
	})

	When("findings touch no renamed or copied path", func() {
		It("classifies them introduced and resolved as before", func() {
			report := build(codesignal.Options{ProjectEnabled: true, IncludeResolved: true}, completeDiffInput(
				[]codesignal.ProjectChange{projectChangeAt("cycle:pkg/c<->pkg/d", "project.import_cycle", "pkg/c/c.go")},
				[]codesignal.ProjectChange{projectChangeAt("cycle:pkg/e<->pkg/f", "project.import_cycle", "pkg/e/e.go")},
				moved,
			))

			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(1))
			Expect(continuityDiagnostics(report)).To(BeEmpty())
		})
	})

	When("several findings share one moved anchor", func() {
		It("names that path once per side", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, completeDiffInput(
				[]codesignal.ProjectChange{
					projectChangeAt("cycle:pkg/new<->pkg/b", "project.import_cycle", "pkg/new/a.go"),
					projectChangeAt("cycle:pkg/new<->pkg/c", "project.import_cycle", "pkg/new/a.go"),
				},
				nil,
				moved,
			))

			Expect(report.ProjectChanges).To(HaveLen(2))
			Expect(continuityDiagnostics(report)).To(HaveLen(1))
		})
	})

	When("a library caller leaves the head revision empty", func() {
		It("names the side without a dangling revision", func() {
			input := completeDiffInput([]codesignal.ProjectChange{projectChangeAt("cycle:pkg/new<->pkg/b", "project.import_cycle", "pkg/new/a.go")}, nil, moved)
			input.Scope.Revision = ""
			report := build(codesignal.Options{ProjectEnabled: true}, input)

			diagnostics := continuityDiagnostics(report)
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Message).To(HaveSuffix("at head revision"))
			Expect(diagnostics[0].Message).NotTo(ContainSubstring("  "))
		})
	})

	When("a library caller leaves the head revision empty and head coverage is incomplete", func() {
		It("names the side without a dangling revision", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
				ProjectCoverage: &domain.Coverage{Phase: "full", Complete: false},
			})

			diagnostics := continuityDiagnostics(report)
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Message).To(ContainSubstring("head revision project analysis coverage is incomplete"))
		})
	})

	When("TypeScript coverage is complete and the only project finding was moved", func() {
		It("does not report complete_no_match, because an unknown finding is not an absence of findings", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				Language:               "typescript",
				Scope:                  codesignal.Scope{Revision: "HEAD_SHA", Base: "BASE_SHA"},
				ProjectBaseAnalyzed:    true,
				HeadProjectScope:       headProjectScope(),
				BaseProjectScope:       headProjectScope(),
				HeadModelCoverage:      completeCoverage("model"),
				HeadBypassCoverage:     notRequestedCoverage(),
				BaseModelCoverage:      completeCoverage("model"),
				BaseBypassCoverage:     notRequestedCoverage(),
				ProjectCoverage:        &domain.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage:    &domain.Coverage{Phase: "full", Complete: true},
				ProjectChanges:         []codesignal.ProjectChange{projectChangeAt("cycle:pkg/new<->pkg/b", "project.import_cycle", "pkg/new/a.go")},
				BaseProjectChanges:     []codesignal.ProjectChange{projectChangeAt("cycle:pkg/old<->pkg/b", "project.import_cycle", "pkg/old/a.go")},
				UndeterminedContinuity: []codesignal.PathContinuity{moved},
			})

			Expect(report.ProjectChanges).NotTo(BeEmpty())
			Expect(rawReportFields(report)).NotTo(HaveKey("project_next_actions"))
		})
	})
})
