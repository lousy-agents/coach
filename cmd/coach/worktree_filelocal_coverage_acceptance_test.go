package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree qualifies what file-local analysis did not analyze", func() {
	notAnalyzedPaths := func(stdout []byte) []string {
		paths := []string{}
		for _, diagnostic := range diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed") {
			paths = append(paths, diagnostic.Path)
		}
		return paths
	}

	When("a tracked file is edited only on disk and is absent from the merge-base..HEAD diff", func() {
		const oneConstructor = "package model\n\ntype A struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n"
		diffRepo := func() (repo, base string) {
			repo = newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			base = commitFile(repo, "pkg/other/other.go", oneConstructor)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			Expect(os.WriteFile(filepath.Join(repo, "pkg/other/other.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())
			return repo, base
		}
		densityPaths := func(stdout []byte) []string {
			paths := []string{}
			for _, signal := range decodeCoachReport(stdout).Signals {
				if signal.RuleID == "structure.constructor_density" {
					paths = append(paths, signal.Path)
				}
			}
			return paths
		}

		It("is not analyzed by a committed diff scan (control)", func() {
			repo, base := diffRepo()

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(densityPaths(stdout)).To(BeEmpty())
		})

		It("is analyzed, because --worktree makes working-tree edits part of the change", func() {
			repo, base := diffRepo()

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--project-config", "project.json", "--format=json", "--worktree")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(densityPaths(stdout)).NotTo(BeEmpty(),
				"a signal that exists only in the edited bytes proves file-local analysis read the file from disk")
			Expect(densityPaths(stdout)).To(HaveEach("pkg/other/other.go"))
			Expect(notAnalyzedPaths(stdout)).NotTo(ContainElement("pkg/other/other.go"))
		})
	})

	When("a new file is staged but not committed", func() {
		stagedRepo := func() string {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			Expect(os.WriteFile(filepath.Join(repo, "pkg/handlers/staged.go"), []byte("package handlers\n"), 0o644)).To(Succeed())
			runGitInRepo(repo, "add", "pkg/handlers/staged.go")
			return repo
		}

		It("discloses it in a committed baseline scan (control)", func() {
			stdout, _, exitCode := runCoachCodesignalBaselineRaw(stagedRepo(), "--project-config", "project.json")

			Expect(exitCode).To(Equal(0))
			Expect(string(stdout)).To(ContainSubstring("staged"))
		})

		It("analyzes it as an added file, so nothing is disclosed as unanalyzed", func() {
			repo := stagedRepo()

			jsonOut, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json", "--worktree")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(notAnalyzedPaths(jsonOut)).NotTo(ContainElement("pkg/handlers/staged.go"))
		})
	})

	When("a tracked file is renamed with git mv and not committed", func() {
		It("analyzes the new path as an added file, while the old path is simply gone", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			runGitInRepo(repo, "mv", "pkg/handlers/handlers.go", "pkg/handlers/renamed.go")

			jsonOut, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json", "--worktree")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(notAnalyzedPaths(jsonOut)).NotTo(ContainElement("pkg/handlers/renamed.go"))
			Expect(notAnalyzedPaths(jsonOut)).NotTo(ContainElement("pkg/handlers/handlers.go"))
			Expect(decodeCoachReport(jsonOut).Summary.FilesAnalyzed).To(BeNumerically(">=", 1), "the renamed file is analyzed under its new name")
		})
	})
})
