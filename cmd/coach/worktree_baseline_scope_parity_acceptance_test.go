package main

import (
	"regexp"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func scanBaselineTwins(committed, worktree parityRepo, flags ...string) (parityScan, parityScan) {
	return scanBaselineParity(committed, flags...), scanBaselineParity(worktree, slices.Concat(flags, []string{"--worktree"})...)
}

func coverageSectionLines(text string) []string {
	_, section, _ := strings.Cut(text, "\nCoverage:\n")
	return strings.Split(strings.TrimRight(section, "\n"), "\n")
}

var (
	baselineHeadline        = regexp.MustCompile(`(?m)^Repository Baseline for revision .*$`)
	baselineDiscoveredLabel = regexp.MustCompile(`(?m)^(tracked files discovered|files discovered \(tracked and new\)): `)
)

func baselineTextBody(text string) string {
	body := baselineHeadline.ReplaceAllString(text, "headline")
	return baselineDiscoveredLabel.ReplaceAllString(body, "files discovered: ")
}

func expectSameBaselineScopeOutcome(committed, worktree parityScan) {
	ExpectWithOffset(1, worktree.report.Scope.AppliedScope).To(Equal(committed.report.Scope.AppliedScope))
	ExpectWithOffset(1, worktree.report.Signals).To(Equal(committed.report.Signals))
	ExpectWithOffset(1, worktree.report.Summary).To(Equal(committed.report.Summary))
	ExpectWithOffset(1, worktree.report.Diagnostics).To(Equal(committed.report.Diagnostics))
	ExpectWithOffset(1, worktree.report.Coverage).To(Equal(committed.report.Coverage))
	ExpectWithOffset(1, baselineTextBody(worktree.text)).To(Equal(baselineTextBody(committed.text)))
}

func expectBaselineWorktreeScope(committed, worktree parityScan, worktreeRevision string) {
	ExpectWithOffset(1, committed.report.Scope.Worktree).To(BeNil(), "the committed run is not a --worktree scan")
	ExpectWithOffset(1, worktree.report.Scope.Worktree).NotTo(BeNil(), "the second run must be the --worktree scan")
	ExpectWithOffset(1, worktree.report.Scope.Revision).To(Equal(worktreeRevision))
	ExpectWithOffset(1, *worktree.report.Scope.Worktree).To(Equal(codesignal.WorktreeScope{
		Revision:    worktreeRevision,
		Modified:    2,
		ConfigFiles: []string{},
	}))
}

var _ = Describe("coach codesignal --baseline --worktree behaves as the committed baseline does for --scope (#168 item 23)", func() {
	When("--baseline --scope production is given for production and test files", func() {
		It("excludes the test file and keeps the production signal, committed and in the working tree alike", func() {
			committedRepo, worktreeRepo := twinRepos(seedShippingRepo, changeShippingFiles)

			committed, worktree := scanBaselineTwins(committedRepo, worktreeRepo, "--scope", "production", "--build-target", "./cmd/app")

			for _, scan := range []parityScan{committed, worktree} {
				Expect(scan.report.Scope.AppliedScope).To(Equal("production"))
				Expect(paritySignalPaths(scan.report)).To(Equal([]string{"shipping/shipping.go"}))
				Expect(sourceScopeForPath(scan.json, "shipping/shipping.go")).To(Equal("production"))
				Expect(scan.report.Summary.ActiveSignals).To(Equal(1))
				Expect(scan.report.Coverage.Excluded).To(Equal([]codesignal.CoverageGroup{{Reason: "test_only", Language: "go", Count: 1}}))
				Expect(coverageSectionLines(scan.text)).To(Equal([]string{"  unsupported: 1 .mod files", "  excluded: 1 test_only go files"}))
				Expect(scan.text).NotTo(ContainSubstring("shipping_test.go"))
			}
			expectSameBaselineScopeOutcome(committed, worktree)
			expectBaselineWorktreeScope(committed, worktree, worktreeRepo.base)
		})
	})

	When("--baseline --scope all is given for production and test files", func() {
		It("includes the test-only signal alongside the production signal, committed and in the working tree alike", func() {
			committedRepo, worktreeRepo := twinRepos(seedShippingRepo, changeShippingFiles)

			committed, worktree := scanBaselineTwins(committedRepo, worktreeRepo, "--scope", "all", "--build-target", "./cmd/app")

			for _, scan := range []parityScan{committed, worktree} {
				Expect(scan.report.Scope.AppliedScope).To(Equal("all"))
				Expect(paritySignalPaths(scan.report)).To(ConsistOf("shipping/shipping.go", "shipping/shipping_test.go"))
				Expect(sourceScopeForPath(scan.json, "shipping/shipping_test.go")).To(Equal("test_only"))
				Expect(scan.report.Summary.ActiveSignals).To(Equal(2))
				Expect(scan.report.Coverage.Excluded).To(BeEmpty())
				Expect(coverageSectionLines(scan.text)).To(Equal([]string{"  unsupported: 1 .mod files"}))
				Expect(scan.text).To(ContainSubstring("path: shipping/shipping_test.go"))
			}
			expectSameBaselineScopeOutcome(committed, worktree)
			expectBaselineWorktreeScope(committed, worktree, worktreeRepo.base)
		})

		It("differs from the production run on the same repository, so neither baseline row can pass by accident", func() {
			committedRepo, worktreeRepo := twinRepos(seedShippingRepo, changeShippingFiles)

			productionCommitted, productionWorktree := scanBaselineTwins(committedRepo, worktreeRepo, "--scope", "production", "--build-target", "./cmd/app")
			allCommitted, allWorktree := scanBaselineTwins(committedRepo, worktreeRepo, "--scope", "all", "--build-target", "./cmd/app")

			for _, production := range []parityScan{productionCommitted, productionWorktree} {
				Expect(signalsForPath(production.report, "shipping/shipping_test.go")).To(BeEmpty())
			}
			for _, all := range []parityScan{allCommitted, allWorktree} {
				Expect(signalsForPath(all.report, "shipping/shipping_test.go")).To(HaveLen(1))
			}
		})
	})
})
