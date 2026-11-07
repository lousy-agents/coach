//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

var _ = Describe("coach codesignal (TypeScript, controlling terminal): compiler setup for a scan whose policy was discovered, not named", func() {
	It("continues the same scan after a successful install, judging the discovered policy from the analyzed revision as it always has", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
		writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", tstoolchain.SupportedTypescriptVersions[0]))
		miseDir := writeStatefulStubMiseScript(tstoolchain.SupportedTypescriptVersions[0])
		path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

		session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
			"codesignal", "--baseline", "--project-language", "typescript", "--format=json")
		session.waitForPrompt("an unrecognized or blank answer cancels.")
		session.writeLine("project_mise")
		session.waitForPrompt("Type 'install' to run this setup command now")
		session.writeLine("install")
		stdout, stderr, _, exitCode := session.wait()

		Expect(exitCode).To(Equal(0), "the readiness rerun must pass the discovered policy so the scan continues; stderr: %s", stderr)
		Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "sanity: the confirmed selection must have invoked `mise install`")
		var report map[string]any
		Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout: %s stderr: %s", stdout, stderr)
		Expect(report).To(HaveKey("coverage"))
	})
})
