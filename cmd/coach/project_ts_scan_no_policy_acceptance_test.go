//go:build linux

package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal (real scan): a TypeScript scan with no policy anywhere stays file-local on a controlling terminal", func() {
	When("no policy is named, none is committed, and a controlling terminal is available", func() {
		It("renders the file-local report, opens no guided policy authoring, and writes no policy file", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "src/a.ts", "export const a = 1;\n")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), "",
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			report := decodeCoachReport(stdout)
			Expect(report.ProjectProvenance).To(BeNil(), "no policy was selected, so no project analysis may run")
			Expect(string(stderr)).NotTo(ContainSubstring("Start guided policy authoring"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "stderr: %s", stderr)
			Expect(string(stdout)).NotTo(ContainSubstring(`"roots"`), "a policy candidate must not reach stdout; stdout: %s", stdout)
			for _, policy := range []string{coachConfigPath, legacyConfigPath} {
				_, statErr := os.Stat(filepath.Join(repo, policy))
				Expect(os.IsNotExist(statErr)).To(BeTrue(), "%s must not be created by a scan with no policy", policy)
			}
		})
	})
})
