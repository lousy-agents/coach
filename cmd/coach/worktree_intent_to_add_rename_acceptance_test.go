package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// `git add -N` on the destination of a moved file makes git report the move in
// the worktree column (" R old -> new"), not the index column a staged rename
// uses. It is still a rename: PRD 8.1 counts it as one deleted path and one
// added path.
var _ = Describe("coach codesignal --worktree treats an intent-to-add rename as a rename", func() {
	const (
		movedFrom = "pkg/model/m.go"
		movedTo   = "pkg/model/n.go"
	)
	movedWithIntentToAdd := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, movedFrom, modelFileWithTwoConstructors)
		commitFile(repo, "pkg/model/seed.go", "package model\n")
		ExpectWithOffset(1, os.Rename(filepath.Join(repo, movedFrom), filepath.Join(repo, movedTo))).To(Succeed())
		runGitInRepo(repo, "add", "--intent-to-add", movedTo)
		expectDirtyPorcelain(repo, " R "+movedFrom+" -> "+movedTo)
		return repo
	}
	subjectsAt := func(report *codesignal.Report, path string) []string {
		var subjects []string
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == path {
				subjects = append(subjects, signal.Subject)
			}
		}
		return subjects
	}

	When("the scan compares the working tree with HEAD", func() {
		scan := func(repo string) *codesignal.Report {
			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--worktree", "--format=json")
			ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
			return decodeCoachReport(stdout)
		}

		It("analyzes the new path, which holds the moved file's bytes on disk", func() {
			report := scan(movedWithIntentToAdd())

			Expect(subjectsAt(report, movedTo)).To(ConsistOf("NewA", "NewB"))
			Expect(subjectsAt(report, movedFrom)).To(BeEmpty(), "the old path is gone from disk")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		})

		It("gives the moved file's signals the undetermined continuity a staged rename gets", func() {
			report := scan(movedWithIntentToAdd())

			Expect(diagnosticsOfKind(report.Diagnostics, "continuity_not_determined")).To(ContainElement(HaveField("Path", movedTo)))
			for _, signal := range report.Signals {
				if signal.Path == movedTo {
					Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
				}
			}
		})

		It("counts one deleted path and one added path", func() {
			counts := scan(movedWithIntentToAdd()).Scope.Worktree

			Expect(counts).To(HaveField("Added", 1))
			Expect(counts).To(HaveField("Deleted", 1))
			Expect(counts).To(HaveField("Modified", 0))
		})
	})

	When("the scan is a baseline", func() {
		baseline := func(repo string) *codesignal.Report {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json", "--fail-on-incomplete-coverage")
			ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
			return decodeCoachReport(stdout)
		}

		It("analyzes the new path and leaves the old path out", func() {
			report := baseline(movedWithIntentToAdd())

			Expect(subjectsAt(report, movedTo)).To(ConsistOf("NewA", "NewB"))
			Expect(subjectsAt(report, movedFrom)).To(BeEmpty())
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		})

		It("discovers the same number of files as before the move, because one path left and one arrived", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, movedFrom, modelFileWithTwoConstructors)
			commitFile(repo, "pkg/model/seed.go", "package model\n")
			control := baseline(repo)

			coverage := baseline(movedWithIntentToAdd()).Coverage

			Expect(coverage.TrackedFilesDiscovered).To(Equal(control.Coverage.TrackedFilesDiscovered))
			Expect(coverage.FilesAnalyzed).To(Equal(control.Coverage.FilesAnalyzed))
		})
	})
})
