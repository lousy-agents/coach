package render

import (
	"regexp"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const (
	policyTestPath   = ".lousy-agents/coach.json"
	policyTestDigest = "pcfg_24ee6328b822aabbccddeeff00112233445566778899aabbccddeeff00112233"
	policyTestPrefix = "policy: .lousy-agents/coach.json (pcfg_24ee6328b822), "

	policyDiffersWarning    = "WARNING: the project policy on disk differs from HEAD's; this scan used the policy on disk, so its result can differ from CI, which reads the committed policy."
	policyNotCommittedWarn  = "WARNING: the project policy .lousy-agents/coach.json is on disk and not committed at HEAD; this scan used it."
	policyMissingOnDiskWarn = "WARNING: .lousy-agents/coach.json is committed at HEAD and missing on disk; this scan used the committed copy."
	policyQualifiedAllClear = "No active CodeSignal findings, but the project policy on disk differs from HEAD."
	policyUnqualifiedClear  = "No active CodeSignal findings."
	policyTestWorktreeRev   = "0123456789abcdef0123"
	policyTestWorktreeBase  = "fedcba9876543210fedc"
	policyTestBaselineRev   = "0123456789abcdef0123"
	policyTestFindingRuleID = "test.rule"
)

type policyLayout struct {
	name    string
	summary *regexp.Regexp
	scope   codesignal.Scope
	// worktreeAfterSummary is true when the layout prints its worktree line after the summary line.
	worktreeAfterSummary bool
}

var policyLayouts = []policyLayout{
	{
		name:    "base",
		summary: regexp.MustCompile(`^scope: production, filtered: 0, files analyzed: `),
		scope:   codesignal.Scope{AppliedScope: "production", Base: policyTestWorktreeBase},
	},
	{
		name:                 "base worktree",
		summary:              regexp.MustCompile(`^scope: production, filtered: 0, files analyzed: `),
		scope:                codesignal.Scope{AppliedScope: "production", Base: policyTestWorktreeBase, Worktree: &codesignal.WorktreeScope{Revision: policyTestWorktreeRev}},
		worktreeAfterSummary: true,
	},
	{
		name:    "baseline",
		summary: regexp.MustCompile(`^tracked files discovered: `),
		scope:   codesignal.Scope{Baseline: true, Revision: policyTestBaselineRev},
	},
	{
		name:    "baseline worktree",
		summary: regexp.MustCompile(`^files discovered \(tracked and new\): `),
		scope:   codesignal.Scope{Baseline: true, Revision: policyTestBaselineRev, Worktree: &codesignal.WorktreeScope{Revision: policyTestWorktreeRev}},
	},
}

func policyReport(layout policyLayout, policy *codesignal.PolicyScope) *codesignal.Report {
	scope := layout.scope
	scope.Policy = policy
	return &codesignal.Report{Scope: scope}
}

func scopePolicy(matchesHead bool, cause codesignal.PolicyCause) *codesignal.PolicyScope {
	return &codesignal.PolicyScope{Path: policyTestPath, Digest: policyTestDigest, MatchesHead: matchesHead, Cause: cause}
}

func linesAfterSummary(layout policyLayout, text string, count int) []string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if layout.summary.MatchString(line) {
			return lines[i+1 : min(i+1+count, len(lines))]
		}
	}
	Fail("no summary line in " + layout.name + " text:\n" + text)
	return nil
}

var _ = Describe("ReportText project policy disclosure (#168 A2a)", func() {
	When("the scan used no project policy", func() {
		for _, layout := range policyLayouts {
			It("renders the "+layout.name+" text without a policy line or WARNING", func() {
				text := ReportText(policyReport(layout, nil))

				Expect(text).NotTo(ContainSubstring("policy:"))
				Expect(text).NotTo(ContainSubstring("WARNING"))
				Expect(text).To(ContainSubstring(policyUnqualifiedClear + "\n"))
			})
		}
	})

	When("the scan used a project policy", func() {
		for _, layout := range policyLayouts {
			It("puts the policy line directly after the "+layout.name+" summary line", func() {
				text := ReportText(policyReport(layout, scopePolicy(true, codesignal.PolicyCauseSameAsHead)))

				Expect(linesAfterSummary(layout, text, 1)).To(Equal([]string{policyTestPrefix + "same as HEAD."}), "text:\n%s", text)
				Expect(text).NotTo(ContainSubstring("WARNING"))
			})
		}

		It("puts the policy line before the worktree line a --base --worktree scan prints after its summary", func() {
			layout := policyLayouts[1]

			text := ReportText(policyReport(layout, scopePolicy(false, codesignal.PolicyCauseDiffersFromHead)))

			after := linesAfterSummary(layout, text, 3)
			Expect(after[0]).To(Equal(policyTestPrefix + "differs from HEAD."))
			Expect(after[1]).To(Equal(policyDiffersWarning))
			Expect(after[2]).To(HavePrefix("worktree: "))
		})

		It("keeps the worktree line before the summary line in a --baseline --worktree scan", func() {
			layout := policyLayouts[3]

			text := ReportText(policyReport(layout, scopePolicy(true, codesignal.PolicyCauseSameAsHead)))

			Expect(strings.Index(text, "worktree: ")).To(BeNumerically("<", strings.Index(text, "policy: ")))
		})

		for _, layout := range policyLayouts {
			It("adds the differs WARNING right after the policy line in the "+layout.name+" layout", func() {
				text := ReportText(policyReport(layout, scopePolicy(false, codesignal.PolicyCauseDiffersFromHead)))

				Expect(linesAfterSummary(layout, text, 2)).To(Equal([]string{policyTestPrefix + "differs from HEAD.", policyDiffersWarning}), "text:\n%s", text)
			})
		}
	})

	DescribeTable("the policy line and WARNING chosen by where the policy came from",
		func(policy *codesignal.PolicyScope, wantLine, wantWarning string) {
			layout := policyLayouts[0]

			text := ReportText(policyReport(layout, policy))

			if wantWarning == "" {
				Expect(linesAfterSummary(layout, text, 1)).To(Equal([]string{wantLine}), "text:\n%s", text)
				Expect(text).NotTo(ContainSubstring("WARNING"))
				return
			}
			Expect(linesAfterSummary(layout, text, 2)).To(Equal([]string{wantLine, wantWarning}), "text:\n%s", text)
			Expect(strings.Count(text, "WARNING")).To(Equal(1))
		},
		Entry("matches HEAD", scopePolicy(true, codesignal.PolicyCauseSameAsHead), policyTestPrefix+"same as HEAD.", ""),
		Entry("zero cause and matches_head true reads as same as HEAD", scopePolicy(true, codesignal.PolicyCauseUnset), policyTestPrefix+"same as HEAD.", ""),
		Entry("zero cause and matches_head false reads as differs from HEAD", scopePolicy(false, codesignal.PolicyCauseUnset), policyTestPrefix+"differs from HEAD.", policyDiffersWarning),
		Entry("differs from HEAD", scopePolicy(false, codesignal.PolicyCauseDiffersFromHead), policyTestPrefix+"differs from HEAD.", policyDiffersWarning),
		Entry("not committed at HEAD", scopePolicy(false, codesignal.PolicyCauseNotCommitted), policyTestPrefix+"not committed at HEAD.", policyNotCommittedWarn),
		Entry("committed at HEAD and missing on disk", scopePolicy(false, codesignal.PolicyCauseMissingOnDisk), policyTestPrefix+"differs from HEAD.", policyMissingOnDiskWarn),
	)

	DescribeTable("a policy path that holds a control character is shown quoted and escaped, so the text carries no line break or escape",
		func(path string, cause codesignal.PolicyCause, quotedPath string, wantLineRelation string, wantWarning string) {
			layout := policyLayouts[0]
			policy := scopePolicy(cause == codesignal.PolicyCauseSameAsHead, cause)
			policy.Path = path

			text := ReportText(policyReport(layout, policy))

			Expect(text).NotTo(ContainSubstring(path), "text:\n%s", text)
			Expect(text).NotTo(ContainSubstring("\x1b"))
			want := []string{"policy: " + quotedPath + " (pcfg_24ee6328b822), " + wantLineRelation}
			if wantWarning != "" {
				want = append(want, wantWarning)
			}
			Expect(linesAfterSummary(layout, text, len(want))).To(Equal(want), "text:\n%s", text)
		},
		Entry("a line break, on the policy line", "my\nconfig.json", codesignal.PolicyCauseSameAsHead, `"my\nconfig.json"`, "same as HEAD.", ""),
		Entry("an escape byte, on the policy line", "my\x1bconfig.json", codesignal.PolicyCauseSameAsHead, `"my\x1bconfig.json"`, "same as HEAD.", ""),
		Entry("a line break, in the not-committed WARNING", "my\nconfig.json", codesignal.PolicyCauseNotCommitted, `"my\nconfig.json"`, "not committed at HEAD.",
			`WARNING: the project policy "my\nconfig.json" is on disk and not committed at HEAD; this scan used it.`),
		Entry("an escape byte, in the not-committed WARNING", "my\x1bconfig.json", codesignal.PolicyCauseNotCommitted, `"my\x1bconfig.json"`, "not committed at HEAD.",
			`WARNING: the project policy "my\x1bconfig.json" is on disk and not committed at HEAD; this scan used it.`),
		Entry("a line break, in the missing-on-disk WARNING", "my\nconfig.json", codesignal.PolicyCauseMissingOnDisk, `"my\nconfig.json"`, "differs from HEAD.",
			`WARNING: "my\nconfig.json" is committed at HEAD and missing on disk; this scan used the committed copy.`),
		Entry("an escape byte, in the missing-on-disk WARNING", "my\x1bconfig.json", codesignal.PolicyCauseMissingOnDisk, `"my\x1bconfig.json"`, "differs from HEAD.",
			`WARNING: "my\x1bconfig.json" is committed at HEAD and missing on disk; this scan used the committed copy.`),
	)

	DescribeTable("the verdict of a result with no findings",
		func(policy *codesignal.PolicyScope, want string) {
			text := ReportText(policyReport(policyLayouts[0], policy))

			Expect(text).To(ContainSubstring("\n" + want + "\n"))
			if want != policyUnqualifiedClear {
				Expect(text).NotTo(ContainSubstring("\n" + policyUnqualifiedClear + "\n"))
			}
		},
		Entry("no policy stays an unqualified all-clear", nil, policyUnqualifiedClear),
		Entry("a policy matching HEAD stays an unqualified all-clear", scopePolicy(true, codesignal.PolicyCauseSameAsHead), policyUnqualifiedClear),
		Entry("a policy differing from HEAD qualifies the all-clear", scopePolicy(false, codesignal.PolicyCauseDiffersFromHead), policyQualifiedAllClear),
		Entry("a zero-cause policy with matches_head false qualifies the all-clear", scopePolicy(false, codesignal.PolicyCauseUnset), policyQualifiedAllClear),
		Entry("a policy not committed at HEAD qualifies the all-clear", scopePolicy(false, codesignal.PolicyCauseNotCommitted),
			"No active CodeSignal findings, but the project policy on disk is not committed at HEAD."),
		Entry("a policy missing on disk qualifies the all-clear", scopePolicy(false, codesignal.PolicyCauseMissingOnDisk),
			"No active CodeSignal findings, but the project policy is missing on disk and the committed copy was used."),
	)

	When("the policy differs from HEAD and another cause already qualifies the all-clear", func() {
		It("keeps that cause and adds the policy cause", func() {
			report := policyReport(policyLayouts[0], scopePolicy(false, codesignal.PolicyCauseDiffersFromHead))
			report.Summary.FilesUnanalyzed = 2
			report.Diagnostics = []codesignal.Diagnostic{{Kind: "project_config_deprecated", Message: "deprecated"}}

			text := ReportText(report)

			Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete: 2 paths were not analyzed, and the project policy on disk differs from HEAD.\n"))
		})
	})

	When("the scan has findings", func() {
		findingReport := func(policy *codesignal.PolicyScope) *codesignal.Report {
			report := policyReport(policyLayouts[0], policy)
			report.Signals = []codesignal.Signal{{ID: "s1", RuleID: policyTestFindingRuleID, Evidence: "a finding", Path: "a.go"}}
			return report
		}

		It("still shows the policy line and WARNING, and renders no all-clear verdict", func() {
			text := ReportText(findingReport(scopePolicy(false, codesignal.PolicyCauseDiffersFromHead)))

			Expect(linesAfterSummary(policyLayouts[0], text, 2)).To(Equal([]string{policyTestPrefix + "differs from HEAD.", policyDiffersWarning}))
			Expect(text).To(ContainSubstring("rule_id: " + policyTestFindingRuleID))
			Expect(text).NotTo(ContainSubstring("No active CodeSignal findings"))
		})

		It("shows only the policy line when the policy matches HEAD", func() {
			text := ReportText(findingReport(scopePolicy(true, codesignal.PolicyCauseSameAsHead)))

			Expect(linesAfterSummary(policyLayouts[0], text, 1)).To(Equal([]string{policyTestPrefix + "same as HEAD."}))
			Expect(text).NotTo(ContainSubstring("WARNING"))
		})
	})
})

var _ = Describe("ReportJSON project policy cause (#168 A2a)", func() {
	DescribeTable("serializes the cause beside path, digest and matches_head, and keeps the reason out",
		func(policy *codesignal.PolicyScope, wantCause string) {
			data, err := ReportJSON(policyReport(policyLayouts[0], policy))
			Expect(err).NotTo(HaveOccurred())

			Expect(string(data)).To(ContainSubstring(`"policy":{"path":".lousy-agents/coach.json","digest":"` + policyTestDigest +
				`","matches_head":` + strconv.FormatBool(policy.MatchesHead) + `,"cause":"` + wantCause + `"}`))
			Expect(string(data)).NotTo(ContainSubstring("invalid JSON"))
		},
		Entry("same as HEAD", scopePolicy(true, codesignal.PolicyCauseSameAsHead), "same_as_head"),
		Entry("differs from HEAD", scopePolicy(false, codesignal.PolicyCauseDiffersFromHead), "differs_from_head"),
		Entry("not committed", scopePolicy(false, codesignal.PolicyCauseNotCommitted), "not_committed"),
		Entry("missing on disk", scopePolicy(false, codesignal.PolicyCauseMissingOnDisk), "missing_on_disk"),
	)
})
