package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --base --worktree compares against the merge-base of a ref that has diverged from HEAD", func() {
	It("diffs from the merge-base, so work that exists only on the other branch is not part of the change", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		mergeBase := commitFile(repo, "pkg/model/seed.go", "package model\n")
		start := currentBranch(repo)
		runGitInRepo(repo, "checkout", "-q", "-b", "feature")
		commitFile(repo, "pkg/model/featureonly.go", modelFileWithTwoConstructors)
		runGitInRepo(repo, "checkout", "-q", start)
		commitFile(repo, "pkg/model/headonly.go", modelFileWithTwoConstructors)
		writeUntrackedFile(repo, "pkg/model/fresh.go", modelFileWithTwoConstructors)
		Expect(runGitInRepo(repo, "merge-base", "feature", "HEAD")).To(Equal(mergeBase))
		Expect(runGitInRepo(repo, "rev-parse", "feature")).NotTo(Equal(mergeBase),
			"the ref must be ahead of the merge-base, or the spec cannot tell the two apart")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "feature", "--worktree", "--format=json")
		report := decodeCoachReport(stdout)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(report.Scope.Base).To(Equal(mergeBase), "scope.base names the revision the comparison started from")
		Expect(lifecyclesAt(report, "pkg/model/headonly.go")).To(HaveEach(codesignal.Lifecycle("introduced")))
		Expect(lifecyclesAt(report, "pkg/model/headonly.go")).NotTo(BeEmpty())
		Expect(lifecyclesAt(report, "pkg/model/fresh.go")).To(HaveEach(codesignal.Lifecycle("introduced")))
		Expect(lifecyclesAt(report, "pkg/model/fresh.go")).NotTo(BeEmpty())
		Expect(lifecyclesAt(report, "pkg/model/featureonly.go")).To(BeEmpty(),
			"a file that exists only on the other branch is neither in HEAD nor in the working tree")
	})
})
