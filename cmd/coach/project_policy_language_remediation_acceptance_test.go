//go:build linux

package main

import (
	"fmt"
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// scanOnControllingTerminalWithCompilerGap runs a scan with a controlling terminal, no offer budget, and a compiler
// gap that genuinely offers project_package, returning what it wrote.
func scanOnControllingTerminalWithCompilerGap(policy string, f codesignalFlags) (stdout, stderr []byte, exitCode int) {
	nodeDir := writeStubNodeScript("v24.9.9")
	npmDir := writeStubPackageManagerScript("npm", "11.0.0")
	GinkgoT().Setenv("PATH", nodeDir+string(os.PathListSeparator)+npmDir+string(os.PathListSeparator)+pathExcludingToolchain())
	GinkgoT().Setenv("HOME", os.Getenv("HOME"))
	GinkgoT().Setenv("CI", "")

	repo := newTempGitRepo()
	commitFile(repo, "project.json", policy)
	commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", tstoolchain.SupportedTypescriptVersions[0]))
	commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

	master, slave := openPTYPair()
	DeferCleanup(master.Close)
	DeferCleanup(slave.Close)
	originalStdin := os.Stdin
	os.Stdin = slave
	DeferCleanup(func() { os.Stdin = originalStdin })
	_, writeErr := master.WriteString("cancel\n")
	Expect(writeErr).NotTo(HaveOccurred())

	outRead, outWrite, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	errRead, errWrite, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())

	exitCode = runCodesignalScan(repo, f, os.Stdin, outWrite, errWrite, scanOfferBudget{}, defaultScanPorts())
	Expect(outWrite.Close()).To(Succeed())
	Expect(errWrite.Close()).To(Succeed())
	stdout, readErr := io.ReadAll(outRead)
	Expect(readErr).NotTo(HaveOccurred())
	stderr, readErr = io.ReadAll(errRead)
	Expect(readErr).NotTo(HaveOccurred())
	return stdout, stderr, exitCode
}

var _ = Describe("coach codesignal: advice that depends on the language follows the language the scan ran in (#805)", func() {
	When("the policy names typescript, a controlling terminal is available, and the compiler gap offers project_package", func() {
		It("words the refusal as it does for --project-language typescript: one line, with no command appended beside the offer", func() {
			named := policyref.Named("project.json")
			byFlag := codesignalFlags{baseline: true, format: "json", namedPolicy: named, projectLanguage: "typescript", projectLanguageSet: true}
			byPolicy := codesignalFlags{baseline: true, format: "json", namedPolicy: named, projectLanguage: "go"}

			_, flagStderr, flagExit := scanOnControllingTerminalWithCompilerGap(`{"schema_version":"1","roots":["."]}`+"\n", byFlag)
			stdout, policyStderr, policyExit := scanOnControllingTerminalWithCompilerGap(`{"schema_version":"1","language":"typescript","roots":["."]}`+"\n", byPolicy)

			Expect(flagExit).To(Equal(2), "the flag's own refusal, to be matched: %s", flagStderr)
			Expect(policyExit).To(Equal(flagExit), "stdout: %s stderr: %s", stdout, policyStderr)
			Expect(stdout).To(BeEmpty())
			Expect(stderrLines(policyStderr)).To(Equal(stderrLines(flagStderr)))
		})
	})
})
