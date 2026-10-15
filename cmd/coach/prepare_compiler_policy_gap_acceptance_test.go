//go:build linux

package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --prepare-compiler with a canonical policy that does not pass", func() {
	DescribeTable("exits 2 and names the policy gap in the refusal",
		func(contents string) {
			repo := newTempGitRepo()
			commitFile(repo, coachConfigPath, contents)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(
				commandPath, repo, stubToolchainEnv(pathWithoutNode()), "",
				"codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript",
			)

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("a reviewed, committed policy is required before compiler setup (policy_invalid)"), "stderr=%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "a policy gap must refuse before any compiler-setup prompt; stderr=%s", stderr)
		},
		Entry("invalid canonical file", invalidPolicyText),
		Entry("oversized canonical file", strings.Repeat("x", (1<<20)+1)),
	)
})
