package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree reports a dirty submodule whatever the repository configures", func() {
	// ignoringDirtySubmodule commits a submodule whose .gitmodules tells git
	// status to ignore working-tree changes inside it, then edits a file in it.
	ignoringDirtySubmodule := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		library := newTempGitRepo()
		commitFile(library, "lib.go", "package lib\n")
		runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
		runGitInRepo(repo, "config", "-f", ".gitmodules", "submodule.third/lib.ignore", "dirty")
		runGitInRepo(repo, "add", ".gitmodules")
		runGitInRepo(repo, "commit", "-q", "-m", "add the submodule, ignoring its dirty state")
		Expect(os.WriteFile(filepath.Join(repo, "third/lib/lib.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())
		return repo
	}

	It("shows the fixture is live: plain git status hides the edit inside the submodule (control)", func() {
		repo := ignoringDirtySubmodule()

		Expect(gitPorcelain(repo)).NotTo(ContainSubstring("third/lib"))
	})

	It("qualifies the verdict, names the submodule and exits 3 under --fail-on-incomplete-coverage", func() {
		repo := ignoringDirtySubmodule()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json", "--fail-on-incomplete-coverage")

		Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		diagnostics := diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_scan_incomplete")
		Expect(diagnostics).To(HaveLen(1), "a submodule edit that the repository's own ignore setting hides must still be reported")
		Expect(diagnostics[0].Message).To(ContainSubstring("submodule"))
	})
})
