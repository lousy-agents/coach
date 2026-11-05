package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A new file (untracked, staged, or the new path of a rename) is analyzed by the
// file-local rules as an added file, as PRD 8.1 requires.
var _ = Describe("coach codesignal --worktree analyzes new files with the file-local rules (acceptance item 19)", func() {
	densitySubjects := func(report *codesignal.Report, path string) map[string]bool {
		subjects := map[string]bool{}
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == path {
				subjects[signal.Subject] = signal.Changed
			}
		}
		return subjects
	}
	newFileRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/seed.go", "package model\n")
		return repo
	}
	writeNew := func(repo, name string) {
		ExpectWithOffset(1, os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755)).To(Succeed())
		ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, name), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())
	}

	It("analyzes an untracked file in a baseline scan, and does not say it was not analyzed", func() {
		repo := newFileRepo()
		writeNew(repo, "pkg/model/fresh.go")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		report := decodeCoachReport(stdout)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(densitySubjects(report, "pkg/model/fresh.go")).To(HaveKey("NewA"),
			"a signal that exists only in the new file proves file-local analysis read it")
		for _, diagnostic := range diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed") {
			Expect(diagnostic.Path).NotTo(Equal("pkg/model/fresh.go"))
		}
	})

	It("classifies the signals of an untracked file as baseline in a baseline scan", func() {
		repo := newFileRepo()
		writeNew(repo, "pkg/model/fresh.go")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		report := decodeCoachReport(stdout)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		lifecycles := lifecyclesAt(report, "pkg/model/fresh.go")
		Expect(lifecycles).NotTo(BeEmpty())
		Expect(lifecycles).To(HaveEach(codesignal.Lifecycle("baseline")),
			"a baseline scan compares nothing, so it assigns no lifecycle other than baseline, whatever the file's origin")
	})

	It("analyzes a staged new file the same way", func() {
		repo := newFileRepo()
		writeNew(repo, "pkg/model/staged.go")
		runGitInRepo(repo, "add", "pkg/model/staged.go")

		stdout, _, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0))
		Expect(densitySubjects(decodeCoachReport(stdout), "pkg/model/staged.go")).To(HaveKey("NewA"))
	})

	It("analyzes the new path of a git mv as an added file", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/old.go", modelFileWithTwoConstructors)
		runGitInRepo(repo, "mv", "pkg/model/old.go", "pkg/model/renamed.go")

		stdout, _, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		report := decodeCoachReport(stdout)

		Expect(exitCode).To(Equal(0))
		Expect(densitySubjects(report, "pkg/model/renamed.go")).To(HaveKey("NewA"))
		Expect(densitySubjects(report, "pkg/model/old.go")).To(BeEmpty(), "the old path is gone")
	})

	It("marks every signal of an untracked file changed in a diff scan, and leaves an unchanged committed constructor unmarked", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/stable.go", modelFileWithTwoConstructors)
		base := commitFile(repo, "pkg/model/seed.go", "package model\n")
		commitFile(repo, "pkg/model/stable.go", modelFileWithTwoConstructors+"\n// committed edit below the constructors\n")
		writeNew(repo, "pkg/model/fresh.go")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")
		report := decodeCoachReport(stdout)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(densitySubjects(report, "pkg/model/fresh.go")).To(Equal(map[string]bool{"NewA": true, "NewB": true}))
		Expect(densitySubjects(report, "pkg/model/stable.go")).To(Equal(map[string]bool{"NewA": false, "NewB": false}),
			"the committed file is in the diff but its constructors did not change; marking every file changed would hide that")
	})
	It("reports a layer violation in an untracked file as introduced in a diff scan", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		base := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// committed edit\n")
		overwriteOnDisk(repo, "pkg/handlers/extra.go", handlersImportingDB)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--project-config", "project.json", "--worktree", "--format=json")
		report := decodeCoachReport(stdout)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(layerViolationCount(report)).To(Equal(1))
		for _, change := range report.ProjectChanges {
			if change.RuleID == "architecture.layer_violation" {
				Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")),
					"the file exists only in the working tree, so the violation is new relative to the base")
			}
		}
	})
	It("analyzes a file added with git add --intent-to-add as a new file, and counts it as added", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/seed.go", "package model\n")
		writeNew(repo, "pkg/model/n.go")
		runGitInRepo(repo, "add", "--intent-to-add", "pkg/model/n.go")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		report := decodeCoachReport(stdout)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(densitySubjects(report, "pkg/model/n.go")).To(HaveKey("NewA"),
			"a signal that exists only in the new file proves file-local analysis read it")
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		Expect(report.Scope.Worktree.Added).To(Equal(1))
		Expect(report.Scope.Worktree.Modified).To(BeZero())
	})
})
