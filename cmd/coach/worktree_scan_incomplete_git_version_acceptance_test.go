//go:build unix

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree under a git that cannot be told not to fetch names each submodule root (#168 item 22)", func() {
	It("keeps the verdict unqualified and names git 2.43.0 and the submodule in a disclosed line (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addLibrarySubmodule(repo, "third/lib")
		dirtySubmodule(repo, "third/lib")

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, envWithGitBefore244(),
			"codesignal", "--baseline", "--worktree", "--project-config", "project.json")

		Expect(string(stderr)).To(BeEmpty())
		Expect(exitCode).To(Equal(0))
		Expect(noFindingsHeadline(string(stdout))).To(Equal(unqualifiedAllClear))
		Expect(string(stdout)).To(MatchRegexp(hiddenLine("git 2.43.0")))
		Expect(string(stdout)).To(ContainSubstring("third/lib/"))
	})

	It("gives one git_version entry per submodule root, clean or dirty, sorted by root (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addLibrarySubmodule(repo, "third/other")
		addLibrarySubmodule(repo, "third/lib")
		dirtySubmodule(repo, "third/lib")

		run := runWorktreeScan(repo, envWithGitBefore244(), "--project-config", "project.json")

		Expect(run.roots("git_version")).To(Equal([]string{"third/lib", "third/other"}))
		Expect(run.ofReason("git_version")).To(HaveEach(HaveKeyWithValue("hides_in_scope", "no")))
		Expect(run.ofReason("submodule")).To(BeEmpty())
		Expect(run.exitCode).To(Equal(0))
	})

	It("runs the path of a submodule named like a Go file through the scope filter (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addLibrarySubmodule(repo, "third/x.go")
		addLibrarySubmodule(repo, "third/lib")

		run := runWorktreeScan(repo, envWithGitBefore244(), "--project-config", "project.json")

		Expect(run.ofReason("git_version")).To(ConsistOf(
			And(HaveKeyWithValue("root", "third/lib"), HaveKeyWithValue("hides_in_scope", "no")),
			And(HaveKeyWithValue("root", "third/x.go"), HaveKeyWithValue("hides_in_scope", "yes")),
		))
		Expect(run.exitCode).To(Equal(3))
	})
})
