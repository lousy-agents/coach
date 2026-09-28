package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func body_projectAcceptanceTest_namesOnlyHeadCoverageInTheIndeterminateDiagnosti_229() {
	report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
		ProjectChanges:  []codesignal.ProjectChange{projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")},
		ProjectCoverage: &domain.Coverage{Phase: "full", Complete: false},
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
}

func body_projectAcceptanceTest_doesNotPromoteAProjectChangeWithAnEmptyPrimaryAn_388() {
	anchorless := projectChange("cycle:pkg/a<->pkg/b", "architecture.layer_violation")
	anchorless.PrimaryAnchor = codesignal.ProjectLocation{}

	report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
		ProjectChanges:  []codesignal.ProjectChange{anchorless},
		ProjectCoverage: &domain.Coverage{Phase: "full", Complete: true},
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
		ProjectCoverage: &domain.Coverage{Phase: "full", Complete: true},
	})
	Expect(control.Signals).To(HaveLen(1))
	Expect(control.Signals[0].Path).To(Equal("pkg/a/a.go"))
	Expect(control.Summary.ActiveSignals).To(Equal(1))
}
