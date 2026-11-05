package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A path git stages and the user then deletes from disk (RD, AD, CD) is in
// neither HEAD nor the working tree, so the scan has nothing to read for it.
var _ = Describe("coach codesignal --worktree treats a staged path deleted from disk as absent", func() {
	const (
		movedFrom = "pkg/model/m.go"
		movedTo   = "pkg/model/n.go"
	)
	scan := func(repo string, extraArgs ...string) (*codesignal.Report, int, []byte) {
		args := append([]string{"--worktree", "--format=json", "--fail-on-incomplete-coverage"}, extraArgs...)
		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", args...)
		ExpectWithOffset(1, exitCode).To(BeElementOf(0, 3), "stderr: %s", stderr)
		return decodeCoachReport(stdout), exitCode, stdout
	}
	baseline := func(repo string) *codesignal.Report {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json", "--fail-on-incomplete-coverage")
		ExpectWithOffset(1, exitCode).To(BeElementOf(0, 3), "stderr: %s", stderr)
		return decodeCoachReport(stdout)
	}
	expectNothingSaidAbout := func(report *codesignal.Report, raw []byte, name string) {
		for _, diagnostic := range report.Diagnostics {
			ExpectWithOffset(1, diagnostic.Path).NotTo(Equal(name), "%s: %s", diagnostic.Kind, diagnostic.Message)
		}
		ExpectWithOffset(1, diagnosticsOfKind(report.Diagnostics, "head_read_failed")).To(BeEmpty())
		ExpectWithOffset(1, string(raw)).NotTo(ContainSubstring("fatal:"), "raw git stderr must not reach the report")
	}
	committedModel := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, movedFrom, modelFileWithTwoConstructors)
		commitFile(repo, "pkg/model/seed.go", "package model\n")
		return repo
	}
	deleteFromDisk := func(repo, name string) {
		ExpectWithOffset(1, os.Remove(filepath.Join(repo, name))).To(Succeed())
	}
	lifecycles := func(report *codesignal.Report, path string) []codesignal.Lifecycle {
		var found []codesignal.Lifecycle
		for _, signal := range report.Signals {
			if signal.Path == path {
				found = append(found, signal.Lifecycle)
			}
		}
		return found
	}

	When("a staged rename's new path is deleted from disk (RD)", func() {
		renamedThenDeleted := func() string {
			repo := committedModel()
			runGitInRepo(repo, "mv", movedFrom, movedTo)
			deleteFromDisk(repo, movedTo)
			expectDirtyPorcelain(repo, "RD "+movedFrom+" -> "+movedTo)
			return repo
		}

		It("reads nothing for the vanished new path and does not qualify the verdict, so the exit stays 0", func() {
			report, exitCode, raw := scan(renamedThenDeleted())

			Expect(exitCode).To(Equal(0))
			expectNothingSaidAbout(report, raw, movedTo)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		})

		It("counts the moved-away file as the one deleted path, because the new path never reached HEAD", func() {
			report, _, _ := scan(renamedThenDeleted())

			Expect(report.Scope.Worktree).To(HaveField("Deleted", 1))
			Expect(report.Scope.Worktree).To(HaveField("Added", 0))
		})

		It("reports the old path's signals as resolved, because nothing carries them forward", func() {
			report, _, _ := scan(renamedThenDeleted())

			Expect(lifecycles(report, movedFrom)).NotTo(BeEmpty())
			Expect(lifecycles(report, movedFrom)).To(HaveEach(codesignal.Lifecycle("resolved")))
		})

		It("drops the old path from the baseline's discovered files and counts nothing as unanalyzable", func() {
			control := baseline(committedModel())

			coverage := baseline(renamedThenDeleted()).Coverage

			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.Coverage.TrackedFilesDiscovered - 1))
			Expect(coverage.FilesUnanalyzable).To(BeZero())
		})
	})

	When("a file is staged as new and then deleted from disk (AD)", func() {
		stagedThenDeleted := func() string {
			repo := committedModel()
			stageWorktreeFile(repo, movedTo, modelFileWithTwoConstructors)
			deleteFromDisk(repo, movedTo)
			expectDirtyPorcelain(repo, "AD "+movedTo)
			return repo
		}

		It("reads nothing for it and does not qualify the verdict, so the exit stays 0", func() {
			report, exitCode, raw := scan(stagedThenDeleted())

			Expect(exitCode).To(Equal(0))
			expectNothingSaidAbout(report, raw, movedTo)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		})

		It("counts it nowhere, because neither HEAD nor the working tree holds it", func() {
			report, _, _ := scan(stagedThenDeleted())

			Expect(report.Scope.Worktree).To(HaveField("Deleted", 0))
			Expect(report.Scope.Worktree).To(HaveField("Added", 0))
		})

		It("does not name a vanished file of an unsupported language either", func() {
			repo := committedModel()
			stageWorktreeFile(repo, "docs/notes.md", "# notes\n")
			deleteFromDisk(repo, "docs/notes.md")
			expectDirtyPorcelain(repo, "AD docs/notes.md")

			report, exitCode, _ := scan(repo)

			Expect(exitCode).To(Equal(0))
			Expect(diagnosticsOfKind(report.Diagnostics, "unsupported_language")).To(BeEmpty())
		})

		It("leaves the baseline's discovered files as they are committed", func() {
			control := baseline(committedModel())

			coverage := baseline(stagedThenDeleted()).Coverage

			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.Coverage.TrackedFilesDiscovered))
			Expect(coverage.FilesUnanalyzable).To(BeZero())
		})
	})

	When("a staged copy's new path is deleted from disk (CD)", func() {
		copiedThenDeleted := func() string {
			repo := committedModel()
			runGitInRepo(repo, "config", "status.renames", "copies")
			edited := modelFileWithTwoConstructors + "\n// edited\n"
			stageWorktreeFile(repo, movedFrom, edited)
			stageWorktreeFile(repo, movedTo, edited)
			deleteFromDisk(repo, movedTo)
			expectDirtyPorcelain(repo, "CD "+movedFrom+" -> "+movedTo)
			return repo
		}

		It("reads nothing for the vanished copy and does not qualify the verdict, so the exit stays 0", func() {
			report, exitCode, raw := scan(copiedThenDeleted())

			Expect(exitCode).To(Equal(0))
			expectNothingSaidAbout(report, raw, movedTo)
		})

		It("counts only the edited source, which stays on disk", func() {
			report, _, _ := scan(copiedThenDeleted())

			Expect(report.Scope.Worktree).To(HaveField("Modified", 1))
			Expect(report.Scope.Worktree).To(HaveField("Deleted", 0))
			Expect(report.Scope.Worktree).To(HaveField("Added", 0))
		})

		It("keeps the copy source in the baseline's discovered files", func() {
			control := baseline(committedModel())

			coverage := baseline(copiedThenDeleted()).Coverage

			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.Coverage.TrackedFilesDiscovered))
			Expect(coverage.FilesUnanalyzable).To(BeZero())
		})
	})
})
