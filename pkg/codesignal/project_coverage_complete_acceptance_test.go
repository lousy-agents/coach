package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func provenanceWithCoverage(model, bypass string) *codesignal.ProjectProvenance {
	return &codesignal.ProjectProvenance{
		Head: codesignal.ProvenanceRevision{
			Coverage: codesignal.ProvenanceCoverage{Model: model, Bypass: bypass, Reachability: "complete"},
		},
	}
}

var _ = Describe("RequiredCoverageIncomplete for a report without provenance", func() {
	DescribeTable("reads the completeness a Go report does carry",
		func(report *codesignal.Report, want bool) {
			Expect(codesignal.RequiredCoverageIncomplete(report)).To(Equal(want))
		},
		Entry("a complete project_coverage is complete",
			&codesignal.Report{SchemaVersion: "2", ProjectCoverage: &domain.Coverage{Complete: true}}, false),
		Entry("an incomplete project_coverage, which folds the model and any bypass search, is incomplete",
			&codesignal.Report{SchemaVersion: "2", ProjectCoverage: &domain.Coverage{Complete: false}}, true),
		Entry("no recorded project_coverage is not treated as incomplete",
			&codesignal.Report{SchemaVersion: "2"}, false),
		Entry("an incomplete base side of a diff arrives as a coverage diagnostic",
			&codesignal.Report{
				SchemaVersion:   "2",
				ProjectCoverage: &domain.Coverage{Complete: true},
				Diagnostics:     []codesignal.Diagnostic{{Kind: codesignal.DiagKindProjectCoverageIncomplete}},
			}, true),
		Entry("an indeterminate lifecycle is incomplete",
			&codesignal.Report{
				SchemaVersion:   "2",
				ProjectCoverage: &domain.Coverage{Complete: true},
				Diagnostics:     []codesignal.Diagnostic{{Kind: codesignal.DiagKindProjectLifecycleIndeterminate}},
			}, true),
		Entry("an unrelated diagnostic does not make coverage incomplete",
			&codesignal.Report{
				SchemaVersion:   "2",
				ProjectCoverage: &domain.Coverage{Complete: true},
				Diagnostics:     []codesignal.Diagnostic{{Kind: codesignal.DiagKindWorktreeNotClean}},
			}, false),
		Entry("a schema-1 report with no project analysis is not incomplete",
			&codesignal.Report{SchemaVersion: "1"}, false),
		Entry("an unavailable backend is incomplete, as before",
			&codesignal.Report{Diagnostics: []codesignal.Diagnostic{{Kind: "project_backend_unavailable"}}}, true),
	)

	DescribeTable("leaves a report with provenance to its model and bypass phases, as before",
		func(report *codesignal.Report, want bool) {
			Expect(codesignal.RequiredCoverageIncomplete(report)).To(Equal(want))
		},
		Entry("complete provenance wins over an incomplete project_coverage",
			&codesignal.Report{
				ProjectProvenance: provenanceWithCoverage("complete", "not_requested"),
				ProjectCoverage:   &domain.Coverage{Complete: false},
			}, false),
		Entry("an incomplete model phase is incomplete",
			&codesignal.Report{ProjectProvenance: provenanceWithCoverage("incomplete", "not_requested")}, true),
		Entry("an incomplete bypass phase is incomplete",
			&codesignal.Report{ProjectProvenance: provenanceWithCoverage("complete", "incomplete")}, true),
	)
})
