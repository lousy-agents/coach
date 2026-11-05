package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --base --worktree names a changed file in a language it does not analyze", func() {
	diffRepo := func() (repo, base string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "notes.txt", "first\n")
		base = commitFile(repo, "pkg/model/seed.go", "package model\n")
		return repo, base
	}
	unsupportedPaths := func(stdout []byte) []string {
		paths := []string{}
		for _, diagnostic := range diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "unsupported_language") {
			paths = append(paths, diagnostic.Path)
		}
		return paths
	}

	notCleanMessages := func(stdout []byte) []string {
		messages := []string{}
		for _, diagnostic := range diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_not_clean") {
			messages = append(messages, diagnostic.Message)
		}
		return messages
	}

	It("names a file edited only on disk, as a committed diff names a committed edit", func() {
		repo, base := diffRepo()
		overwriteOnDisk(repo, "notes.txt", "edited\n")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(unsupportedPaths(stdout)).To(ConsistOf("notes.txt"))
	})

	It("names an untracked file in an unsupported language", func() {
		repo, base := diffRepo()
		overwriteOnDisk(repo, "fresh.txt", "new\n")

		stdout, _, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0))
		Expect(unsupportedPaths(stdout)).To(ConsistOf("fresh.txt"))
	})

	It("names a file once when the committed diff and the working tree both changed it", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		base := commitFile(repo, "notes.txt", "first\n")
		commitFile(repo, "notes.txt", "second\n")
		overwriteOnDisk(repo, "notes.txt", "third\n")

		stdout, _, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0))
		Expect(unsupportedPaths(stdout)).To(ConsistOf("notes.txt"))
	})

	It("names nothing for a clean tree (control)", func() {
		repo, base := diffRepo()

		stdout, _, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0))
		Expect(unsupportedPaths(stdout)).To(BeEmpty())
	})

	It("does not also list a named file as a dirty file the scan ignored", func() {
		repo, base := diffRepo()
		overwriteOnDisk(repo, "notes.txt", "edited\n")
		overwriteOnDisk(repo, "fresh.txt", "new\n")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(unsupportedPaths(stdout)).To(ConsistOf("notes.txt", "fresh.txt"))
		for _, message := range notCleanMessages(stdout) {
			Expect(message).NotTo(ContainSubstring("notes.txt"), "the file already has its own unsupported_language diagnostic")
			Expect(message).NotTo(ContainSubstring("fresh.txt"), "the file already has its own unsupported_language diagnostic")
		}
	})

	It("still lists a dirty file that no unsupported_language diagnostic names (control)", func() {
		repo, base := diffRepo()
		Expect(os.MkdirAll(filepath.Join(repo, "node_modules/pkg"), 0o755)).To(Succeed())
		overwriteOnDisk(repo, "node_modules/pkg/readme.txt", "vendored\n")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(unsupportedPaths(stdout)).To(BeEmpty(), "a path under node_modules is not named as unsupported")
		Expect(notCleanMessages(stdout)).To(ContainElement(ContainSubstring("node_modules/pkg/readme.txt")))
	})

	DescribeTable("does not name a project policy file as an unsupported language",
		func(policyPath string) {
			repo := goOverlayRepo()
			base := commitFile(repo, policyPath, goLayerPolicyConfigJSON+"\n")
			overwriteOnDisk(repo, policyPath, goLayerPolicyConfigJSON+"\n\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report := decodeCoachReport(stdout)
			Expect(unsupportedPaths(stdout)).NotTo(ContainElement(policyPath))
			Expect(report.Summary.FilesUnanalyzed).To(Equal(1),
				"the edited policy is counted once, by its worktree_changes_not_analyzed diagnostic, not as source in an unsupported language")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(ConsistOf(HaveField("Path", policyPath)),
				"nothing read the edited policy, so the dirty file is disclosed under its own diagnostic")
			Expect(notCleanMessages(stdout)).NotTo(ContainElement(ContainSubstring(policyPath)),
				"the list of unread changes does not repeat a path that has its own diagnostic")
		},
		Entry("the canonical policy file", ".lousy-agents/coach.json"),
		Entry("the deprecated root policy file", "project.json"),
	)

	It("leaves a project configuration file to scope.worktree.config_files instead of naming it as unsupported", func() {
		repo, base := diffRepo()
		overwriteOnDisk(repo, "go.mod", "module example.com/renamed\n\ngo 1.25\n")

		stdout, _, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0))
		Expect(unsupportedPaths(stdout)).NotTo(ContainElement("go.mod"))
	})
})
