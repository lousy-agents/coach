package main

import (
	"encoding/json"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree counts a path left unmerged by a conflict", func() {
	conflictedRepo := func(onMain, onOther func(repo string)) string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 1\n")
		runGitInRepo(repo, "checkout", "-q", "-b", "other")
		onOther(repo)
		runGitInRepo(repo, "checkout", "-q", "-")
		onMain(repo)
		merge := exec.Command("git", "merge", "other")
		merge.Dir = repo
		merge.Env = commitEnv
		output, _ := merge.CombinedOutput()
		Expect(string(output)).To(ContainSubstring("CONFLICT"))
		return repo
	}
	worktreeCounts := func(repo string) map[string]any {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		var report struct {
			Scope struct {
				Worktree map[string]any `json:"worktree"`
			} `json:"scope"`
		}
		ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed())
		return report.Scope.Worktree
	}

	It("counts a file deleted by the other side and still on disk as modified, not deleted", func() {
		repo := conflictedRepo(
			func(repo string) { commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 2\n") },
			func(repo string) {
				runGitInRepo(repo, "rm", "-q", "pkg/model/m.go")
				runGitInRepo(repo, "commit", "-q", "-m", "delete m.go")
			},
		)
		expectDirtyPorcelain(repo, "UD pkg/model/m.go")

		counts := worktreeCounts(repo)

		Expect(counts).To(HaveKeyWithValue("modified", float64(1)), "the conflicted file is present on disk and at HEAD")
		Expect(counts).To(HaveKeyWithValue("deleted", float64(0)))
	})

	It("does not select a both-added path as a new file; the diff scan names it as unanalyzed", func() {
		repo := conflictedRepo(
			func(repo string) { commitFile(repo, "pkg/model/n.go", "package model\n\nvar N = 1\n") },
			func(repo string) { commitFile(repo, "pkg/model/n.go", "package model\n\nvar N = 2\n") },
		)
		expectDirtyPorcelain(repo, "AA pkg/model/n.go")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, stdout)
		report := decodeCoachReport(stdout)
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(ConsistOf(HaveField("Path", "pkg/model/n.go")),
			"HEAD holds the file, so it is not new; an unmerged file is disclosed, not analyzed with its conflict markers")
		for _, signal := range report.Signals {
			Expect(signal.Path).NotTo(Equal("pkg/model/n.go"), "no signal may come from a file read with conflict markers in it")
		}
	})

	It("counts a file both sides added, which HEAD already holds, as modified, not added", func() {
		repo := conflictedRepo(
			func(repo string) { commitFile(repo, "pkg/model/n.go", "package model\n\nvar N = 1\n") },
			func(repo string) { commitFile(repo, "pkg/model/n.go", "package model\n\nvar N = 2\n") },
		)
		expectDirtyPorcelain(repo, "AA pkg/model/n.go")

		counts := worktreeCounts(repo)

		Expect(counts).To(HaveKeyWithValue("modified", float64(1)))
		Expect(counts).To(HaveKeyWithValue("added", float64(0)))
	})
})
