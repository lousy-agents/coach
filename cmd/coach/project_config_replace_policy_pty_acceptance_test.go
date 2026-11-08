//go:build linux

package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A committed policy over the size budget is fixed by a new document as one that is not valid is, and a TypeScript scan
// on a terminal opens no guided authoring for it either: the only session it has ends by asking the user to save a
// candidate over the file. It prints the config error, no candidate, and nothing about saving over the file.
var _ = Describe("coach codesignal (TypeScript, controlling terminal): a committed policy over the size budget", func() {
	DescribeTable("opens no guided authoring and prints no candidate",
		func(policyPath string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policyPath, bigPolicy())
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, scanArgs...)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), discoveredAuthoringSessionAnswers, args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty(), "no candidate is printed to save over the file; stdout: %s", stdout)
			Expect(string(stderr)).NotTo(ContainSubstring("replacing the current file"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("guided authoring"), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("coach codesignal: the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one."), "the scan says in words how to get past the policy; stderr: %s", stderr)
			expectNoRunnableCommand(string(stderr))
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json"),
		Entry("named with --project-config", "pol.json", "--project-config", "pol.json"),
		Entry("named with --project-config at the canonical path", ".lousy-agents/coach.json", "--project-config", ".lousy-agents/coach.json"),
	)
})

// Where there is work in the file, a terminal opens no session that ends in a candidate to save over it: the message
// says to review the file and commit it, as it does without a terminal.
var _ = Describe("coach codesignal (TypeScript, controlling terminal): a committed policy that is invalid, with work in the file", func() {
	DescribeTable("opens no guided authoring and keeps the user's bytes",
		func(work usersWork) {
			const policy = "pol.json"
			repo := commitStubToolchainTypeScriptRepo(policy, invalidPolicy)
			onDisk := work.arrange(repo, policy)
			args := []string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json", "--project-config", policy}

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), discoveredAuthoringSessionAnswers, args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty(), "no candidate is printed to save over the file; stdout: %s", stdout)
			Expect(string(stderr)).NotTo(ContainSubstring("replacing the current file"), "stderr: %s", stderr)
			content, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal(onDisk))
		},
		theUsersWorkEntries(),
	)
})
