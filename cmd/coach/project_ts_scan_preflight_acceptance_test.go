package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// D3 fixes the existing single-line remediation classifyAnalysisError has
// always printed for a real (non --check-project, non --prepare-compiler)
// scan's CompilerUnresolvedError/ProjectConfigError, appending its own
// supported interactive-setup command while a controlling terminal is
// unavailable, without changing that first line or the exit code. These
// specs exercise the appended line only.
var _ = Describe("coach codesignal (real scan): appended interactive-setup remediation (AC-SET-9)", func() {
	When("a --baseline scan's explicit --project-config resolves no TypeScript compiler, a verified mise scope can install it, and no controlling terminal is available", func() {
		It("keeps the existing --check-project remediation line unchanged, appends the --prepare-compiler remediation, leaves stdout empty, and exits 2", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))

			path, _ := pathWithStatefulStubNodeAndMise("v24.9.9", codesignalcli.SupportedTypescriptVersions[0])

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "the existing D3 remediation line must not change")
			Expect(lines[1]).To(Equal("on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"), "AC-SET-9's appended command must be the interactive setup flow that resolves this exact gap")
		})
	})

	When("a --baseline scan's compiler gap is resolvable only by the project package manager, and no controlling terminal is available", func() {
		It("keeps the existing --check-project remediation, and appends rerunning this same scan on a terminal (R2) rather than --prepare-compiler, since --prepare-compiler runs mise scopes only and would exit 0 having set nothing up", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", codesignalcli.SupportedTypescriptVersions[0]))
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			nodeDir := writeStubNodeScript("v24.9.9")
			npmDir := writeStubPackageManagerScript("npm", "11.0.0")
			path := nodeDir + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + pathExcludingToolchain()

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "--prepare-compiler discards the project_package choice this repository's menu offers and would exit 0 reporting nothing to set up, which a piped CI operator cannot tell apart from success -- but rerunning this same scan on a terminal opens the combined setup offer that can actually run project_package (R2); stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(lines[1]).To(Equal("on a terminal: coach codesignal --baseline --project-config project.json --project-language typescript -- offers project-package setup"))
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

	When("a --baseline scan's compiler gap offers no genuinely executable compiler-setup choice (no package manager and no mise present), and no controlling terminal is available (O2)", func() {
		It("keeps the existing --check-project remediation as the only stderr line, never naming --prepare-compiler for a menu that would only open to report nothing to set up", func() {
			repo := noSupportedCompilerRepo()
			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(1), "a gap whose readiness snapshot offers no genuinely executable compiler-setup choice must never print a --prepare-compiler command that only opens to report it had nothing to do; stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
		})
	})

	When("a --base diff scan hits the same unresolved-compiler gap", func() {
		It("applies the identical appended remediation, since classifyAnalysisError is shared by both scan modes", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			baseSHA := commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))

			path, _ := pathWithStatefulStubNodeAndMise("v24.9.9", codesignalcli.SupportedTypescriptVersions[0])

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(path), "codesignal", "--base", baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(lines[1]).To(Equal("on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"))
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
			// checks.project_shape itself reports not_checked rather than
			// unsupported_repository_shape (R1): without a validated policy it
			// has no basis to tell a genuinely unsupported shape apart from an
			// as-yet-uncommitted monorepo policy, so it is absent from gaps[]
			// entirely rather than named alongside the other two.

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines[0]).To(ContainSubstring("project.json"), "the existing project_config_invalid message must still name the path")
			Expect(alsoFailingGapCodes(lines)).To(Equal([]string{"typescript_compiler_missing", "package_manager_version_unverifiable"}), "AC-SET-13 requires reporting every gap loadProjectConfig's short circuit would otherwise mask -- not only the compiler check -- in readiness's own frozen order; stderr: %s", stderr)
			Expect(lines[len(lines)-1]).To(Equal("on a terminal: coach codesignal --baseline --suggest-project-config --project-language typescript -- guided authoring requires a controlling terminal; without one, draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>"), "AC-SET-9's appended command must never combine --suggest-project-config with --project-config (validateSuggestProjectConfigFlags rejects that combination)")
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
				Expect(lines).To(HaveLen(4), "stderr: %s", stderr)
				Expect(lines[0]).To(ContainSubstring("project.json"), "the existing project_config_invalid message must still name the path")
				Expect(alsoFailingGapCodes(lines)).To(Equal([]string{"typescript_compiler_missing", "package_manager_version_unverifiable"}), "AC-SET-13 requires reporting every gap loadProjectConfig's short circuit would otherwise mask; stderr: %s", stderr)
				Expect(lines[len(lines)-1]).To(Equal("on a terminal: coach codesignal --baseline --suggest-project-config --project-language typescript -- guided authoring requires a controlling terminal; without one, draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>"), "the only offered interactive action must still be guided policy authoring, never --prepare-compiler, until a policy is reviewed and committed")
			},
			Entry("--format=json", "--format=json"),
			Entry("--format=text", "--format=text"),
		)
	})

	When("a --baseline scan's explicit --project-config names a policy that was never committed, and the repository's only package.json is nested under an unvalidated root (a genuine monorepo shape)", func() {
		It("still reports the masked compiler gap, since checks.compiler runs unconditionally on project_shape and readiness's own gaps[] already names it, and never reports unsupported_repository_shape for a shape checkProjectShape had no basis to condemn (R1)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "packages/app/package.json", `{"name":"app","version":"1.0.0"}`+"\n")
			commitFile(repo, "packages/app/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			// project.json is deliberately never committed, so roots is never
			// resolved: project_shape can only check the repository root
			// (checkProjectShape's policyPassed-gated walk never runs), which has
			// no package.json. Since R1, that reports not_checked rather than
			// unsupported_repository_shape: without the policy's roots, this
			// check cannot tell a genuinely unsupported shape apart from a real
			// TypeScript package that simply lives lower in the tree, exactly
			// as it does here. The compiler check still runs independently and
			// still fails (typescript_compiler_missing), which is reported.

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines[0]).To(ContainSubstring("project.json"), "the existing project_config_invalid message must still name the path")
			Expect(lines).To(HaveLen(4), "stderr: %s", stderr)
			Expect(alsoFailingGapCodes(lines)).To(Equal([]string{"typescript_compiler_missing", "package_manager_version_unverifiable"}), "a nested, unvalidated package.json must not suppress a compiler gap readiness itself reports, and unsupported_repository_shape must never appear once checkProjectShape reports not_checked; stderr: %s", stderr)
			Expect(lines[len(lines)-1]).To(Equal("on a terminal: coach codesignal --baseline --suggest-project-config --project-language typescript -- guided authoring requires a controlling terminal; without one, draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>"))
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
// what a combined setup offer needs. Confirming and executing exactly one of
// the two choices is a separate concern: no production caller wires
// AvailableSetupChoices' output through a single-use confirmation yet.
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

// PrepareTSRuntime's resolveHostNode (project_ts_runtime.go) maps only
// errHostNodeNotFound and errHostNodeMajorDisallowed into an actionable
// CompilerUnresolvedError gap code; any other probe failure never reaches
// classifyAnalysisError's remediation branch. So a node_unverifiable gap is
// unreachable from this real-scan boundary by design (tracked at the scan
// boundary in #355) and is not asserted here.
var _ = Describe("codesignalcli.CheckProjectReadiness never gates or warns on a selected supported-set Node release (AC-SET-10)", func() {
	It("has more than one supported Node major, so the table below cannot silently degrade to exercising just one", func() {
		Expect(len(codesignalcli.SupportedNodeMajors)).To(BeNumerically(">", 1), "codesignalcli.SupportedNodeMajors=%v", codesignalcli.SupportedNodeMajors)
	})

	tableArgs := []any{func(major int) {
		version := fmt.Sprintf("v%d.0.0", major)
		path := pathWithStubNode(version)
		GinkgoT().Setenv("PATH", path)
		GinkgoT().Setenv("HOME", os.Getenv("HOME"))

		repo := newTempGitRepo()
		head := commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

		readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
		Expect(err).NotTo(HaveOccurred())

		Expect(readiness.Checks.Node.State).To(Equal(codesignalcli.ReadinessPass), "detail=%s", readiness.Checks.Node.Detail)
		Expect(readiness.Checks.Node.Code).To(BeEmpty())
		Expect(readiness.Checks.Runtime.State).To(Equal(codesignalcli.ReadinessPass), "detail=%s", readiness.Checks.Runtime.Detail)
		Expect(readiness.Checks.Runtime.Code).To(BeEmpty())

		for _, gap := range readiness.Gaps {
			Expect(gap.Code).NotTo(HavePrefix("node_"), "a supported-set Node release must never contribute a node_* gap, got %q", gap.Code)
		}
		for _, warning := range readiness.Warnings {
			Expect(warning.Code).NotTo(HavePrefix("node_"), "a supported-set Node release must never warn, got %q", warning.Code)
		}
		for _, action := range readiness.NextActions {
			Expect(action.RuntimeKind).NotTo(Equal("node"), "a supported-set Node release must never contribute a runtime next action, got kind=%q", action.Kind)
		}

		_, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
		Expect(exitCode).To(Equal(2), "stdout/stderr: %s", stderr)
		Expect(stderrLines(stderr)[0]).To(HavePrefix("typescript_compiler_missing:"), "the scan must fail on the later missing-compiler boundary, not on Node; stderr: %s", stderr)
	}}
	for _, major := range codesignalcli.SupportedNodeMajors {
		tableArgs = append(tableArgs, Entry(fmt.Sprintf("Node major %d", major), major))
	}

	DescribeTable("passes node/runtime with no gap, no warning, and no runtime next action, and a real scan proceeds past the Node boundary to fail only on the later missing-compiler gap", tableArgs...)
})

// R2: readinessFromGapChecks derives Runtime's and Compiler's next actions
// independently of one another (project_readiness_aggregate.go), so a
// readiness snapshot can carry both a non-executable install_supported_runtime
// action (Node genuinely missing) and a genuinely executable prepare_compiler
// action (the repository's mise scope independently pins a supported
// version) at once. The interim --prepare-compiler dispatch must refuse for
// the runtime-boundary gap exactly as the real scan's own gate does
// (gapCodeIsExecutablePrepareCompiler), never opening the compiler-setup
// menu or invoking `mise install` while host Node itself is unreachable.
var _ = Describe("coach's interim standalone prepare_compiler dispatch refuses for a runtime-boundary gap, sharing the scan path's own gate (R2)", func() {
	When("host Node is genuinely unreachable while the repository's mise scope independently declares an installable TypeScript version", func() {
		It("refuses naming node_missing, never opens the interactive compiler-setup menu, and never invokes mise install", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
			requireNodeUnreachable(path)
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(transcript).NotTo(ContainSubstring("TypeScript compiler setup:"), "the interactive compiler-setup menu must never open for a runtime-boundary gap, even though the repository's mise scope independently offers an installable version; transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("node_missing"), "the refusal must name the runtime-boundary gap that actually blocks; transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "withholding the offer must never invoke `mise install`")
		})
	})
})

// stderrLines splits stderr on newlines after trimming exactly one trailing
// newline, so each fmt.Fprintln call's line is a distinct element rather
// than leaving a spurious empty trailing element.
func stderrLines(stderr []byte) []string {
	return strings.Split(strings.TrimRight(string(stderr), "\n"), "\n")
}

// alsoFailingGapCodes extracts the gap codes from AC-SET-13's "also failing"
// lines, so a spec can assert which gaps were reported and in what order
// without restating the shared remediation sentence for each one.
var _ = Describe("coach codesignal: optional preparation after a successful scan (SA-280-046)", func() {
	When("an injected optional offer declines", func() {
		It("still renders the CodeSignal report", func() {
			original := runOptionalScanPreparation
			called := false
			runOptionalScanPreparation = func(dir string, f codesignalFlags, stdout, stderr *os.File) optionalPreparationResult {
				called = true
				return optionalPreparationResult{Declined: true}
			}
			DeferCleanup(func() { runOptionalScanPreparation = original })

			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			outRead, outWrite, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			errRead, errWrite, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())

			f := codesignalFlags{baseline: true, format: "json"}
			exitCode := runCodesignalScan(repo, f, outWrite, errWrite, newScanOfferBudget())
			Expect(outWrite.Close()).To(Succeed())
			Expect(errWrite.Close()).To(Succeed())
			stdout, readErr := io.ReadAll(outRead)
			Expect(readErr).NotTo(HaveOccurred())
			stderr, readErr := io.ReadAll(errRead)
			Expect(readErr).NotTo(HaveOccurred())

			Expect(called).To(BeTrue(), "the optional-preparation seam must run after a successful scan so Task 13 can attach declaration alignment; stderr: %s", stderr)
			Expect(exitCode).To(Equal(0), "declining an optional action must not abort the scan; stdout: %s stderr: %s", stdout, stderr)
			var report map[string]any
			Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout: %s", stdout)
			Expect(report).To(HaveKey("coverage"))
		})
	})

	When("an injected optional offer cancels", func() {
		It("exits 2 with no report", func() {
			original := runOptionalScanPreparation
			runOptionalScanPreparation = func(dir string, f codesignalFlags, stdout, stderr *os.File) optionalPreparationResult {
				return optionalPreparationResult{Cancelled: true}
			}
			DeferCleanup(func() { runOptionalScanPreparation = original })

			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			outRead, outWrite, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			errRead, errWrite, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())

			f := codesignalFlags{baseline: true, format: "json"}
			exitCode := runCodesignalScan(repo, f, outWrite, errWrite, newScanOfferBudget())
			Expect(outWrite.Close()).To(Succeed())
			Expect(errWrite.Close()).To(Succeed())
			stdout, readErr := io.ReadAll(outRead)
			Expect(readErr).NotTo(HaveOccurred())
			_, readErr = io.ReadAll(errRead)
			Expect(readErr).NotTo(HaveOccurred())

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
		})
	})
})

func alsoFailingGapCodes(lines []string) []string {
	var codes []string
	for _, line := range lines {
		code, rest, found := strings.Cut(line, ": also failing, ")
		if !found || rest == "" {
			continue
		}
		codes = append(codes, code)
	}
	return codes
}
