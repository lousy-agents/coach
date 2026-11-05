package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func groupedFiles(groups []codesignal.CoverageGroup) int {
	total := 0
	for _, group := range groups {
		total += group.Count
	}
	return total
}

func unsupportedLanguages(coverage codesignal.Coverage) []string {
	languages := []string{}
	for _, group := range coverage.Unsupported {
		languages = append(languages, group.Language)
	}
	return languages
}

var _ = Describe("coach codesignal --worktree reports coverage counts that add up", func() {
	coverageOf := func(repo string) codesignal.Coverage {
		return *scanGoBaseline(repo, "--worktree").Coverage
	}
	accountedFor := func(coverage codesignal.Coverage) int {
		return coverage.FilesAnalyzed + groupedFiles(coverage.Unsupported) + groupedFiles(coverage.Excluded) + coverage.FilesUnanalyzable
	}

	It("accounts for every discovered file when nothing changed (control)", func() {
		coverage := coverageOf(goOverlayRepo())

		Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
	})

	When("go.mod is deleted from the working tree", func() {
		It("does not list the deleted file as a discovered unsupported file", func() {
			repo := goOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "go.mod"))).To(Succeed())

			coverage := coverageOf(repo)

			Expect(unsupportedLanguages(coverage)).NotTo(ContainElement(".mod"))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
		})
	})

	When("a supported source file is deleted from the working tree", func() {
		It("no longer counts the file as discovered", func() {
			repo := goOverlayRepo()
			control := coverageOf(repo)
			Expect(os.Remove(filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())

			coverage := coverageOf(repo)

			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.TrackedFilesDiscovered - 1))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
		})
	})

	When("a supported source file is replaced by a symbolic link", func() {
		It("counts the skipped path as unanalyzable", func() {
			repo := goOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())
			Expect(os.Symlink("../db/db.go", filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())

			coverage := coverageOf(repo)

			Expect(coverage.FilesUnanalyzable).To(Equal(1))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
		})
	})

	When("a new supported source file is added to the working tree", func() {
		It("counts the file as discovered as well as analyzed", func() {
			repo := goOverlayRepo()
			control := coverageOf(repo)
			Expect(os.WriteFile(filepath.Join(repo, "pkg/handlers/new.go"), []byte("package handlers\n"), 0o644)).To(Succeed())

			coverage := coverageOf(repo)

			Expect(coverage.FilesAnalyzed).To(Equal(control.FilesAnalyzed + 1))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
		})
	})
	When("new files of an unsupported language are added to the working tree", func() {
		unsupportedCount := func(coverage codesignal.Coverage, language string) int {
			for _, group := range coverage.Unsupported {
				if group.Language == language {
					return group.Count
				}
			}
			return 0
		}

		It("counts an untracked, a staged and an intent-to-add file under their extensions, as the committed result will", func() {
			repo := goOverlayRepo()
			control := coverageOf(repo)
			writeUntrackedFile(repo, "docs/notes.md", "# notes\n")
			stageWorktreeFile(repo, "scripts/run.sh", "echo hi\n")
			intentToAddWorktreeFile(repo, "scripts/more.sh", "echo more\n")

			coverage := coverageOf(repo)

			Expect(unsupportedCount(coverage, ".md")).To(Equal(unsupportedCount(control, ".md") + 1))
			Expect(unsupportedCount(coverage, ".sh")).To(Equal(unsupportedCount(control, ".sh") + 2))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.TrackedFilesDiscovered + 3))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
		})

		It("adds to the group of a committed file with the same extension", func() {
			repo := goOverlayRepo()
			control := coverageOf(repo)
			Expect(unsupportedCount(control, ".json")).To(BeNumerically(">", 0), "project.json is committed")
			writeUntrackedFile(repo, "extra.json", "{}\n")

			coverage := coverageOf(repo)

			Expect(unsupportedCount(coverage, ".json")).To(Equal(unsupportedCount(control, ".json") + 1))
			Expect(unsupportedLanguages(coverage)).To(Equal(unsupportedLanguages(control)), "no second group, and the order of the groups is kept")
		})

		It("does not count a committed file twice when it is only removed from the index", func() {
			repo := goOverlayRepo()
			control := coverageOf(repo)
			runGitInRepo(repo, "rm", "--cached", "-q", "project.json")

			coverage := coverageOf(repo)

			Expect(unsupportedCount(coverage, ".json")).To(Equal(unsupportedCount(control, ".json")))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.TrackedFilesDiscovered))
		})
	})

	When("a tracked file that is not analyzed is deleted from the working tree", func() {
		It("no longer counts a deleted non-source file as discovered or unsupported", func() {
			repo := goOverlayRepo()
			commitFile(repo, "README.md", "# app\n")
			control := coverageOf(repo)
			Expect(unsupportedLanguages(control)).To(ContainElement(".md"))
			Expect(os.Remove(filepath.Join(repo, "README.md"))).To(Succeed())

			coverage := coverageOf(repo)

			Expect(unsupportedLanguages(coverage)).NotTo(ContainElement(".md"))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.TrackedFilesDiscovered - 1))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
		})

		It("no longer counts a deleted file that --scope excludes", func() {
			repo := goOverlayRepo()
			commitFile(repo, "pkg/handlers/handlers_test.go", "package handlers\n")
			control := coverageOf(repo)
			Expect(groupedFiles(control.Excluded)).To(Equal(1), "the test file is excluded from the production scope")
			Expect(os.Remove(filepath.Join(repo, "pkg/handlers/handlers_test.go"))).To(Succeed())

			coverage := coverageOf(repo)

			Expect(groupedFiles(coverage.Excluded)).To(BeZero())
			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.TrackedFilesDiscovered - 1))
			Expect(coverage.TrackedFilesDiscovered).To(Equal(accountedFor(coverage)))
		})
	})
})
