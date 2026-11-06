package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --worktree treats a file removed from the index but kept on disk as one unchanged path", func() {
	modelRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
		return repo
	}
	coverageOf := func(repo string) codesignal.Coverage {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		return *decodeCoachReport(stdout).Coverage
	}
	lifecyclesAt := func(report *codesignal.Report, path string) []codesignal.Lifecycle {
		lifecycles := []codesignal.Lifecycle{}
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == path {
				lifecycles = append(lifecycles, signal.Lifecycle)
			}
		}
		return lifecycles
	}

	It("keeps the file's signals existing under --base when its bytes did not change", func() {
		repo := modelRepo()
		runGitInRepo(repo, "rm", "-q", "--cached", "pkg/model/m.go")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		lifecycles := lifecyclesAt(decodeCoachReport(stdout), "pkg/model/m.go")
		Expect(lifecycles).NotTo(BeEmpty())
		Expect(lifecycles).To(HaveEach(codesignal.Lifecycle("existing")),
			"the file is byte-identical to HEAD, so it is neither introduced nor resolved")
	})

	It("counts the path once, as untracked, in scope.worktree", func() {
		repo := modelRepo()
		runGitInRepo(repo, "rm", "-q", "--cached", "pkg/model/m.go")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		counts := decodeCoachReport(stdout).Scope.Worktree
		Expect(counts).NotTo(BeNil())
		Expect([]int{counts.Modified, counts.Added, counts.Deleted, counts.Untracked}).To(Equal([]int{0, 0, 0, 1}),
			"one path, and an untracked path is checked first")
	})

	It("analyzes the file's working-tree bytes in a baseline scan without discovering it twice", func() {
		repo := modelRepo()
		control := coverageOf(repo)
		runGitInRepo(repo, "rm", "-q", "--cached", "pkg/model/m.go")

		coverage := coverageOf(repo)

		Expect(coverage.TrackedFilesDiscovered).To(Equal(control.TrackedFilesDiscovered))
		Expect(coverage.FilesAnalyzed).To(Equal(control.FilesAnalyzed))
	})
})
