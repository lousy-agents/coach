package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree names the directory that hides each path, never a file (#168 item 22)", func() {
	DescribeTable("collapses a hidden subtree to its top directory and a mixed directory to itself (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{
				"vendor/a/x.go":    "package a\n",
				"vendor/b/y.go":    "package b\n",
				"mixed/hidden.go":  "package mixed\n",
				"mixed/visible.go": "package mixed\n",
			})
			m.hide(repo, "vendor/a/x.go", "vendor/b/y.go", "mixed/hidden.go")

			run := runProjectScan(repo)

			Expect(run.roots(m.reason)).To(Equal([]string{"mixed", "vendor"}))
			Expect(run.ofReason(m.reason)).To(HaveLen(2))
			Expect(run.roots(m.reason)).To(HaveEach(Not(BeElementOf(trackedFiles(repo)))), "a root is a directory, never a tracked file")
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("gives the repository root as \".\" when every tracked file is hidden (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(nil)
			m.hide(repo, trackedFiles(repo)...)

			run := runWorktreeScan(repo, nil)

			entry := run.only(m.reason)
			Expect(entry).To(HaveKeyWithValue("root", "."), "present, unlike the root of a sparse checkout that hides nothing")
			Expect(entry).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(run.exitCode).To(Equal(3))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)
})
