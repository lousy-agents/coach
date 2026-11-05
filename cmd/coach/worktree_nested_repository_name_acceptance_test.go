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
	It("still analyzes the rest of the tree and names the nested repository as the cause of the incomplete verdict", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/a.go", nestedIfs("A", 6))
		nested := filepath.Join(repo, "pkg/go.mod")
		Expect(os.MkdirAll(nested, 0o755)).To(Succeed())
		runGitInRepo(nested, "init", "-q")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json", "--fail-on-incomplete-coverage")

		report := decodeCoachReport(stdout)
		Expect(report.Signals).NotTo(BeEmpty(), "pkg/a.go is committed and unchanged, so its signals must still be reported; stderr: %s", stderr)
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_status_check_failed")).To(BeEmpty(),
			"one directory entry must not fail the overlay for every other path")
		Expect(diagnosticsOfKind(report.Diagnostics, "head_read_failed")).To(BeEmpty())
		incomplete := diagnosticsOfKind(report.Diagnostics, "worktree_scan_incomplete")
		Expect(incomplete).To(HaveLen(1))
		Expect(incomplete[0].Message).To(ContainSubstring("nested repository"))
		Expect(exitCode).To(Equal(3), "the nested repository is unread, so the verdict stays incomplete")
	})
})
