package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree does not follow a symbolic link in a parent directory", func() {
	// replaceDirectoryWithSymlink swaps a tracked directory for a symlink to an
	// outside directory that holds a file of the same name.
	replaceDirectoryWithSymlink := func(repo, directory, fileName, outsideContents string) {
		outside := GinkgoT().TempDir()
		ExpectWithOffset(1, os.WriteFile(filepath.Join(outside, fileName), []byte(outsideContents), 0o644)).To(Succeed())
		ExpectWithOffset(1, os.RemoveAll(filepath.Join(repo, directory))).To(Succeed())
		ExpectWithOffset(1, os.Symlink(outside, filepath.Join(repo, directory))).To(Succeed())
	}

	It("does not read an outside file as the repository file, so a planted violation is not reported", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		replaceDirectoryWithSymlink(repo, "pkg/handlers", "handlers.go", handlersImportingDB)

		report := scanGoBaseline(repo, "--worktree")

		Expect(layerViolationCount(report)).To(Equal(0),
			"a violation here came from a file outside the repository, reached through a symlinked parent directory")
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_symlink_skipped")).NotTo(BeEmpty(),
			"a symlinked ancestor is a skipped symbolic link")
	})

	It("qualifies the verdict instead of reading a clean outside file in place of a violating repository file", func() {
		repo := goOverlayRepo()
		replaceDirectoryWithSymlink(repo, "pkg/handlers", "handlers.go", handlersWithoutImport)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("the analysis is incomplete"))
		Expect(string(stdout)).NotTo(MatchRegexp(`(?m)^No active CodeSignal findings\.$`),
			"an unqualified all-clear here would come from a file outside the repository")
	})
})
