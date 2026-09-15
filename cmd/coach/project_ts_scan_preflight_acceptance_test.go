package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// D3 fixes the existing single-line remediation classifyAnalysisError has
// always printed for a real (non --check-project, non --prepare-compiler)
// scan's CompilerUnresolvedError/ProjectConfigError, and requires Task 7
// (#330) to append its own supported interactive-setup command while a
// controlling terminal is unavailable, without changing that first line or
// the exit code. These specs exercise the appended line only -- every
// existing exact-match assertion elsewhere in this package for the
// unchanged first line was updated alongside this file to expect it too.
var _ = Describe("coach codesignal (real scan): appended interactive-setup remediation (AC-SET-9)", func() {
	When("a --baseline scan's explicit --project-config resolves no TypeScript compiler and no controlling terminal is available", func() {
		It("keeps the existing --check-project remediation line unchanged, appends the --prepare-compiler remediation, leaves stdout empty, and exits 2", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "the existing D3 remediation line must not change")
			Expect(lines[1]).To(Equal("coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"), "AC-SET-9's appended command must be the interactive setup flow that resolves this exact gap")
		})
	})

	When("a --base diff scan hits the same unresolved-compiler gap", func() {
		It("applies the identical appended remediation, since classifyAnalysisError is shared by both scan modes", func() {
			repo := newTempGitRepo()
			baseSHA := commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(path), "codesignal", "--base", baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(lines[1]).To(Equal("coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"))
		})
	})

	When("a --baseline scan's explicit --project-config names a policy that was never committed", func() {
		It("keeps the existing project_config_invalid message unchanged, appends the --suggest-project-config remediation, leaves stdout empty, and exits 2", func() {
			repo := newTempGitRepo()
			commitFile(repo, "README.md", "seed\n")
			// project.json is deliberately never committed: loadProjectConfig
			// fails with ProjectConfigError before any language-specific
			// backend or compiler resolution runs.

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "stderr: %s", stderr)
			Expect(lines[0]).To(ContainSubstring("project.json"), "the existing project_config_invalid message must still name the path")
			Expect(lines[1]).To(Equal("coach codesignal --baseline --suggest-project-config --project-language typescript"), "AC-SET-9's appended command must never combine --suggest-project-config with --project-config (validateSuggestProjectConfigFlags rejects that combination)")
		})
	})
})

// stderrLines splits stderr on newlines after trimming exactly one trailing
// newline, so each fmt.Fprintln call's line is a distinct element rather
// than leaving a spurious empty trailing element.
func stderrLines(stderr []byte) []string {
	return strings.Split(strings.TrimRight(string(stderr), "\n"), "\n")
}
