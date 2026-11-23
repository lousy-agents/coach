//go:build unix

package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree lists in worktree_not_clean only a path that has no diagnostic of its own (#168 item 22)", func() {
	notCleanMessages := func(run worktreeScanRun) []string {
		messages := []string{}
		for _, diagnostic := range run.ofKind("worktree_not_clean") {
			messages = append(messages, diagnostic["message"].(string))
		}
		return messages
	}

	notCleanLines := func(text string) []string {
		lines := []string{}
		for line := range strings.SplitSeq(text, "\n") {
			if strings.Contains(line, "worktree_not_clean") {
				lines = append(lines, line)
			}
		}
		return lines
	}

	It("names an untracked nested repository only by its nested_repository entry, with no worktree_not_clean diagnostic (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addNestedRepository(repo, "nest")

		run := runProjectScan(repo)

		Expect(run.only("nested_repository")).To(HaveKeyWithValue("root", "nest"))
		Expect(run.ofKind("worktree_not_clean")).To(BeEmpty(), "the only unread path is nest/, which has its own entry")
	})

	It("keeps listing a dirty path that has no diagnostic of its own beside a nested repository (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addNestedRepository(repo, "nest")
		overwriteOnDiskCreatingParents(repo, "node_modules/pkg/readme.txt", "vendored\n")

		run := runProjectScan(repo)

		Expect(run.only("nested_repository")).To(HaveKeyWithValue("root", "nest"))
		messages := notCleanMessages(run)
		Expect(messages).To(ContainElement(ContainSubstring("node_modules/pkg/readme.txt")))
		Expect(messages).NotTo(ContainElement(ContainSubstring("nest/")), "the nested repository has its own nested_repository entry")
	})

	It("does not list the nested repository under worktree_not_clean in the text output (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addNestedRepository(repo, "nest")
		overwriteOnDiskCreatingParents(repo, "node_modules/pkg/readme.txt", "vendored\n")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree")

		Expect(string(stderr)).To(BeEmpty())
		Expect(exitCode).To(Equal(0))
		lines := notCleanLines(string(stdout))
		Expect(lines).To(ContainElement(ContainSubstring("node_modules/pkg/readme.txt")))
		Expect(lines).NotTo(ContainElement(ContainSubstring("nest/")))
	})

	It("names a dirty submodule only by its submodule entry, with no worktree_not_clean naming it (#168 item 22)", func() {
		if !installedGitCanBeToldNotToFetch() {
			Skip("git before 2.44 cannot be told not to fetch, so a status never lists the dirty submodule; the git_version row covers that git")
		}
		repo := scanRepoWith(nil)
		addLibrarySubmodule(repo, "third/lib")
		dirtySubmodule(repo, "third/lib")

		run := runProjectScan(repo)

		Expect(run.only("submodule")).To(HaveKeyWithValue("root", "third/lib"))
		Expect(notCleanMessages(run)).NotTo(ContainElement(ContainSubstring("third/lib")))
	})

	It("names a dirty submodule only by its git_version entry under git 2.43.0, with no worktree_not_clean naming it (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addLibrarySubmodule(repo, "third/lib")
		dirtySubmodule(repo, "third/lib")

		run := runWorktreeScan(repo, envWithGitBefore244(), "--project-config", "project.json")

		Expect(run.only("git_version")).To(HaveKeyWithValue("root", "third/lib"))
		Expect(notCleanMessages(run)).NotTo(ContainElement(ContainSubstring("third/lib")))
	})
})
