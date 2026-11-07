package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The way out for a committed policy that cannot be used is a new document at its path, and it is offered only
// where it loses nothing: the copy in the working tree is the committed copy, and the index holds no edit and no
// conflict for it. Otherwise there is work in the file, and Coach says to review it and commit it, whichever
// language the scan is for. Every command Coach prints is pasted, and the replacement is told in words, because a
// line that overwrites a user's fix looks the same as a good one in the text.
const (
	validPolicyWithLayers = `{"schema_version":"1","roots":["."],"layers":[{"name":"core","prefixes":["core"]}]}` + "\n"
	conflictedMarkers     = "<<<<<<< ours\n{\"schema_version\":\"1\",\"roots\":[\"a\"]}\n=======\n{\"schema_version\":\"1\",\"roots\":[\"b\"]}\n>>>>>>> theirs\n"
)

type usersWork struct {
	arrange   func(repo, policy string) (onDisk string)
	wantWords []string
	wantCause string
}

func pasteWhatCoachPrints(repo string, stderr []byte) {
	GinkgoHelper()
	if suggest := suggestCommandIn(lastLine(stderr)); suggest != "" {
		runInShell(repo, strings.Replace(suggest, "coach ", commandPath+" ", 1))
	}
}

func theUsersWorkEntries() []TableEntry {
	return []TableEntry{
		Entry("a fix on disk that is not committed", usersWork{func(repo, policy string) string {
			writeWorktreeFile(repo, policy, validPolicyWithLayers)
			return validPolicyWithLayers
		}, []string{"differs from the committed", "git diff"}, "changed_on_disk"}),
		Entry("a fix on disk that is staged", usersWork{func(repo, policy string) string {
			writeWorktreeFile(repo, policy, validPolicyWithLayers)
			gitIn(repo, "add", policy)
			return validPolicyWithLayers
		}, []string{"staged edit", "git diff --cached"}, "staged_edit"}),
		Entry("an unresolved conflict", usersWork{func(repo, policy string) string {
			stageConflicting(repo, policy, 2, 3)
			writeWorktreeFile(repo, policy, conflictedMarkers)
			return conflictedMarkers
		}, []string{"merge conflict", "resolve the conflict in it, mark it resolved, and commit it"}, "merge_conflict"}),
	}
}

var _ = Describe("coach codesignal (Go): a committed policy that is invalid, with work in the file", func() {
	const policy = "cfg/p.json"

	DescribeTable("says to review the file, and keeps the user's bytes whatever is pasted",
		func(work usersWork) {
			repo := commitGoRepoWithPolicy(policy, invalidPolicy)
			onDisk := work.arrange(repo, policy)
			index := gitIn(repo, "ls-files", "--stage", "--", policy)

			_, stderr := scanExpectingRefusal(repo, "--project-config", policy)
			pasteWhatCoachPrints(repo, stderr)

			for _, words := range work.wantWords {
				Expect(string(stderr)).To(ContainSubstring(words), "stderr:\n%s", stderr)
			}
			expectNoRunnableCommand(string(stderr))
			Expect(string(stderr)).NotTo(ContainSubstring("replace the policy with it"), "a replacement would discard the work in the file:\n%s", stderr)
			content, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal(onDisk), "the user's bytes survive what Coach printed")
			Expect(gitIn(repo, "ls-files", "--stage", "--", policy)).To(Equal(index), "the index is untouched")
		},
		theUsersWorkEntries(),
	)

	It("does not tell the user to replace the candidate that Coach's own advice for a deleted policy produced", func() {
		repo := commitGoRepoWithPolicy(policy, invalidPolicy)
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		_, first := scanExpectingRefusal(repo, "--project-config", policy)
		suggest := strings.Replace(regexpFor(`coach codesignal --baseline --suggest-project-config --output \S+`, first), "coach ", commandPath+" ", 1)
		Expect(suggest).NotTo(BeEmpty(), "the first message names the command that writes the policy:\n%s", first)
		output, exitCode := runInShell(repo, suggest)
		Expect(exitCode).To(Equal(0), "command: %s\noutput: %s", suggest, output)
		writeWorktreeFile(repo, policy, validPolicyWithLayers)

		_, second := scanExpectingRefusal(repo, "--project-config", policy)
		pasteWhatCoachPrints(repo, second)

		expectNoRunnableCommand(string(second))
		Expect(string(second)).NotTo(ContainSubstring("replace the policy with it"), "a replacement would discard the layers the user added:\n%s", second)
		content, err := os.ReadFile(filepath.Join(repo, policy))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal(validPolicyWithLayers), "the layers the user added survive")
	})

	It("still offers the replacement where the file on disk is the committed copy and the index holds nothing else", func() {
		repo := commitGoRepoWithPolicy(policy, invalidPolicy)

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(suggestCommandIn(lastLine(stderr))).NotTo(BeEmpty(), "stderr:\n%s", stderr)
		Expect(lastLine(stderr)).To(ContainSubstring("replace the policy with it, commit, and rerun"), "stderr:\n%s", stderr)
		expectNoRunnableCommand(lastLine(stderr))
	})
})

var _ = Describe("coach codesignal --check-project: a committed policy that is invalid, with work in the file", func() {
	const policy = "cfg/p.json"

	DescribeTable("names the cause and offers no new policy, as the scan does",
		func(work usersWork) {
			repo := commitGoRepoWithPolicy(policy, invalidPolicy)
			work.arrange(repo, policy)

			doc := expectCheckProject(repo, "--project-config", policy)

			Expect(causeOfGap(doc, "policy_invalid")).To(Equal(work.wantCause), "gaps: %+v", doc.Gaps)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "a new policy would replace the work in the file")
		},
		theUsersWorkEntries(),
	)

	It("still offers a new policy where the file is the committed copy", func() {
		repo := commitGoRepoWithPolicy(policy, invalidPolicy)

		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
	})
})

var _ = Describe("coach codesignal (TypeScript, no terminal): a committed policy that is invalid, with work in the file", func() {
	const policy = "pol.json"

	DescribeTable("names no scan that would replace the file",
		func(work usersWork) {
			repo := commitStubToolchainTypeScriptRepo(policy, invalidPolicy)
			work.arrange(repo, policy)
			args := []string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json", "--project-config", policy}

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("guided authoring"), "that session ends by saving a candidate over the file:\n%s", stderr)
			for _, words := range work.wantWords {
				Expect(string(stderr)).To(ContainSubstring(words), "stderr:\n%s", stderr)
			}
		},
		theUsersWorkEntries(),
	)
})
