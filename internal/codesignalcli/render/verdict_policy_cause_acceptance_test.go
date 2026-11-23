package render

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("ReportText verdict qualified by the project policy (#168 A2a)", func() {
	differs := func() *codesignal.PolicyScope {
		return scopePolicy(false, codesignal.PolicyOriginDiffersFromHead, "")
	}
	matchesHead := func() *codesignal.PolicyScope {
		return scopePolicy(true, codesignal.PolicyOriginMatchesHead, "")
	}
	coveredReport := func(policy *codesignal.PolicyScope) *codesignal.Report {
		report := policyReport(policyLayouts[0], policy)
		report.ProjectNextActions = []codesignal.ProjectNextAction{{Kind: "record_baseline"}}
		return report
	}
	noFindingsReport := func(policy *codesignal.PolicyScope, diagnostics ...codesignal.Diagnostic) *codesignal.Report {
		report := policyReport(policyLayouts[0], policy)
		report.Diagnostics = diagnostics
		return report
	}

	When("the scan is a covered TypeScript scan with no configured match", func() {
		It("keeps the unqualified verdict when the policy matches HEAD", func() {
			Expect(ReportText(coveredReport(matchesHead()))).To(ContainSubstring("\nComplete scan found no configured covered match.\n"))
		})

		It("qualifies the verdict when the policy differs from HEAD", func() {
			Expect(ReportText(coveredReport(differs()))).To(ContainSubstring(
				"\nComplete scan found no configured covered match, but the project policy on disk differs from HEAD.\n"))
		})

		It("adds the policy as a clause of its own after unanalyzed paths", func() {
			report := coveredReport(differs())
			report.Summary.FilesUnanalyzed = 2

			Expect(ReportText(report)).To(ContainSubstring(
				"\nComplete scan found no configured covered match, but 2 paths were not analyzed, and the project policy on disk differs from HEAD.\n"))
		})

		It("adds the policy as a clause of its own after an unresolved import", func() {
			report := coveredReport(differs())
			report.Diagnostics = []codesignal.Diagnostic{{Kind: codesignal.DiagKindProjectImportUnresolved, Path: "a.ts", Message: "./x"}}

			Expect(ReportText(report)).To(ContainSubstring(
				"\nScan found no configured covered match, but the analysis is incomplete: 1 import did not resolve, so layer rules cannot tell whether it crosses a layer, and the project policy on disk differs from HEAD.\n"))
		})
	})

	When("the scan has no findings and a diagnostic is recorded", func() {
		nonInformational := codesignal.Diagnostic{Kind: "project_config_deprecated", Message: "deprecated"}
		informational := codesignal.Diagnostic{Kind: codesignal.DiagKindWorktreeNotClean, Message: "dirty"}

		It("keeps the diagnostics cause and adds the policy as a clause of its own when the diagnostic is not informational", func() {
			Expect(ReportText(noFindingsReport(differs(), nonInformational))).To(ContainSubstring(
				"\nNo active CodeSignal findings, but the analysis is incomplete: additional diagnostics were recorded, and the project policy on disk differs from HEAD.\n"))
		})

		It("keeps the diagnostics-only wording when the diagnostic is not informational and the policy matches HEAD", func() {
			Expect(ReportText(noFindingsReport(matchesHead(), nonInformational))).To(ContainSubstring(
				"\nNo active CodeSignal findings, but the analysis is incomplete: additional diagnostics were recorded.\n"))
		})

		It("names only the policy when the diagnostic is informational", func() {
			Expect(ReportText(noFindingsReport(differs(), informational))).To(ContainSubstring("\n" + policyQualifiedAllClear + "\n"))
		})
	})

	When("the policy differs from HEAD and nothing else qualifies the result", func() {
		It("names only the policy", func() {
			Expect(ReportText(noFindingsReport(differs()))).To(ContainSubstring("\n" + policyQualifiedAllClear + "\n"))
		})
	})
})
