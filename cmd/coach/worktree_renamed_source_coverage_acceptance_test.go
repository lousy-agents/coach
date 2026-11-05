package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// The baseline counts the destination of a staged rename as a new file, so the
// source has to leave the discovered files and the unsupported-language groups,
// whatever its language.
var _ = Describe("coach codesignal --baseline --worktree takes the source of a staged rename out of its coverage", func() {
	scriptRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
		commitFile(repo, "scripts/tool.sh", "package tools\n")
		return repo
	}
	coverageOf := func(repo string) codesignal.Coverage {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		return *decodeCoachReport(stdout).Coverage
	}
	accountedFor := func(coverage codesignal.Coverage) int {
		return coverage.FilesAnalyzed + groupedFiles(coverage.Unsupported) + groupedFiles(coverage.Excluded) + coverage.FilesUnanalyzable
	}

	It("stops counting a renamed-away unsupported file under its old extension", func() {
		repo := scriptRepo()
		control := coverageOf(repo)
		Expect(unsupportedLanguages(control)).To(ContainElement(".sh"))
		runGitInRepo(repo, "mv", "scripts/tool.sh", "scripts/tool.txt")
		expectDirtyPorcelain(repo, "R  scripts/tool.sh -> scripts/tool.txt")

		coverage := coverageOf(repo)

		Expect(unsupportedLanguages(coverage)).NotTo(ContainElement(".sh"))
		Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
	})

	It("counts a file renamed from an unsupported to a supported extension once, at its new path", func() {
		repo := scriptRepo()
		control := coverageOf(repo)
		runGitInRepo(repo, "mv", "scripts/tool.sh", "scripts/tool.go")
		expectDirtyPorcelain(repo, "R  scripts/tool.sh -> scripts/tool.go")

		coverage := coverageOf(repo)

		Expect(unsupportedLanguages(coverage)).NotTo(ContainElement(".sh"))
		Expect(coverage.FilesAnalyzed).To(Equal(control.FilesAnalyzed + 1))
		Expect(coverage.TrackedFilesDiscovered).To(Equal(control.TrackedFilesDiscovered),
			"one path left and one arrived")
		Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
	})
})
