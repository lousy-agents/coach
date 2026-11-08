package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: an explicit policy that is on disk but not committed is committed, not authored again", func() {
	const uncommittedPolicy = "un.json"
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		writeWorktreeFile(repo, uncommittedPolicy, singleRootPolicyJSON)
	})

	scan := func(language string) (stderr []byte) {
		args := []string{"codesignal", "--baseline", "--project-config", uncommittedPolicy, "--format=json"}
		if language != "" {
			args = append(args, "--project-language", language)
		}
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)
		ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		ExpectWithOffset(1, string(stderr)).To(ContainSubstring("exists in the worktree but is not committed"))
		return stderr
	}

	It("says to commit the file and offers no new policy for a Go scan", func() {
		stderr := scan("")

		Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "a file is already there; a new policy cannot be written at the path:\n%s", stderr)
	})

	It("says to commit the file and offers no new policy for a TypeScript scan", func() {
		stderr := scan("typescript")

		Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
	})

	It("names neither author_policy nor restore_policy as the next action of --check-project", func() {
		doc := expectCheckProject(repo, "--project-config", uncommittedPolicy)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
	})
})

var _ = Describe("coach codesignal: an explicit policy over the size budget keeps the bound text and the offer of a smaller policy", func() {
	const oversizePolicy = "big.json"
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, oversizePolicy, strings.Repeat(" ", 1<<20+1)+singleRootPolicyJSON)
	})

	It("names the bound that tripped and still offers a new policy", func() {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", oversizePolicy, "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("is not readable at revision"))
		Expect(string(stderr)).To(ContainSubstring("git stdout exceeded"), "the text of the bound that tripped stays:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(suggestCommand), "a smaller policy fixes a policy over the size budget:\n%s", stderr)
	})
})
