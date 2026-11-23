package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func hiddenLine(label string) string {
	return `(?m)^[^\n]*` + label + `[^\n]*(\bno\b[^\n]*in-scope|in-scope[^\n]*\bno\b)[^\n]*$`
}

var _ = Describe("coach codesignal --worktree names each cause that hides tracked paths from git status (#168 item 22)", func() {
	DescribeTable("an in-scope Go file hidden by git qualifies the verdict and exits 3 (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{"lib/in_scope.go": "package lib\n"})
			m.hide(repo, "lib/in_scope.go")

			run := runProjectScan(repo)
			text := runProjectText(repo)

			entry := run.only(m.reason)
			Expect(entry).To(HaveKeyWithValue("kind", worktreeScanIncompleteKind))
			Expect(entry).To(HaveKeyWithValue("root", "lib"))
			Expect(entry).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(pathOf(entry)).To(BeEmpty(), "a diagnostic path would inflate summary.files_unanalyzed")
			Expect(run.exitCode).To(Equal(3))
			Expect(noFindingsHeadline(text)).NotTo(Equal(unqualifiedAllClear))
			Expect(text).To(ContainSubstring("the analysis is incomplete"))
			Expect(text).To(ContainSubstring(m.hint))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("a hidden path that is not in-scope source is disclosed without qualifying the verdict (#168 item 22)",
		func(m hiding, hidden, contents, root string) {
			repo := scanRepoWith(map[string]string{hidden: contents})
			m.hide(repo, hidden)

			run := runProjectScan(repo)
			text := runProjectText(repo)

			entry := run.only(m.reason)
			Expect(entry).To(HaveKeyWithValue("root", root))
			Expect(entry).To(HaveKeyWithValue("hides_in_scope", "no"))
			Expect(pathOf(entry)).To(BeEmpty())
			Expect(run.exitCode).To(Equal(0))
			Expect(noFindingsHeadline(text)).To(And(HavePrefix("No active CodeSignal findings"), Not(ContainSubstring("incomplete"))),
				"a layer-reach suffix may follow the all-clear; the verdict is still not qualified")
			Expect(text).To(MatchRegexp(hiddenLine(m.label)))
			Expect(text).To(ContainSubstring(root + "/"))
			Expect(text).To(ContainSubstring(m.hint))
		},
		Entry("sparse checkout hiding a markdown file", sparseHiding, "docs/guide.md", "# guide\n", "docs"),
		Entry("skip-worktree hiding a markdown file", skipWorktreeHiding, "docs/guide.md", "# guide\n", "docs"),
		Entry("assume-unchanged hiding a markdown file", assumeUnchangedHiding, "docs/guide.md", "# guide\n", "docs"),
		Entry("sparse checkout hiding a Go test file under --scope production", sparseHiding, "lib/lib_test.go", "package lib\n", "lib"),
		Entry("skip-worktree hiding a Go test file under --scope production", skipWorktreeHiding, "lib/lib_test.go", "package lib\n", "lib"),
		Entry("assume-unchanged hiding a Go test file under --scope production", assumeUnchangedHiding, "lib/lib_test.go", "package lib\n", "lib"),
	)

	DescribeTable("--scope all alone flips a hidden Go test file from no to yes (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{"lib/lib_test.go": "package lib\n"})
			m.hide(repo, "lib/lib_test.go")

			production := runProjectScan(repo)
			everything := runProjectScan(repo, "--scope", "all")

			Expect(production.only(m.reason)).To(HaveKeyWithValue("hides_in_scope", "no"))
			Expect(production.exitCode).To(Equal(0))
			Expect(everything.only(m.reason)).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(everything.exitCode).To(Equal(3))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("a hidden go.mod, which the overlay reads as project configuration, counts as in-scope (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(nil)
			m.hide(repo, "go.mod")

			run := runProjectScan(repo)

			entry := run.only(m.reason)
			Expect(entry).To(HaveKeyWithValue("root", "."))
			Expect(entry).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(run.exitCode).To(Equal(3))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	It("reports hidden S entries of a sparse checkout under sparse_checkout only, not also as skip_worktree (#168 item 22)", func() {
		repo := scanRepoWith(map[string]string{"lib/in_scope.go": "package lib\n"})
		sparseHiding.hide(repo, "lib/in_scope.go")

		run := runProjectScan(repo)

		Expect(run.only("sparse_checkout")).To(HaveKeyWithValue("root", "lib"))
		Expect(run.ofReason("skip_worktree")).To(BeEmpty())
	})

	It("discloses a sparse checkout that hides nothing as one entry with no root, without qualifying the verdict (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		sparseHiding.hide(repo)

		run := runProjectScan(repo)
		text := runProjectText(repo)

		entry := run.only("sparse_checkout")
		Expect(entry).NotTo(HaveKey("root"))
		Expect(entry).To(HaveKeyWithValue("hides_in_scope", "no"))
		Expect(run.exitCode).To(Equal(0))
		Expect(noFindingsHeadline(text)).To(Equal(unqualifiedAllClear))
		Expect(text).To(MatchRegexp(hiddenLine("sparse checkout")))
	})
})
