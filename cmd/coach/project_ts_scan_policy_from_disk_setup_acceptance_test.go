//go:build linux

package main

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

var _ = Describe("coach codesignal (TypeScript, controlling terminal): compiler setup for a scan whose policy is on disk and not committed", func() {
	const refusedWords = "is required before compiler setup"

	var (
		repo    string
		miseDir string
		path    string
	)

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", tstoolchain.SupportedTypescriptVersions[0]))
		miseDir = writeStatefulStubMiseScript(tstoolchain.SupportedTypescriptVersions[0])
		path = writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
	})

	When("the discovered policy is valid on disk and neither committed nor staged", func() {
		It("reaches the compiler-setup offer, and continues the same scan after the install", func() {
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			session.writeLine("project_mise")
			session.waitForPrompt("Type 'install' to run this setup command now")
			session.writeLine("install")
			stdout, stderr, _, exitCode := session.wait()

			Expect(string(stderr)).NotTo(ContainSubstring(refusedWords), "the offer must not be refused behind a committed-policy requirement; stderr: %s", stderr)
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "the confirmed selection must have invoked `mise install`")
		})

		It("prints that the policy is not committed before the choice prompt", func() {
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			disclosed := session.stderrBuf.String()
			session.writeLine("cancel")
			session.wait()

			Expect(disclosed).To(ContainSubstring("project_policy_not_committed"), "stderr before the prompt: %s", disclosed)
		})
	})
})
