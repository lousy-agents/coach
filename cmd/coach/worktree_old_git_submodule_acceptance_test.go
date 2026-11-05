//go:build unix

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree under a git that cannot be told not to fetch", func() {
	// gitBefore244 is a directory whose git reports 2.43.0 and ignores the instruction not to fetch, as a git before
	// 2.44 does, and is otherwise the git on PATH.
	gitBefore244 := func() string {
		realGit, err := exec.LookPath("git")
		Expect(err).NotTo(HaveOccurred())
		directory := GinkgoT().TempDir()
		script := fmt.Sprintf("#!/bin/sh\nfor last; do :; done\n[ \"$last\" = version ] && { echo 'git version 2.43.0'; exit 0; }\nunset GIT_NO_LAZY_FETCH\nexec %q \"$@\"\n", realGit)
		Expect(os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755)).To(Succeed())
		return directory
	}
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

	It("does not report a tree with an edit inside a submodule as clean, and names the submodule and the git", func() {
		repo := editedSubmodule()
		env := append(os.Environ(), "PATH="+gitBefore244()+string(os.PathListSeparator)+os.Getenv("PATH"))

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env,
			"codesignal", "--baseline", "--worktree", "--format=json", "--fail-on-incomplete-coverage")

		Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		diagnostics := diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_scan_incomplete")
		Expect(diagnostics).To(HaveLen(1))
		Expect(diagnostics[0].Message).To(And(ContainSubstring("git 2.43.0"), ContainSubstring("third/lib")))
	})

	It("names the edit inside the submodule by its own cause under a git that can be told not to fetch", func() {
		repo := editedSubmodule()

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil,
			"codesignal", "--baseline", "--worktree", "--format=json", "--fail-on-incomplete-coverage")

		Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		diagnostics := diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_scan_incomplete")
		Expect(diagnostics).To(HaveLen(1))
		Expect(diagnostics[0].Message).To(ContainSubstring("a submodule has working-tree changes"))
		Expect(diagnostics[0].Message).NotTo(ContainSubstring("cannot be told not to fetch"))
	})
})
