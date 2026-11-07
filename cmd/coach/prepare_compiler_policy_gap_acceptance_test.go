//go:build linux

package main

import (
	"os"
	"path/filepath"
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

var _ = Describe("coach codesignal --prepare-compiler with an explicit policy that is missing", func() {
	DescribeTable("names the remedy readiness names for the policy, not always a new policy",
		func(prepare func(repo string), want []string, unwanted []string) {
			repo := newTempGitRepo()
			prepare(repo)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(
				commandPath, repo, stubToolchainEnv(pathWithoutNode()), "",
				"codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript", "--project-config", "pol.json",
			)

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("a reviewed, committed policy is required before compiler setup (policy_missing)"), "stderr=%s", stderr)
			for _, text := range want {
				Expect(string(stderr)).To(ContainSubstring(text), "stderr=%s", stderr)
			}
			for _, text := range unwanted {
				Expect(string(stderr)).NotTo(ContainSubstring(text), "stderr=%s", stderr)
			}
		},
		Entry("a policy committed and gone from disk is restored, not authored again", func(repo string) {
			commitFile(repo, "pol.json", singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, "pol.json"))).To(Succeed())
		}, []string{"restore_policy", "git restore --source=", "':(top,literal)pol.json'"}, []string{"author_policy", "guided policy authoring"}),
		Entry("a policy that was never committed is authored", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		}, []string{"author_policy", "guided policy authoring"}, []string{"restore_policy", "git restore"}),
	)

	It("names neither a restore nor authoring when a file is in the way of the policy path", func() {
		repo := newTempGitRepo()
		commitFile(repo, "dir/pol.json", singleRootPolicyJSON)
		Expect(os.RemoveAll(filepath.Join(repo, "dir"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "dir"), []byte("my notes"), 0o644)).To(Succeed())

		stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(
			commandPath, repo, stubToolchainEnv(pathWithoutNode()), "",
			"codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript", "--project-config", "dir/pol.json",
		)

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("policy_missing"))
		Expect(string(stderr)).NotTo(ContainSubstring("author_policy"), "stderr=%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "stderr=%s", stderr)
	})
})
