package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (run worktreeScanRun) hidesInScopeByRoot(reason string) map[string]any {
	byRoot := map[string]any{}
	for _, entry := range run.listed(reason) {
		byRoot[entry["root"].(string)] = entry["hides_in_scope"]
	}
	return byRoot
}

var _ = Describe("coach codesignal --worktree decides hides_in_scope from each root's own hidden paths (#168 item 22)", func() {
	DescribeTable("one reason at two roots reports no for the markdown root and yes for the Go root (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{"docs/guide.md": "# guide\n", "lib/in_scope.go": "package lib\n"})
			m.hide(repo, "docs/guide.md", "lib/in_scope.go")

			run := runProjectScan(repo)
			text := runProjectText(repo)

			Expect(run.ofReason(m.reason)).To(HaveLen(2))
			Expect(run.hidesInScopeByRoot(m.reason)).To(Equal(map[string]any{"docs": "no", "lib": "yes"}))
			Expect(run.exitCode).To(Equal(3), "the lib root alone qualifies the verdict")
			Expect(noFindingsHeadline(text)).NotTo(Equal(unqualifiedAllClear))
			Expect(text).To(ContainSubstring(m.hint))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("hiding only the markdown root beside a visible Go root stays unqualified (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{"docs/guide.md": "# guide\n", "lib/in_scope.go": "package lib\n"})
			m.hide(repo, "docs/guide.md")

			run := runProjectScan(repo)
			text := runProjectText(repo)

			Expect(run.ofReason(m.reason)).To(HaveLen(1))
			Expect(run.hidesInScopeByRoot(m.reason)).To(Equal(map[string]any{"docs": "no"}))
			Expect(run.exitCode).To(Equal(0))
			Expect(noFindingsHeadline(text)).To(And(HavePrefix("No active CodeSignal findings"), Not(ContainSubstring("incomplete"))))
			Expect(text).To(MatchRegexp(hiddenLine(m.label)))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("a markdown root, a Go root, and a test-only root under the default scope are each decided alone (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{
				"docs/guide.md":   "# guide\n",
				"lib/in_scope.go": "package lib\n",
				"svc/svc_test.go": "package svc\n",
			})
			m.hide(repo, "docs/guide.md", "lib/in_scope.go", "svc/svc_test.go")

			run := runProjectScan(repo)
			everything := runProjectScan(repo, "--scope", "all")

			Expect(run.ofReason(m.reason)).To(HaveLen(3))
			Expect(run.hidesInScopeByRoot(m.reason)).To(Equal(map[string]any{"docs": "no", "lib": "yes", "svc": "no"}))
			Expect(run.exitCode).To(Equal(3))
			Expect(everything.hidesInScopeByRoot(m.reason)).To(Equal(map[string]any{"docs": "no", "lib": "yes", "svc": "yes"}),
				"--scope all alone flips the test-only root")
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)
})
