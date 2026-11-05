package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const worktreeScanIncompleteKind = "worktree_scan_incomplete"

var _ = Describe("coach codesignal --worktree guards against paths it cannot see", func() {
	skipWorktreeRepo := func() string {
		repo := goOverlayRepo()
		overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		runGitInRepo(repo, "add", "pkg/handlers/handlers.go")
		runGitInRepo(repo, "commit", "-q", "-m", "make handlers clean")
		runGitInRepo(repo, "update-index", "--skip-worktree", "pkg/handlers/handlers.go")
		overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		onDisk, err := os.ReadFile(filepath.Join(repo, "pkg/handlers/handlers.go"))
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		ExpectWithOffset(1, string(onDisk)).To(ContainSubstring(`"example.com/app/pkg/db"`),
			"the violating edit must exist on disk before the scan, or the spec proves nothing")
		return repo
	}
	scanText := func(repo string, extraArgs ...string) (string, int) {
		args := append([]string{"--project-config", "project.json"}, extraArgs...)
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
		ExpectWithOffset(1, string(stderr)).To(BeEmpty())
		return string(stdout), exitCode
	}

	When("a skip-worktree file was edited on disk (guard G1)", func() {
		It("qualifies the verdict, because git status never lists the file and its committed bytes were read", func() {
			text, exitCode := scanText(skipWorktreeRepo(), "--worktree")

			Expect(exitCode).To(Equal(0))
			Expect(text).To(ContainSubstring("analysis is incomplete: skip-worktree entries are present, so the worktree scan uses the committed version of those files, not their working-tree bytes"))
		})
	})

	When("sparse checkout is enabled (guard G2)", func() {
		It("qualifies the verdict with the sparse-checkout cause first", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "keep/a.go", "package keep\n")
			commitFile(repo, "excl/b.go", "package excl\n")
			runGitInRepo(repo, "sparse-checkout", "init", "--cone")
			runGitInRepo(repo, "sparse-checkout", "set", "keep")

			text, exitCode := scanText(repo, "--worktree")

			Expect(exitCode).To(Equal(0))
			Expect(text).To(ContainSubstring("analysis is incomplete: sparse checkout is enabled, so the worktree scan uses the committed version of the files outside the checkout, not their working-tree bytes"))
		})
	})

	When("nothing is sparse or skipped (guard G3, control)", func() {
		cleanRepo := func() string {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			return repo
		}

		It("leaves a tree with no uncommitted changes unqualified, exactly as a committed run does", func() {
			repo := cleanRepo()
			committedText, _ := scanText(repo)
			worktreeText, _ := scanText(repo, "--worktree")

			Expect(noFindingsHeadline(committedText)).To(Equal("No active CodeSignal findings."))
			Expect(noFindingsHeadline(worktreeText)).To(Equal(noFindingsHeadline(committedText)))
		})

		It("does not qualify a read edit that adds no finding", func() {
			repo := cleanRepo()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// edited on disk\n")

			worktreeText, _ := scanText(repo, "--worktree")

			Expect(noFindingsHeadline(worktreeText)).To(Equal("No active CodeSignal findings."),
				"the edit was read and holds no finding, so nothing is left to qualify")
			Expect(worktreeText).NotTo(ContainSubstring(worktreeScanIncompleteKind))
		})
	})

	When("git status cannot see some of the working tree", func() {
		It("does not claim there are no uncommitted changes when a skip-worktree file was edited", func() {
			text, _ := scanText(skipWorktreeRepo(), "--worktree")

			Expect(text).NotTo(ContainSubstring("worktree: no uncommitted changes"))
			Expect(text).To(MatchRegexp(`worktree: git status reports no uncommitted changes on revision [0-9a-f]{12}`))
		})

		It("tells the reader how to read the hidden files, for each cause", func() {
			skipText, _ := scanText(skipWorktreeRepo(), "--worktree")
			Expect(skipText).To(ContainSubstring("git update-index --no-skip-worktree"))

			sparse := newTempGitRepo()
			commitFile(sparse, "go.mod", goModuleFile)
			commitFile(sparse, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(sparse, "project.json", goLayerPolicyConfigJSON)
			commitFile(sparse, "keep/a.go", "package keep\n")
			commitFile(sparse, "excl/b.go", "package excl\n")
			runGitInRepo(sparse, "sparse-checkout", "init", "--cone")
			runGitInRepo(sparse, "sparse-checkout", "set", "keep")
			sparseText, _ := scanText(sparse, "--worktree")
			Expect(sparseText).To(ContainSubstring("git sparse-checkout disable"))

			assume := newTempGitRepo()
			commitFile(assume, "go.mod", goModuleFile)
			commitFile(assume, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(assume, "project.json", goLayerPolicyConfigJSON)
			runGitInRepo(assume, "update-index", "--assume-unchanged", "pkg/handlers/handlers.go")
			assumeText, _ := scanText(assume, "--worktree")
			Expect(assumeText).To(ContainSubstring("git update-index --no-assume-unchanged"))
		})

		It("keeps saying there are no uncommitted changes when git status can see everything (control)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			text, _ := scanText(repo, "--worktree")

			Expect(text).To(MatchRegexp(`worktree: no uncommitted changes on revision [0-9a-f]{12}`))
		})
	})

	When("--fail-on-incomplete-coverage is supplied for the skip-worktree case (guard G4)", func() {
		It("exits 3 and still writes the report", func() {
			text, exitCode := scanText(skipWorktreeRepo(), "--worktree", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3))
			Expect(text).To(ContainSubstring("analysis is incomplete: skip-worktree entries are present"))
		})
	})

	When("the same skip-worktree repository is scanned without --worktree (guard G5, control)", func() {
		It("is unchanged: the guard never touches a committed scan", func() {
			text, exitCode := scanText(skipWorktreeRepo(), "--format=json")

			Expect(exitCode).To(Equal(0))
			Expect(text).NotTo(ContainSubstring(worktreeScanIncompleteKind))
			textMode, _ := scanText(skipWorktreeRepo())
			Expect(noFindingsHeadline(textMode)).To(Equal("No active CodeSignal findings."))
		})
	})

	When("an assume-unchanged file was edited on disk (hidden from git status like skip-worktree)", func() {
		assumeUnchangedRepo := func() string {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			runGitInRepo(repo, "update-index", "--assume-unchanged", "pkg/handlers/handlers.go")
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			onDisk, err := os.ReadFile(filepath.Join(repo, "pkg/handlers/handlers.go"))
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			ExpectWithOffset(1, string(onDisk)).To(ContainSubstring(`"example.com/app/pkg/db"`),
				"the violating edit must exist on disk before the scan")
			return repo
		}
		scan := func(repo string, extraArgs ...string) (string, int) {
			stdout, _, exitCode := runCoachCodesignalBaselineRaw(repo, append([]string{"--project-config", "project.json"}, extraArgs...)...)
			return string(stdout), exitCode
		}

		It("qualifies the verdict instead of analysing the stale committed bytes as if they were the edit", func() {
			text, exitCode := scan(assumeUnchangedRepo(), "--worktree")

			Expect(exitCode).To(Equal(0))
			Expect(text).To(ContainSubstring("analysis is incomplete: assume-unchanged entries are present, so the worktree scan uses the committed version of those files, not their working-tree bytes"))
		})

		It("exits 3 under --fail-on-incomplete-coverage and still writes the report", func() {
			text, exitCode := scan(assumeUnchangedRepo(), "--worktree", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3))
			Expect(text).To(ContainSubstring("assume-unchanged"))
		})

		It("is unchanged without --worktree (control)", func() {
			text, exitCode := scan(assumeUnchangedRepo())

			Expect(exitCode).To(Equal(0))
			Expect(text).NotTo(ContainSubstring("assume-unchanged"))
		})
	})

	When("the fsmonitor-valid bit is set on a clean file (regression guard G7)", func() {
		It("is not mistaken for assume-unchanged: the verdict is the same as a committed scan's", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			runGitInRepo(repo, "add", "pkg/handlers/handlers.go")
			runGitInRepo(repo, "commit", "-q", "-m", "make handlers clean")
			runGitInRepo(repo, "update-index", "--fsmonitor-valid", "pkg/handlers/handlers.go")

			worktreeText, _, _ := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree")
			committedText, _, _ := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")

			Expect(noFindingsHeadline(string(worktreeText))).To(Equal(noFindingsHeadline(string(committedText))))
			Expect(string(worktreeText)).NotTo(ContainSubstring("assume-unchanged"))
		})
	})
})
