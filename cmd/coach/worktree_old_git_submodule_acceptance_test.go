//go:build unix

package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree under a git that cannot be told not to fetch", func() {
	// editedSubmodule commits a submodule and leaves an edit to a file in it.
	editedSubmodule := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		library := newTempGitRepo()
		commitFile(library, "lib.go", "package lib\n")
		runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
		runGitInRepo(repo, "commit", "-q", "-m", "add the submodule")
		Expect(os.WriteFile(filepath.Join(repo, "third/lib/lib.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())
		return repo
	}

	It("shows the fixture is live: a plain git status sees the edit inside the submodule (control)", func() {
		Expect(gitPorcelain(editedSubmodule())).To(ContainSubstring("third/lib"))
	})

	It("discloses an edit inside a submodule as one git_version entry that hides no in-scope source, never also as submodule (#168 item 22)", func() {
		run := runWorktreeScan(editedSubmodule(), envWithGitBefore244())

		entry := run.only("git_version")
		Expect(entry).To(HaveKeyWithValue("root", "third/lib"))
		Expect(entry).To(HaveKeyWithValue("hides_in_scope", "no"))
		Expect(entry["message"]).To(ContainSubstring("git 2.43.0"))
		Expect(pathOf(entry)).To(BeEmpty())
		Expect(run.ofReason("submodule")).To(BeEmpty())
		Expect(run.exitCode).To(Equal(0))
	})

	It("discloses the edit inside the submodule by its own cause, hiding no in-scope source, under a git that can be told not to fetch (#168 item 22)", func() {
		if !installedGitCanBeToldNotToFetch() {
			Skip("a real submodule entry needs git 2.44 or later; the installed git is older, so the git_version specs cover its submodules")
		}

		run := runWorktreeScan(editedSubmodule(), nil)

		Expect(run.only("submodule")).To(And(HaveKeyWithValue("root", "third/lib"), HaveKeyWithValue("hides_in_scope", "no")))
		Expect(run.ofReason("git_version")).To(BeEmpty())
		Expect(run.exitCode).To(Equal(0))
	})
})
