package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("RequiredCoverageIncomplete under --worktree", func() {
	worktreeReport := func(kind string) *codesignal.Report {
		return &codesignal.Report{
			Scope:       codesignal.Scope{Worktree: &codesignal.WorktreeScope{}},
			Diagnostics: []codesignal.Diagnostic{{Kind: kind}},
		}
	}
	committedReport := func(kind string) *codesignal.Report {
		return &codesignal.Report{Diagnostics: []codesignal.Diagnostic{{Kind: kind}}}
	}

	DescribeTable("is incomplete under --worktree for each of the four kinds that say a part was not read",
		func(kind string) {
			Expect(codesignal.RequiredCoverageIncomplete(worktreeReport(kind))).To(BeTrue())
		},
		Entry("a Git state the scan cannot see through", codesignal.DiagKindWorktreeScanIncomplete),
		Entry("a symbolic link the scan skipped", codesignal.DiagKindWorktreeSymlinkSkipped),
		Entry("a changed path nothing analyzed", codesignal.DiagKindWorktreeChangesNotAnalyzed),
		Entry("a working-tree status read that failed", codesignal.DiagKindWorktreeStatusCheckFailed),
	)

	DescribeTable("leaves the same kinds alone when the scan is not a --worktree scan, which describe a committed scan of a dirty tree there",
		func(kind string) {
			Expect(codesignal.RequiredCoverageIncomplete(committedReport(kind))).To(BeFalse())
		},
		Entry("a Git state the scan cannot see through", codesignal.DiagKindWorktreeScanIncomplete),
		Entry("a symbolic link the scan skipped", codesignal.DiagKindWorktreeSymlinkSkipped),
		Entry("a changed path nothing analyzed", codesignal.DiagKindWorktreeChangesNotAnalyzed),
		Entry("a working-tree status read that failed", codesignal.DiagKindWorktreeStatusCheckFailed),
	)

	DescribeTable("leaves a --worktree verdict qualified for another reason at exit 0",
		func(kind string) {
			Expect(codesignal.RequiredCoverageIncomplete(worktreeReport(kind))).To(BeFalse())
		},
		Entry("a file in a language the scan does not analyze", "unsupported_language"),
		Entry("a rename whose continuity was not determined", "continuity_not_determined"),
		Entry("a dirty tree the report lists without reading", codesignal.DiagKindWorktreeNotClean),
		Entry("a report that reflects the committed HEAD", codesignal.DiagKindWorktreeReportReflectsCommittedHEAD),
	)
})
