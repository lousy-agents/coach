package render

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

const layerReachClause = "forbidden_imports was evaluated only between files inside declared layers"

func layerMembershipWith(outside int) *codesignal.LayerMembershipReport {
	return &codesignal.LayerMembershipReport{
		Head: codesignal.LayerMembershipRevisionReport{Revision: "head", AnalyzedFiles: 20, FilesOutsideLayers: outside},
	}
}

var _ = Describe("RenderText verdict when files sit outside every declared layer", func() {
	DescribeTable("carries the reach of forbidden_imports so the sentence a reader quotes cannot be taken as covering those files",
		func(report *codesignal.Report, want string) {
			Expect(ReportText(report)).To(ContainSubstring(want))
		},
		Entry("the unqualified all-clear names what forbidden_imports covered",
			&codesignal.Report{LayerMembership: layerMembershipWith(3)},
			"No active CodeSignal findings; "+layerReachClause+".\n"),
		Entry("the severity floor stays first and the reach second",
			&codesignal.Report{
				LayerMembership: layerMembershipWith(3),
				SignalsWithheld: &codesignal.SignalsWithheld{MinSeverity: "medium", BelowMinSeverity: 2},
			},
			"No active CodeSignal findings at or above --min-severity medium; "+layerReachClause+".\n"),
		Entry("a complete scan with no match names what forbidden_imports covered, like the all-clear",
			&codesignal.Report{
				LayerMembership:    layerMembershipWith(3),
				ProjectNextActions: []codesignal.ProjectNextAction{{Kind: "review_policy_coverage"}},
			},
			"Complete scan found no configured covered match; "+layerReachClause+".\n"),
		Entry("with an unanalyzed-paths tail the reach is a separate sentence, not part of the causes",
			&codesignal.Report{
				LayerMembership:    layerMembershipWith(3),
				ProjectNextActions: []codesignal.ProjectNextAction{{Kind: "review_policy_coverage"}},
				Summary:            codesignal.Summary{FilesUnanalyzed: 2},
			},
			"Complete scan found no configured covered match, but 2 paths were not analyzed. Separately, "+layerReachClause+".\n"),
		Entry("a no-match scan with unresolved imports keeps its causes whole and adds the reach separately",
			&codesignal.Report{
				LayerMembership:    layerMembershipWith(3),
				ProjectNextActions: []codesignal.ProjectNextAction{{Kind: "inspect_diagnostics"}},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "src/a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "m"},
				},
			},
			"Scan found no configured covered match, but the analysis is incomplete: 1 import did not resolve, so layer rules cannot tell whether it crosses a layer. Separately, "+layerReachClause+".\n"),
		Entry("an incomplete analysis keeps its causes whole and adds the reach separately",
			&codesignal.Report{
				LayerMembership: layerMembershipWith(3),
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: false},
			},
			"No active CodeSignal findings, but the analysis is incomplete: project analysis did not complete. Separately, "+layerReachClause+".\n"),
	)

	DescribeTable("leaves the verdict unchanged when nothing is outside every layer or the count was not computed",
		func(report *codesignal.Report, want string) {
			text := ReportText(report)

			Expect(text).To(ContainSubstring(want))
			Expect(text).NotTo(ContainSubstring(layerReachClause))
			Expect(text).NotTo(ContainSubstring("inside declared layers"))
			Expect(text).NotTo(ContainSubstring("Separately,"))
		},
		Entry("nothing outside every layer leaves the all-clear unchanged",
			&codesignal.Report{LayerMembership: layerMembershipWith(0)},
			"No active CodeSignal findings.\n"),
		Entry("a count that was not computed leaves the all-clear unchanged",
			&codesignal.Report{},
			"No active CodeSignal findings.\n"),
		Entry("nothing outside every layer leaves the complete-scan verdict unchanged",
			&codesignal.Report{
				LayerMembership:    layerMembershipWith(0),
				ProjectNextActions: []codesignal.ProjectNextAction{{Kind: "review_policy_coverage"}},
			},
			"Complete scan found no configured covered match.\n"),
	)
})
