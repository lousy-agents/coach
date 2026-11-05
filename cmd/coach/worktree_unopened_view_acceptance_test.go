package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree reports what the overlay could not read when no analysis opened the working-tree view", func() {
	replaceGoModWithSymlink := func(repo string) {
		target := filepath.Join(GinkgoT().TempDir(), "go.mod")
		Expect(os.WriteFile(target, []byte(goModuleFile), 0o644)).To(Succeed())
		Expect(os.Remove(filepath.Join(repo, "go.mod"))).To(Succeed())
		Expect(os.Symlink(target, filepath.Join(repo, "go.mod"))).To(Succeed())
	}
	noSourceRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "notes.txt", "no source file reaches file-local analysis\n")
		return repo
	}

	It("names a go.mod that is a symbolic link, in a project with no source file and no policy", func() {
		repo := noSourceRepo()
		replaceGoModWithSymlink(repo)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		skipped := diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_symlink_skipped")
		Expect(skipped).To(HaveLen(1))
		Expect(skipped[0].Path).To(Equal("go.mod"))
	})

	It("reports nothing for a go.mod that is a regular file (control)", func() {
		repo := noSourceRepo()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_symlink_skipped")).To(BeEmpty())
	})
})
