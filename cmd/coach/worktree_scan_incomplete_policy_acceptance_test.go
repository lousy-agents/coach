package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree counts the selected policy file as in-scope whether it was named or discovered (#168 item 22)", func() {
	DescribeTable("the named policy file, hidden by git, counts as in-scope (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(nil)
			m.hide(repo, "project.json")

			run := runProjectScan(repo)

			Expect(run.only(m.reason)).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(run.exitCode).To(Equal(3))
		},
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("the discovered policy file, hidden by git, counts as in-scope (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{discoveredPolicyPath: goLayerPolicyConfigJSON})
			m.hide(repo, discoveredPolicyPath)

			run := runWorktreeScan(repo, nil)

			Expect(run.only(m.reason)).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(run.exitCode).To(Equal(3))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("a hidden project.json is not in-scope when discovery selects .lousy-agents/coach.json instead (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{discoveredPolicyPath: goLayerPolicyConfigJSON})
			m.hide(repo, "project.json")

			run := runWorktreeScan(repo, nil)

			Expect(run.only(m.reason)).To(HaveKeyWithValue("hides_in_scope", "no"))
			Expect(run.exitCode).To(Equal(0))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("a hidden .lousy-agents/coach.json is not in-scope when --project-config names project.json instead (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(map[string]string{discoveredPolicyPath: goLayerPolicyConfigJSON})
			m.hide(repo, discoveredPolicyPath)

			run := runProjectScan(repo)

			Expect(run.only(m.reason)).To(HaveKeyWithValue("hides_in_scope", "no"))
			Expect(run.exitCode).To(Equal(0))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)
})
