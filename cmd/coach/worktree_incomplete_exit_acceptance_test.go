package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree exits 3 when part of the working tree could not be read", func() {
	When("a path the scan could not read is qualified in the verdict", func() {
		It("exits 3 under --fail-on-incomplete-coverage for an unreadable file", func() {
			if os.Geteuid() == 0 {
				Skip("root reads files regardless of their mode")
			}
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// edited\n")
			path := filepath.Join(repo, "pkg/handlers/handlers.go")
			Expect(os.Chmod(path, 0o000)).To(Succeed())
			DeferCleanup(os.Chmod, path, os.FileMode(0o644))

			text, _, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree")
			_, stderr, failExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree", "--fail-on-incomplete-coverage")

			Expect(textExit).To(Equal(0))
			Expect(string(text)).To(ContainSubstring("the analysis is incomplete"))
			Expect(failExit).To(Equal(3), "stderr: %s", stderr)
		})

		It("exits 3 for a symbolic link in a project-policy scan, where the dropped file also lowers project coverage", func() {
			repo := goOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())
			Expect(os.Symlink("../db/db.go", filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("exits 3 for an unreadable file when no project policy is configured", func() {
			if os.Geteuid() == 0 {
				Skip("root reads files regardless of their mode")
			}
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/model/m.go", "package model\n")
			overwriteOnDisk(repo, "pkg/model/m.go", "package model\n\nvar Value = 1\n")
			path := filepath.Join(repo, "pkg/model/m.go")
			Expect(os.Chmod(path, 0o000)).To(Succeed())
			DeferCleanup(os.Chmod, path, os.FileMode(0o644))

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("exits 3 for a symbolic link when no project policy is configured", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/model/other.go", "package model\n")
			commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 1\n")
			Expect(os.Remove(filepath.Join(repo, "pkg/model/m.go"))).To(Succeed())
			Expect(os.Symlink("other.go", filepath.Join(repo, "pkg/model/m.go"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).To(BeNil(), "without a project policy, project coverage cannot be what makes the scan incomplete")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_symlink_skipped")).To(ConsistOf(HaveField("Path", "pkg/model/m.go")))
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("exits 0 under --fail-on-incomplete-coverage when the scan read every changed file (control)", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// edited\n")

			text, _, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree")
			_, stderr, failExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree", "--fail-on-incomplete-coverage")

			Expect(textExit).To(Equal(0))
			Expect(noFindingsHeadline(string(text))).To(Equal("No active CodeSignal findings."))
			Expect(failExit).To(Equal(0), "a verdict that is not qualified must not exit 3; stderr: %s", stderr)
		})

		It("leaves a committed scan of a tree with a dirty file at exit 0 (control)", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// edited\n")

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		})
	})

	When("a verdict is qualified without any part of the working tree being unread", func() {
		diffRepo := func() string {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 1\n")
			return repo
		}
		failOnIncomplete := func(repo string) (text string, exitCode int) {
			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--worktree", "--fail-on-incomplete-coverage")
			ExpectWithOffset(1, string(stdout)).To(ContainSubstring("the analysis is incomplete"),
				"the verdict must be qualified for this spec to prove the exit status ignores the qualification; stderr: %s", stderr)
			return string(stdout), exitCode
		}

		It("exits 0 when the only qualification is an untracked file in a language the scan does not analyze", func() {
			repo := diffRepo()
			writeUntrackedFile(repo, "README.md", "# notes\n")

			text, exitCode := failOnIncomplete(repo)

			Expect(text).To(ContainSubstring("README.md"))
			Expect(exitCode).To(Equal(0))
		})

		It("exits 0 when the only qualification is a staged rename whose continuity is undetermined", func() {
			repo := diffRepo()
			runGitInRepo(repo, "mv", "pkg/model/m.go", "pkg/model/n.go")

			text, exitCode := failOnIncomplete(repo)

			Expect(text).To(ContainSubstring("pkg/model/n.go"))
			Expect(exitCode).To(Equal(0))
		})

		It("exits 3 when the qualification is a symbolic link the scan skipped (control)", func() {
			repo := diffRepo()
			writeUntrackedFile(repo, "pkg/model/other.go", "package model\n")
			Expect(os.Remove(filepath.Join(repo, "pkg/model/m.go"))).To(Succeed())
			Expect(os.Symlink("other.go", filepath.Join(repo, "pkg/model/m.go"))).To(Succeed())

			text, exitCode := failOnIncomplete(repo)

			Expect(text).To(ContainSubstring("pkg/model/m.go"))
			Expect(exitCode).To(Equal(3))
		})
	})

	When("a user reads the help text of --fail-on-incomplete-coverage", func() {
		It("describes only what the committed scan does, until --worktree graduates from experimental", func() {
			stdout, stderr, exitCode := runCoachRaw("codesignal", "--help")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			help := string(stdout) + string(stderr)
			Expect(help).To(ContainSubstring("exit 3 when required project coverage (model or bypass, on any analyzed side) is incomplete; the report is still written to stdout"))
			Expect(help).NotTo(ContainSubstring("working tree"), "the hidden mode's rule is not part of the documented help")
			Expect(help).NotTo(ContainSubstring("-worktree"))
		})
	})

	When("the overlay dropped a file inside the project's roots", func() {
		It("reports project coverage as incomplete, because the package count fell with the file", func() {
			repo := goOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())
			Expect(os.Symlink("../db/db.go", filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())

			report := scanGoBaseline(repo, "--worktree")

			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
		})

		It("reports project coverage as complete when the overlay read everything (control)", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport)

			report := scanGoBaseline(repo, "--worktree")

			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
		})
	})
})
