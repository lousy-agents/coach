package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree for a nested repository named like a project file", func() {
	// git status lists an untracked nested repository as "pkg/go.mod/", with a
	// trailing slash, and its name makes it look like a project configuration file.
	It("still analyzes the rest of the tree and discloses the nested repository by its reason (#168 item 22)", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/a.go", nestedIfs("A", 6))
		nested := filepath.Join(repo, "pkg/go.mod")
		Expect(os.MkdirAll(nested, 0o755)).To(Succeed())
		runGitInRepo(nested, "init", "-q")

		stdout, stderr, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json", "--fail-on-incomplete-coverage")

		report := decodeCoachReport(stdout)
		Expect(report.Signals).NotTo(BeEmpty(), "pkg/a.go is committed and unchanged, so its signals must still be reported; stderr: %s", stderr)
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_status_check_failed")).To(BeEmpty(),
			"one directory entry must not fail the overlay for every other path")
		Expect(diagnosticsOfKind(report.Diagnostics, "head_read_failed")).To(BeEmpty())
		run := runWorktreeScan(repo, nil)
		entry := run.only("nested_repository")
		Expect(entry).To(HaveKeyWithValue("root", "pkg/go.mod"))
		Expect(entry).To(HaveKeyWithValue("hides_in_scope", "no"), "the nested path is judged by the scope filter alone, which never keeps a directory named go.mod")
		Expect(run.exitCode).To(Equal(0))
	})

	It("judges a nested directory named like the policy file by the scope filter alone too (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addNestedRepository(repo, "pkg/project.json")

		run := runProjectScan(repo)

		Expect(run.only("nested_repository")).To(And(HaveKeyWithValue("root", "pkg/project.json"), HaveKeyWithValue("hides_in_scope", "no")))
		Expect(run.exitCode).To(Equal(0))
	})
})
