package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const handlerWithoutDBImport = handlersWithoutImport

func goOverlayRepo() string {
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/db/db.go", dbPackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	return repo
}

func overwriteOnDisk(repo, name, contents string) {
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, name), []byte(contents), 0o644)).To(Succeed())
}

func layerViolationCount(report *codesignal.Report) int {
	count := 0
	for _, change := range report.ProjectChanges {
		if change.RuleID == "architecture.layer_violation" {
			count++
		}
	}
	return count
}

func unresolvedEdgeCount(report *codesignal.Report) int {
	ExpectWithOffset(1, report.ProjectCoverage).NotTo(BeNil())
	return report.ProjectCoverage.Counts["unresolved_edges"]
}

func scanGoBaseline(repo string, extraArgs ...string) *codesignal.Report {
	args := append([]string{"--project-config", "project.json", "--format=json"}, extraArgs...)
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
	return decodeCoachReport(stdout)
}

var _ = Describe("coach codesignal --worktree for Go project analysis", func() {
	When("go.mod on disk differs from HEAD (acceptance item 16)", func() {
		const renamedModule = "module example.com/renamed\n\ngo 1.25\n"

		It("ignores the edit without the flag, so the committed import still resolves (control)", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "go.mod", renamedModule)

			report := scanGoBaseline(repo)

			Expect(layerViolationCount(report)).To(Equal(1))
			Expect(unresolvedEdgeCount(report)).To(Equal(0))
		})

		It("reads the module path from disk, so the import is unresolved and the layer violation is gone", func() {
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "go.mod", renamedModule)

			report := scanGoBaseline(repo, "--worktree")

			Expect(unresolvedEdgeCount(report)).To(Equal(1),
				"the import of example.com/app/pkg/db must be unresolved against the edited module path")
			Expect(layerViolationCount(report)).To(Equal(0))
		})
	})

	When("a tracked file that another file imports is deleted on disk (acceptance item 17)", func() {
		It("still sees the package without the flag (control)", func() {
			repo := goOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "pkg/db/db.go"))).To(Succeed())

			report := scanGoBaseline(repo)

			Expect(layerViolationCount(report)).To(Equal(1))
			Expect(unresolvedEdgeCount(report)).To(Equal(0))
		})

		It("treats the import as unresolved, not as an import of an empty package", func() {
			repo := goOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "pkg/db/db.go"))).To(Succeed())

			report := scanGoBaseline(repo, "--worktree")

			Expect(unresolvedEdgeCount(report)).To(Equal(1),
				"a deleted file must be absent; serving it as empty would resolve the import and hide the gap")
			Expect(layerViolationCount(report)).To(Equal(0))
		})
	})

	When("a tracked supported path is a symbolic link on disk (acceptance item 20)", func() {
		replaceWithSymlinkTo := func(repo, name, targetContents string) {
			target := filepath.Join(GinkgoT().TempDir(), "target.go")
			ExpectWithOffset(1, os.WriteFile(target, []byte(targetContents), 0o644)).To(Succeed())
			ExpectWithOffset(1, os.Remove(filepath.Join(repo, name))).To(Succeed())
			ExpectWithOffset(1, os.Symlink(target, filepath.Join(repo, name))).To(Succeed())
		}

		It("does not serve the committed bytes for the path, and names it in worktree_symlink_skipped", func() {
			repo := goOverlayRepo()
			replaceWithSymlinkTo(repo, "pkg/handlers/handlers.go", handlerWithoutDBImport)

			report := scanGoBaseline(repo, "--worktree")

			Expect(layerViolationCount(report)).To(Equal(0),
				"the committed handlers.go imports db; a violation means the stale committed bytes were served for a symlink")
			skipped := diagnosticsOfKind(report.Diagnostics, "worktree_symlink_skipped")
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0].Path).To(Equal("pkg/handlers/handlers.go"))
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).NotTo(ContainElement(HaveField("Path", "pkg/handlers/handlers.go")),
				"the symlink diagnostic replaces the generic one for the same path")
		})

		It("does not follow the link, so a planted import in the target has no effect", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlerWithoutDBImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			replaceWithSymlinkTo(repo, "pkg/handlers/handlers.go", handlersImportingDB)

			report := scanGoBaseline(repo, "--worktree")

			Expect(layerViolationCount(report)).To(Equal(0),
				"a violation means the symlink target's planted import was read")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_symlink_skipped")).To(HaveLen(1))
		})
	})

	When("an overlay read is incomplete (PRD section 8.3)", func() {
		It("qualifies the text verdict instead of printing an unqualified all-clear when a path is a symbolic link", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlerWithoutDBImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			target := filepath.Join(GinkgoT().TempDir(), "target.go")
			Expect(os.WriteFile(target, []byte(handlersImportingDB), 0o644)).To(Succeed())
			Expect(os.Remove(filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())
			Expect(os.Symlink(target, filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring("the analysis is incomplete: 1 path was not analyzed"),
				"the skipped path must be counted, not merely noted in a generic diagnostics line")
			Expect(string(stdout)).NotTo(MatchRegexp(`(?m)^No active CodeSignal findings\.$`),
				"an unqualified all-clear would tell the reader a skipped path was checked")
		})

		It("treats a changed path it cannot read as absent, never as its committed bytes", func() {
			if os.Geteuid() == 0 {
				Skip("file permissions do not stop root from reading")
			}
			repo := goOverlayRepo()
			unreadable := filepath.Join(repo, "pkg/handlers/handlers.go")
			Expect(os.WriteFile(unreadable, []byte(handlersImportingDB+"\n// edited\n"), 0o644)).To(Succeed())
			Expect(os.Chmod(unreadable, 0o000)).To(Succeed())
			DeferCleanup(os.Chmod, unreadable, os.FileMode(0o644))

			report := scanGoBaseline(repo, "--worktree")

			Expect(layerViolationCount(report)).To(Equal(0),
				"the committed handlers.go imports db; a violation means its stale bytes were served for a file the overlay could not read")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(
				ContainElement(HaveField("Path", "pkg/handlers/handlers.go")),
				"the unread path must be named in a diagnostic that carries it, so the verdict is qualified")
		})
	})

	When("go.work on disk differs from HEAD", func() {
		goWorkRepo := func() string {
			repo := newTempGitRepo()
			commitFile(repo, "go.work", "go 1.25\n\nuse (\n\t./moda\n\t./modb\n)\n")
			commitFile(repo, "moda/go.mod", "module example.com/moda\n\ngo 1.25\n")
			commitFile(repo, "modb/go.mod", "module example.com/modb\n\ngo 1.25\n")
			commitFile(repo, "modb/db/db.go", dbPackageFile)
			commitFile(repo, "moda/handlers/handlers.go", "package handlers\n\nimport \"example.com/modb/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["moda/handlers"]},{"name":"db","prefixes":["modb/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`)
			overwriteOnDisk(repo, "go.work", "go 1.25\n\nuse (\n\t./moda\n\t./modb\n\t./modb\n)\n")
			return repo
		}
		coverageDiagnosticCodes := func(report *codesignal.Report) []string {
			Expect(report.ProjectCoverage).NotTo(BeNil())
			codes := []string{}
			for _, diagnostic := range report.ProjectCoverage.Diagnostics {
				codes = append(codes, diagnostic.Code)
			}
			return codes
		}

		It("ignores the edit without the flag (control)", func() {
			Expect(coverageDiagnosticCodes(scanGoBaseline(goWorkRepo()))).NotTo(ContainElement("project_root_duplicate"))
		})

		It("reads the edited go.work from disk, so its duplicate use directive is reported", func() {
			Expect(coverageDiagnosticCodes(scanGoBaseline(goWorkRepo(), "--worktree"))).To(ContainElement("project_root_duplicate"))
		})
	})

	When("tracked and untracked files are dirty (acceptance item 21)", func() {
		cleanBeforeEdit := func() string {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			return repo
		}
		kinds := func(report *codesignal.Report) []string {
			found := []string{}
			for _, diagnostic := range report.Diagnostics {
				found = append(found, diagnostic.Kind)
			}
			return found
		}

		It("discloses a modified file the committed scan did not read (control)", func() {
			repo := cleanBeforeEdit()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersImportingDB)

			report := scanGoBaseline(repo)

			Expect(kinds(report)).To(ContainElements("worktree_changes_not_analyzed", "worktree_report_reflects_committed_head"))
		})

		It("reads the modified file, so it neither discloses it nor claims the report reflects the committed HEAD", func() {
			repo := cleanBeforeEdit()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersImportingDB)

			report := scanGoBaseline(repo, "--worktree")

			Expect(layerViolationCount(report)).To(Equal(1), "the edit adds a forbidden import, which a worktree scan must see")
			Expect(kinds(report)).NotTo(ContainElement("worktree_changes_not_analyzed"))
			Expect(kinds(report)).NotTo(ContainElement("worktree_report_reflects_committed_head"))
		})

		It("analyzes an untracked supported file, so it is not disclosed as unanalyzed", func() {
			repo := cleanBeforeEdit()
			overwriteOnDisk(repo, "pkg/handlers/extra.go", handlersWithoutImport)

			report := scanGoBaseline(repo, "--worktree")

			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty(),
				"file-local analysis reads new files as added files, so there is nothing to disclose")
		})
	})
})
