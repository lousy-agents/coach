package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree names both causes of a path git tags skip-worktree and assume-unchanged (#168 item 22)", func() {
	It("gives one entry under each of skip_worktree and assume_unchanged, both yes at the same root (#168 item 22)", func() {
		repo := scanRepoWith(map[string]string{"lib/in_scope.go": "package lib\n"})
		runGitInRepo(repo, "update-index", "--skip-worktree", "--assume-unchanged", "--", "lib/in_scope.go")

		run := runProjectScan(repo)

		Expect(run.only("skip_worktree")).To(And(HaveKeyWithValue("root", "lib"), HaveKeyWithValue("hides_in_scope", "yes")))
		Expect(run.only("assume_unchanged")).To(And(HaveKeyWithValue("root", "lib"), HaveKeyWithValue("hides_in_scope", "yes")))
		Expect(run.exitCode).To(Equal(3))
	})
})
