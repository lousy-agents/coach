package main

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --worktree names a dirty configuration file the analysis did not read", func() {
	const webManifest = `{"name":"web","version":"1.0.0"}`

	conflictedGoRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 1\n")
		commitFile(repo, "web/package.json", webManifest)
		runGitInRepo(repo, "checkout", "-q", "-b", "other")
		commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 2\n")
		runGitInRepo(repo, "checkout", "-q", "-")
		commitFile(repo, "pkg/model/m.go", "package model\n\nvar Value = 3\n")
		merge := exec.Command("git", "merge", "other")
		merge.Dir = repo
		merge.Env = commitEnv
		mergeOutput, _ := merge.CombinedOutput()
		Expect(string(mergeOutput)).To(ContainSubstring("CONFLICT"))
		return repo
	}
	messagesOfKind := func(report *codesignal.Report, kind string) []string {
		messages := []string{}
		for _, diagnostic := range diagnosticsOfKind(report.Diagnostics, kind) {
			messages = append(messages, diagnostic.Message)
		}
		return messages
	}

	It("names it beside a path that is disclosed on its own", func() {
		repo := conflictedGoRepo()
		overwriteOnDisk(repo, "web/package.json", `{"name":"web","version":"2.0.0"}`)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--format=json", "--worktree")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(ContainElement(HaveField("Path", "pkg/model/m.go")),
			"the conflicted source is disclosed per path, which is what hid the manifest")
		Expect(messagesOfKind(report, "worktree_not_clean")).To(ContainElement(ContainSubstring("web/package.json")),
			"a Go project's analysis never opens a package.json, so the edit to it was not read")
		Expect(messagesOfKind(report, "worktree_not_clean")).To(ContainElement(MatchRegexp(`^working-tree changes this scan did not read: .*web/package\.json`)),
			"--worktree exists to scan a dirty tree, so it says which changes it left unread, not that the tree is not clean")
	})

	It("names no configuration file when none is dirty (control)", func() {
		repo := conflictedGoRepo()

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--format=json", "--worktree")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(messagesOfKind(decodeCoachReport(stdout), "worktree_not_clean")).To(BeEmpty())
	})
})
