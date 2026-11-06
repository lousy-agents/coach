package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree accounts for the paths the scope filter excludes, and only those", func() {
	cleanRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		return repo
	}
	notAnalyzed := func(repo string, extraArgs ...string) []string {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, append([]string{"--format=json", "--worktree"}, extraArgs...)...)
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		paths := []string{}
		for _, diagnostic := range diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed") {
			paths = append(paths, diagnostic.Path)
		}
		return paths
	}

	When("an untracked test file sits under the default production scope", func() {
		It("is counted as excluded and is not also disclosed as unanalyzed", func() {
			repo := cleanRepo()
			Expect(os.WriteFile(filepath.Join(repo, "pkg/handlers/handlers_test.go"), []byte("package handlers\n"), 0o644)).To(Succeed())

			Expect(notAnalyzed(repo)).To(BeEmpty())
			text, _, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree")
			Expect(exitCode).To(Equal(0))
			Expect(noFindingsHeadline(string(text))).To(Equal("No active CodeSignal findings."),
				"--scope all gives a plain all-clear for the same tree, so the production scope must too")
		})

		It("gives the same plain all-clear under --scope all (control)", func() {
			repo := cleanRepo()
			Expect(os.WriteFile(filepath.Join(repo, "pkg/handlers/handlers_test.go"), []byte("package handlers\n"), 0o644)).To(Succeed())

			Expect(notAnalyzed(repo, "--scope", "all")).To(BeEmpty())
		})
	})

	When("a changed file is neither analyzed nor excluded by the scope", func() {
		It("stays disclosed: a diff scan does not select an unmerged file for file-local analysis", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 1\n")
			runGitInRepo(repo, "checkout", "-q", "-b", "other")
			commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 2\n")
			runGitInRepo(repo, "checkout", "-q", "-")
			commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 3\n")
			merge := exec.Command("git", "merge", "other")
			merge.Dir = repo
			merge.Env = commitEnv
			mergeOutput, _ := merge.CombinedOutput()
			Expect(string(mergeOutput)).To(ContainSubstring("CONFLICT"))

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--format=json", "--worktree")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			paths := []string{}
			for _, diagnostic := range diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed") {
				paths = append(paths, diagnostic.Path)
			}
			Expect(paths).To(ContainElement("pkg/model/m.go"),
				"a path the scan did not analyze and the scope did not exclude must not be reported as handled")

			_, failStderr, failExit := runCoachCodesignalRaw(repo, "HEAD", "--format=json", "--worktree", "--fail-on-incomplete-coverage")
			Expect(failExit).To(Equal(3), "a disclosed unread path qualifies the verdict, so the exit status must say so; stderr: %s", failStderr)
		})
	})
})
