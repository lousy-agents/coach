package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Without a controlling terminal the way out is a command to run on one. For a policy that is committed with a
// document that is not valid there is none: authoring cannot write at the file (--suggest-project-config creates,
// and refuses a target that exists), and the scan that opens guided authoring on a terminal ends by asking the user
// to save a replacement over the file, so no line leads to it. The config error still says what is wrong.
var _ = Describe("coach codesignal (TypeScript, no controlling terminal): a committed policy whose document is not valid", func() {
	const brokenPolicy = "this is not a policy document\n"

	DescribeTable("says in words how to get past it, and leads to no session and no command",
		func(policyPath string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policyPath, brokenPolicy)
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript"}, scanArgs...)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("on a terminal:"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("guided authoring"), "the session ends by saving over the file:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "authoring would create a different file and refuse this one:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("coach codesignal: the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one."), "the scan says in words how to get past the policy:\n%s", stderr)
			expectNoRunnableCommand(string(stderr))
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json"),
		Entry("named with --project-config", "pol.json", "--project-config", "pol.json"),
	)

	It("still prints the authoring command for a policy that does not exist, which authoring creates", func() {
		repo := commitStubToolchainTypeScriptRepo("other.json", singleRootPolicyJSON)

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")),
			"codesignal", "--baseline", "--project-language", "typescript", "--project-config", "missing.json")

		Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("on a terminal: coach codesignal --baseline --suggest-project-config --project-language typescript --output missing.json"), "stderr: %s", stderr)
	})
})
