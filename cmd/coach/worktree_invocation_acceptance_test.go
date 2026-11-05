package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree invocation", func() {
	const oneConstructor = "package model\n\ntype A struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n"

	editedRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/other/other.go", oneConstructor)
		overwriteOnDisk(repo, "pkg/other/other.go", modelFileWithTwoConstructors)
		return repo
	}

	When("--worktree is the only mode flag", func() {
		It("compares the working tree with HEAD", func() {
			repo := editedRepo()

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report := decodeCoachReport(stdout)
			Expect(report.Scope.Base).To(Equal(runGitInRepo(repo, "rev-parse", "HEAD")))
			Expect(report.Scope.Worktree).NotTo(BeNil())
			signalPaths := []string{}
			for _, signal := range report.Signals {
				signalPaths = append(signalPaths, signal.Path)
			}
			Expect(signalPaths).To(ContainElement("pkg/other/other.go"),
				"the edit exists only in the working tree, so a signal at that path proves the comparison read it")
		})

		It("still requires a mode when --worktree is absent (control)", func() {
			_, stderr, exitCode := runCoachBinary(commandPath, editedRepo(), nil, "codesignal", "--format=json")

			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).To(ContainSubstring("missing required --base flag"))
		})
	})

	When("--worktree runs from a directory other than the repository root", func() {
		subdirectoryRepo := func() string {
			repo := editedRepo()
			runGitInRepo(repo, "add", ".")
			runGitInRepo(repo, "commit", "-q", "-m", "second commit")
			return repo
		}

		It("refuses with an error that names the repository root and its path, without a usage line that says a base is required", func() {
			repo := subdirectoryRepo()
			root, err := filepath.EvalSymlinks(repo)
			Expect(err).NotTo(HaveOccurred())

			stdout, stderr, exitCode := runCoachBinary(commandPath, filepath.Join(repo, "pkg"), nil,
				"codesignal", "--base", "HEAD~1", "--worktree", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("coach: --worktree must run from the repository root; run: cd '" + root + "' &&"))
			Expect(string(stderr)).NotTo(ContainSubstring("usage:"))
			Expect(stdout).To(BeEmpty())
		})

		It("ends with the command to run from the root, with the flags it was given", func() {
			repo := subdirectoryRepo()
			root, err := filepath.EvalSymlinks(repo)
			Expect(err).NotTo(HaveOccurred())

			_, stderr, exitCode := runCoachBinary(commandPath, filepath.Join(repo, "pkg"), nil,
				"codesignal", "--base", "HEAD~1", "--worktree", "--format=json", "--project-config", "policy file.json")

			Expect(exitCode).To(Equal(2))
			Expect(strings.TrimSpace(string(stderr))).To(Equal(
				"coach: --worktree must run from the repository root; run: cd '" + root + "' && coach codesignal --base HEAD~1 --worktree --format=json --project-config 'policy file.json'"))
		})

		It("quotes a root whose name holds a space and a single quote, so the command can be pasted", func() {
			parent, err := os.MkdirTemp("", "coach quote *")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, parent)
			repo := filepath.Join(parent, "it's a repo")
			Expect(os.Mkdir(repo, 0o755)).To(Succeed())
			runGitInRepo(repo, "init", "-q")
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/a.go", "package pkg\n")
			root, err := filepath.EvalSymlinks(repo)
			Expect(err).NotTo(HaveOccurred())

			_, stderr, exitCode := runCoachBinary(commandPath, filepath.Join(repo, "pkg"), nil, "codesignal", "--worktree")

			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).To(ContainSubstring("run: cd '" + strings.ReplaceAll(root, "'", `'\''`) + "' && coach codesignal --worktree"))
		})

		It("accepts the root spelled with different letter case on a case-insensitive filesystem", func() {
			repo := subdirectoryRepo()
			variant := filepath.Join(filepath.Dir(repo), strings.ToUpper(filepath.Base(repo)))
			variantInfo, statErr := os.Stat(variant)
			if statErr != nil {
				Skip("this filesystem tells names apart by letter case, so there is no second spelling of the root")
			}
			repoInfo, err := os.Stat(repo)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.SameFile(variantInfo, repoInfo)).To(BeTrue())

			_, stderr, exitCode := runCoachBinary(commandPath, variant, nil,
				"codesignal", "--base", "HEAD~1", "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		})

		It("scans from the repository root (control)", func() {
			repo := subdirectoryRepo()

			_, stderr, exitCode := runCoachBinary(commandPath, repo, nil,
				"codesignal", "--base", "HEAD~1", "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		})

		It("does not apply the restriction without the flag (control)", func() {
			repo := subdirectoryRepo()
			Expect(os.MkdirAll(filepath.Join(repo, "pkg"), 0o755)).To(Succeed())

			_, stderr, _ := runCoachBinary(commandPath, filepath.Join(repo, "pkg"), nil,
				"codesignal", "--base", "HEAD~1", "--format=json")

			Expect(string(stderr)).NotTo(ContainSubstring("repository root"))
		})
	})

	When("a flag is misused and the flag package prints its usage", func() {
		It("does not list the experimental flag", func() {
			_, stderr, exitCode := runCoachRaw("codesignal", "--worktree", "--base")

			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).To(ContainSubstring("flag needs an argument: -base"))
			Expect(string(stderr)).To(ContainSubstring("-baseline"), "the usage listing is still printed")
			Expect(string(stderr)).NotTo(ContainSubstring("-worktree"))
		})
	})

	When("a user reads the help text", func() {
		It("does not list the experimental flag, which stays accepted", func() {
			stdout, stderr, exitCode := runCoachRaw("codesignal", "--help")
			Expect(exitCode).To(Equal(0))
			Expect(string(stdout) + string(stderr)).NotTo(ContainSubstring("-worktree"))

			_, runStderr, runExit := runCoachBinary(commandPath, editedRepo(), nil, "codesignal", "--worktree", "--format=json")
			Expect(runExit).To(Equal(0), "stderr: %s", runStderr)
		})
	})
})
