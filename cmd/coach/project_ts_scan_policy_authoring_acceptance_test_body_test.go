//go:build linux

package main

import (
	"fmt"
	"os"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func body_projectTsScanPolicyAuthoringAcceptanceTest_withholdsProjectPackageWithItsReasonRatherThanSi_679() {
	repo := newTempGitRepo()
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
	manifest := fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", tstoolchain.SupportedTypescriptVersions[0])
	for _, pkg := range []string{"packages/a", "packages/b"} {
		commitFile(repo, pkg+"/package.json", manifest)
		commitFile(repo, pkg+"/package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
		commitFile(repo, pkg+"/tsconfig.json", `{"compilerOptions":{}}`+"\n")
	}

	npmDir := writeRecordingStubPackageManagerScript("npm", "11.0.0")
	path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + pathExcludingToolchain()

	stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), "",
		"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

	Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
	Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
	Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "an install that can satisfy at most one of two selected roots must never be offered at all; stderr: %s", stderr)
	Expect(string(stderr)).To(ContainSubstring("manifest_context_ambiguous"), "the customer must be told why the project-package choice was ruled out, not silently handed a default; stderr: %s", stderr)
}

func body_projectTsScanPolicyAuthoringAcceptanceTest_opensTheMenuWithoutProjectPackageAndNamesTheReas_711() {
	repo := newTempGitRepo()
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
	manifest := fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", tstoolchain.SupportedTypescriptVersions[0])
	for _, pkg := range []string{"packages/a", "packages/b"} {
		commitFile(repo, pkg+"/package.json", manifest)
		commitFile(repo, pkg+"/package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
		commitFile(repo, pkg+"/tsconfig.json", `{"compilerOptions":{}}`+"\n")
	}
	writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", tstoolchain.SupportedTypescriptVersions[0]))

	npmDir := writeRecordingStubPackageManagerScript("npm", "11.0.0")
	miseDir := writeStatefulStubMiseScript(tstoolchain.SupportedTypescriptVersions[0])
	path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

	session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
		"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	session.waitForPrompt("an unrecognized or blank answer cancels.")
	session.writeLine("cancel")
	stdout, stderr, _, exitCode := session.wait()

	Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
	Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
	Expect(string(stderr)).To(ContainSubstring("project_package is not offered here (manifest_context_ambiguous)"), "a choice removed from a menu the customer can still see must say why; stderr: %s", stderr)
	Expect(string(stderr)).To(ContainSubstring("- project_mise"), "the surviving mise choice must still be offered; stderr: %s", stderr)
	Expect(string(stderr)).NotTo(ContainSubstring("  - project_package"), "the unserviceable choice must not appear in the menu itself; stderr: %s", stderr)
	Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "cancelling must never invoke `mise install`")
}
