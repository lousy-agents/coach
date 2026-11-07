package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Without a controlling terminal the way out is a command to run on one. For a policy that is committed with a
// document that is not valid, authoring cannot write at the file (--suggest-project-config creates, and refuses
// a target that exists), so the command printed is the scan itself: on a terminal it opens guided authoring and
// prints a replacement to save over the file. A command that writes somewhere else would leave the invalid file
// as it was.
var _ = Describe("coach codesignal (TypeScript, no controlling terminal): a committed policy whose document is not valid", func() {
	const brokenPolicy = "this is not a policy document\n"

	DescribeTable("prints the scan to rerun on a terminal, and no authoring command that cannot replace the file",
		func(policyPath, wantCommand string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policyPath, brokenPolicy)
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript"}, scanArgs...)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("on a terminal: "+wantCommand), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "authoring would create a different file and refuse this one:\n%s", stderr)
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json", "coach codesignal --baseline --project-language typescript"),
		Entry("named with --project-config", "pol.json", "coach codesignal --baseline --project-config pol.json --project-language typescript", "--project-config", "pol.json"),
	)

	It("still prints the authoring command for a policy that does not exist, which authoring creates", func() {
		repo := commitStubToolchainTypeScriptRepo("other.json", singleRootPolicyJSON)

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")),
			"codesignal", "--baseline", "--project-language", "typescript", "--project-config", "missing.json")

		Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("on a terminal: coach codesignal --baseline --suggest-project-config --project-language typescript --output missing.json"), "stderr: %s", stderr)
	})
})
