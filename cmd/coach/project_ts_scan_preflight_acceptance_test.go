package main

import (
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// D3 fixes the existing single-line remediation classifyAnalysisError has
// always printed for a real (non --check-project, non --prepare-compiler)
// scan's CompilerUnresolvedError/ProjectConfigError, and requires Task 7
// (#330) to append its own supported interactive-setup command while a
// controlling terminal is unavailable, without changing that first line or
// the exit code. These specs exercise the appended line only.
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

	When("a --baseline scan's explicit --project-config resolves node_missing and no controlling terminal is available", func() {
		It("keeps the existing --check-project remediation as the only stderr line, since Coach has no executable remediation for a runtime-boundary gap", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithoutNode()
			requireNodeUnreachable(path)

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(1), "node_missing has no executable prepare-compiler remediation, so no line must be appended; stderr: %s", stderr)
			Expect(lines[0]).To(Equal("node_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
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
		It("keeps the existing project_config_invalid message unchanged, additionally reports the compiler gap this repository also fails (AC-SET-13), appends the --suggest-project-config remediation, leaves stdout empty, and exits 2", func() {
			repo := newTempGitRepo()
			commitFile(repo, "README.md", "seed\n")
			// project.json is deliberately never committed: loadProjectConfig
			// fails with ProjectConfigError before any language-specific
			// backend or compiler resolution runs. This repository also has
			// no package.json anywhere, but checks.compiler runs
			// unconditionally on project_shape's own result (CheckProjectReadiness),
			// so readiness's gaps[] independently carries
			// typescript_compiler_missing here too -- AC-SET-13 requires
			// reporting it rather than only the masking policy failure.

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(3), "stderr: %s", stderr)
			Expect(lines[0]).To(ContainSubstring("project.json"), "the existing project_config_invalid message must still name the path")
			Expect(lines[1]).To(Equal("typescript_compiler_missing: also failing; run coach codesignal --baseline --check-project --project-language typescript --project-config project.json once the policy above is authored and committed"), "AC-SET-13 requires reporting the compiler gap that loadProjectConfig's short circuit would otherwise mask, even though project_shape is also failing here")
			Expect(lines[2]).To(Equal("coach codesignal --baseline --suggest-project-config --project-language typescript"), "AC-SET-9's appended command must never combine --suggest-project-config with --project-config (validateSuggestProjectConfigFlags rejects that combination)")
		})
	})

	When("a --baseline scan's explicit --project-config names a policy that was never committed, while the repository is genuinely TypeScript-shaped and has no installed TypeScript compiler either (AC-SET-13)", func() {
		DescribeTable("reports the existing policy message unchanged, additionally reports the masked compiler gap, and still offers only guided policy authoring -- never --prepare-compiler -- as the interactive next step, regardless of --format (AC-9)",
			func(formatArg string) {
				repo := newTempGitRepo()
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
				commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
				// project.json is deliberately never committed, and no TypeScript
				// compiler is installed: the policy check and the compiler check
				// both fail at the same revision, but loadProjectConfig's short
				// circuit (prepareProjectAnalysis) means only the policy failure
				// would ever reach classifyAnalysisError without AC-SET-13's
				// readiness-aware wrapping.

				path := pathWithStubNode("v24.9.9")

				stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", formatArg)

				Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
				Expect(stdout).To(BeEmpty())

				lines := stderrLines(stderr)
				Expect(lines).To(HaveLen(3), "stderr: %s", stderr)
				Expect(lines[0]).To(ContainSubstring("project.json"), "the existing project_config_invalid message must still name the path")
				Expect(lines[1]).To(Equal("typescript_compiler_missing: also failing; run coach codesignal --baseline --check-project --project-language typescript --project-config project.json once the policy above is authored and committed"), "AC-SET-13 requires reporting the compiler gap that loadProjectConfig's short circuit would otherwise mask")
				Expect(lines[2]).To(Equal("coach codesignal --baseline --suggest-project-config --project-language typescript"), "the only offered interactive action must still be guided policy authoring, never --prepare-compiler, until a policy is reviewed and committed")
			},
			Entry("--format=json", "--format=json"),
			Entry("--format=text", "--format=text"),
		)
	})

	When("a --baseline scan's explicit --project-config names a policy that was never committed, and the repository's only package.json is nested under an unvalidated root (a genuine monorepo shape)", func() {
		It("still reports the masked compiler gap, since checks.compiler runs unconditionally on project_shape and readiness's own gaps[] already names it", func() {
			repo := newTempGitRepo()
			commitFile(repo, "packages/app/package.json", `{"name":"app","version":"1.0.0"}`+"\n")
			commitFile(repo, "packages/app/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			// project.json is deliberately never committed, so roots is never
			// resolved: project_shape can only check the repository root
			// (checkProjectShape's policyPassed-gated walk never runs), which has
			// no package.json, so it fails (unsupported_repository_shape) even
			// though a real TypeScript package exists lower in the tree. The
			// compiler check still runs independently and still fails
			// (typescript_compiler_missing); withholding this line on
			// project_shape's result would drop a gap readiness itself reports.

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(3), "stderr: %s", stderr)
			Expect(lines[0]).To(ContainSubstring("project.json"), "the existing project_config_invalid message must still name the path")
			Expect(lines[1]).To(Equal("typescript_compiler_missing: also failing; run coach codesignal --baseline --check-project --project-language typescript --project-config project.json once the policy above is authored and committed"), "a nested, unvalidated package.json must not suppress a compiler gap readiness itself reports")
			Expect(lines[2]).To(Equal("coach codesignal --baseline --suggest-project-config --project-language typescript"))
		})
	})
})

// AvailableSetupChoices' combined-menu composition (AC-11) is otherwise
// exercised only over hand-constructed ReadinessResult values
// (project_ts_setup_choice_acceptance_test.go) or over a real readiness
// result for a single mise scope alone (project_ts_setup_choice_acceptance_test.go:417-458).
// This spec drives the whole pipeline -- a real CheckProjectReadiness result
// whose checks.package_manager passes and whose project mise.toml already
// pins an exact supported version -- to prove project_package and
// project_mise actually compose in one menu from real readiness, which is
// what a combined setup offer (#330 Task 5) needs. Confirming and executing
// exactly one of the two choices is T5's to verify: no production caller
// wires AvailableSetupChoices' output through a single-use confirmation yet.
var _ = Describe("codesignalcli.AvailableSetupChoices composes project_package and project_mise from one real CheckProjectReadiness result (AC-11)", func() {
	It("offers both SetupChoiceProjectPackage and SetupChoiceProjectMise when the project package manager passes and the project mise scope already pins an installable version", func() {
		nodeDir := writeStubNodeScript("v24.9.9")
		npmDir := writeStubPackageManagerScript("npm", "11.0.0")
		miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
		path := nodeDir + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
		GinkgoT().Setenv("PATH", path)
		GinkgoT().Setenv("HOME", os.Getenv("HOME"))

		repo := newTempGitRepo()
		commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
		commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", codesignalcli.SupportedTypescriptVersions[0]))
		head := commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
		writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))

		readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(readiness.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessFail), "the fixture must genuinely need setup, or the menu assertion below proves nothing")
		Expect(readiness.Checks.PackageManager.State).To(Equal(codesignalcli.ReadinessPass), "detail=%s", readiness.Checks.PackageManager.Detail)

		menu := codesignalcli.AvailableSetupChoices(*readiness)

		Expect(choiceKinds(menu.Choices)).To(ContainElement(codesignalcli.SetupChoiceProjectPackage), "an installable manifest declaration plus a passing package-manager check must offer project_package")
		Expect(choiceKinds(menu.Choices)).To(ContainElement(codesignalcli.SetupChoiceProjectMise), "an exact, supported project mise.toml pin must offer project_mise")
	})
})

// stderrLines splits stderr on newlines after trimming exactly one trailing
// newline, so each fmt.Fprintln call's line is a distinct element rather
// than leaving a spurious empty trailing element.
func stderrLines(stderr []byte) []string {
	return strings.Split(strings.TrimRight(string(stderr), "\n"), "\n")
}
