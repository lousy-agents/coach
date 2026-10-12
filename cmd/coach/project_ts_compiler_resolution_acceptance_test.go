package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

var _ = Describe("coach codesignal --project-language typescript against the private embedded analyzer and a confined, host-resolved compiler (coach#326 Task 3)", func() {
	When("no exact TypeScript compiler is genuinely locatable anywhere in the analyzed repository", func() {
		It("exits 2 with empty stdout and one stderr line naming the gap code and the --check-project invocation in JSON mode", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "no package manager and no mise scope are declared, so the readiness menu offers nothing installable and O2 withholds the appended --prepare-compiler command")
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
		})

		It("exits 2 with empty stdout and one stderr line naming the gap code and the --check-project invocation in text mode", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "no package manager and no mise scope are declared, so the readiness menu offers nothing installable and O2 withholds the appended --prepare-compiler command")
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
		})

		It("executes the printed fit-check invocation unchanged and exits 0 with a readiness document", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			path := pathWithStubNode("v24.9.9")

			_, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)

			args := codesignalArgsFromRemediationLine(stderr)
			stdout, runStderr, runExit := runCoachCheckProjectEnv(repo, path, args...)
			Expect(runExit).To(Equal(0), "printed invocation must pass the flag validator unchanged; stderr: %s stdout: %s", runStderr, stdout)
			Expect(string(stdout)).To(ContainSubstring("status:"), "printed invocation must produce a readiness document, got %q", stdout)
			Expect(string(stdout)).To(ContainSubstring("compiler: fail (typescript_compiler_missing)"), "readiness document must name the same gap the scan printed, got %q", stdout)
		})
	})

	When("host Node is missing from PATH even though a supported compiler is installed", func() {
		It("exits 2 with empty stdout and one stderr line naming node_missing and the --check-project invocation in JSON mode", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithoutNode()
			requireNodeUnreachable(path)

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("node_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
		})

		It("exits 2 with empty stdout and one stderr line naming node_missing and the --check-project invocation in text mode", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithoutNode()
			requireNodeUnreachable(path)

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("node_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
		})
	})

	When("the installed compiler is an exact 5.x version", func() {
		It("exits 2 with empty stdout and one stderr line naming typescript_version_mismatch, each root's finding, and the --check-project invocation", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "5.4.0")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_mismatch (.@5.4.0): run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "no package manager and no mise scope are declared, so the readiness menu offers nothing installable and O2 withholds the appended --prepare-compiler command")
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
			Expect(strings.Count(strings.TrimSpace(string(stderr)), "\n")).To(Equal(0), "just the D3 refusal line: O2 withholds the appended --prepare-compiler command when nothing is genuinely offered, got %q", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(repo), "the refusal must never print a filesystem path, got %q", stderr)
		})
	})

	When("two selected roots pin disagreeing exact typescript versions", func() {
		It("exits 2 with empty stdout and one stderr line naming typescript_version_conflict, each root's finding, and the --check-project invocation", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["apps/web","apps/api"],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`+"\n")
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")
			writeInstalledTypescriptUnder(repo, "apps/api", "5.4.0")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_conflict (apps/web@7.0.2,apps/api@5.4.0): run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "no package manager and no mise scope are declared, so the readiness menu offers nothing installable and O2 withholds the appended --prepare-compiler command")
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
			Expect(strings.Count(strings.TrimSpace(string(stderr)), "\n")).To(Equal(0), "just the D3 refusal line: O2 withholds the appended --prepare-compiler command when nothing is genuinely offered, got %q", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(repo), "the refusal must never print a filesystem path, got %q", stderr)
		})
	})

	When("the worktree mise.toml carries an env exec template that would write a sentinel", func() {
		It("produces no side effect during a scan because mise probes use a neutral working directory", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			sentinel := filepath.Join(repo, "mise-exec-side-effect")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[env]\nSIDE_EFFECT = \"{{ exec(command='touch %s') }}\"\n", sentinel))

			path, miseDir := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			_, stderr, _ := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			_, statErr := os.Stat(sentinel)
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "mise exec template must not run during a scan; stderr=%s", stderr)
			for _, cwd := range readStubMiseCwds(miseDir) {
				Expect(cwd).NotTo(Equal(repo), "mise probes must not run with the analyzed repository as cwd, got %q", cwd)
			}
		})
	})

	When("the approved compiler's native platform package is missing", func() {
		It("exits 2 with empty stdout and the D3 typescript_compiler_missing stderr line that does not name the native package", func() {
			repo := newTempGitRepo()
			commitNativePackageGapFixture(repo)

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			expectTypescriptCompilerMissingScan(stdout, stderr, exitCode)
			Expect(tstoolchain.NativeTypescriptPackageName()).To(Equal(nativeTypescriptPackageLookupName()))
		})

		It("executes the printed fit-check invocation and reports typescript_compiler_missing naming the native package with the compiler found_version", func() {
			repo := newTempGitRepo()
			commitNativePackageGapFixture(repo)
			packageName := nativeTypescriptPackageLookupName()

			path := pathWithStubNode("v24.9.9")

			_, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)

			args := codesignalArgsFromRemediationLine(stderr)
			stdout, runStderr, runExit := runCoachCheckProjectEnv(repo, path, args...)
			Expect(runExit).To(Equal(0), "printed invocation must pass the flag validator unchanged; stderr: %s stdout: %s", runStderr, stdout)
			Expect(string(stdout)).To(ContainSubstring("compiler: fail (typescript_compiler_missing)"), "readiness document must name the same gap the scan printed, got %q", stdout)
			Expect(string(stdout)).To(ContainSubstring(packageName), "readiness text must name the native package %s, got %q", packageName, stdout)
			Expect(string(stdout)).NotTo(ContainSubstring(repo+string(os.PathSeparator)), "readiness remediation must not print a filesystem path, got %q", stdout)

			jsonStdout, jsonStderr, jsonExit := runCoachCheckProjectEnv(repo, path, append(args, "--format", "json")...)
			Expect(jsonExit).To(Equal(0), "stderr: %s stdout: %s", jsonStderr, jsonStdout)
			Expect(string(jsonStdout)).To(ContainSubstring(packageName), "JSON remediation must name the native package %s, got %q", packageName, jsonStdout)
			Expect(string(jsonStdout)).NotTo(ContainSubstring(repo+string(os.PathSeparator)), "JSON remediation must not print a filesystem path, got %q", jsonStdout)
			var doc readinessResultDoc
			Expect(json.Unmarshal(jsonStdout, &doc)).To(Succeed(), "stdout: %s", jsonStdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.FoundVersion).To(Equal("7.0.2"), "found_version must be the compiler's probed version, not the native package, got %q stdout=%s", doc.Checks.Compiler.FoundVersion, jsonStdout)
		})
	})

	When("the approved compiler's native platform package is version-divergent", func() {
		It("exits 2 with empty stdout and typescript_compiler_missing, not typescript_version_mismatch, naming the native package with the compiler found_version", func() {
			repo := newTempGitRepo()
			commitNativePackageGapFixture(repo)
			writeInstalledNativeTypescript(repo, "5.0.0")
			packageName := nativeTypescriptPackageLookupName()

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			expectTypescriptCompilerMissingScan(stdout, stderr, exitCode)
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_version_mismatch"))
			Expect(tstoolchain.NativeTypescriptPackageName()).To(Equal(packageName))

			args := codesignalArgsFromRemediationLine(stderr)
			checkStdout, checkStderr, checkExit := runCoachCheckProjectEnv(repo, path, args...)
			Expect(checkExit).To(Equal(0), "stderr: %s stdout: %s", checkStderr, checkStdout)
			Expect(string(checkStdout)).To(ContainSubstring("compiler: fail (typescript_compiler_missing)"))
			Expect(string(checkStdout)).NotTo(ContainSubstring("typescript_version_mismatch"))
			Expect(string(checkStdout)).To(ContainSubstring(packageName), "readiness text must name the native package %s, got %q", packageName, checkStdout)
			Expect(string(checkStdout)).NotTo(ContainSubstring(repo+string(os.PathSeparator)), "readiness remediation must not print a filesystem path, got %q", checkStdout)

			jsonStdout, jsonStderr, jsonExit := runCoachCheckProjectEnv(repo, path, append(args, "--format", "json")...)
			Expect(jsonExit).To(Equal(0), "stderr: %s stdout: %s", jsonStderr, jsonStdout)
			Expect(string(jsonStdout)).To(ContainSubstring(packageName), "JSON remediation must name the native package %s, got %q", packageName, jsonStdout)
			Expect(string(jsonStdout)).NotTo(ContainSubstring(repo+string(os.PathSeparator)), "JSON remediation must not print a filesystem path, got %q", jsonStdout)
			var doc readinessResultDoc
			Expect(json.Unmarshal(jsonStdout, &doc)).To(Succeed(), "stdout: %s", jsonStdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"), "version-divergent native package is not typescript_version_mismatch, got code=%s", doc.Checks.Compiler.Code)
			Expect(doc.Checks.Compiler.FoundVersion).To(Equal("7.0.2"), "found_version must be the compiler's probed version, not the native package, got %q stdout=%s", doc.Checks.Compiler.FoundVersion, jsonStdout)
		})
	})

	When("the resolved native platform package is present and version-equal but unloadable", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("reports a qualified incomplete report at exit 0, not exit 2, without leaking the compiler's absolute path", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			compilerDir := installRealTypescriptCompiler(repo, true)
			nativeName := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
			nativeDest := filepath.Join(repo, "node_modules", "@typescript", nativeName)
			Expect(os.Remove(filepath.Join(nativeDest, "lib", "tsc"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(BeEmpty(), "a degraded compiler must never fabricate the layer violation it never actually reported")
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse())

			message := backendUnavailableDiagnosticMessage(report)
			Expect(message).To(Or(
				ContainSubstring("failed to load resolved TypeScript compiler module"),
				ContainSubstring("native TypeScript executable is missing"),
				ContainSubstring("failed to start ts sidecar analysis backend"),
			), "expected the missing-native-executable failure to surface, got: %s", message)

			Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
			Expect(string(stdout)).NotTo(ContainSubstring(nativeDest), "the native compiler executable path must never appear in the serialized report")
			Expect(message).NotTo(ContainSubstring(repo), "diagnostic must not contain the repository path")
			Expect(message).NotTo(ContainSubstring("coach-ts-analyzer-"), "diagnostic must not contain the analyzer temp-directory prefix")
			Expect(message).NotTo(ContainSubstring("file://"))
			Expect(message).NotTo(ContainSubstring("node:internal"))
		})

		It("renders a qualified verdict, not the unqualified clean-run sentence", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			installRealTypescriptCompiler(repo, true)
			nativeName := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
			nativeDest := filepath.Join(repo, "node_modules", "@typescript", nativeName)
			Expect(os.Remove(filepath.Join(nativeDest, "lib", "tsc"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			text := string(stdout)
			Expect(text).NotTo(ContainSubstring("No active CodeSignal findings.\n"), "a degraded compiler must not render the exact unqualified clean-run verdict")
			Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
			Expect(text).To(ContainSubstring("project analysis did not complete"))
		})
	})

	When("the resolved compiler is missing a required unstable API export", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("reports the missing-unstable-export module-resolution failure specifically, not the pre-#326 generic sidecar-unavailable degrade or a crash", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			compilerDir := installRealTypescriptCompiler(repo, true)
			breakCompilerExportSubpath(compilerDir, "./unstable/fs")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(BeEmpty(), "a degraded compiler must never fabricate the layer violation it never actually reported")
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse())

			message := backendUnavailableDiagnosticMessage(report)
			Expect(message).To(ContainSubstring(`does not declare a "./unstable/fs" export`), "expected the missing-unstable-API failure to surface, got: %s", message)

			Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
			Expect(message).NotTo(ContainSubstring(repo), "diagnostic must not contain the repository path")
			Expect(message).NotTo(ContainSubstring("coach-ts-analyzer-"), "diagnostic must not contain the analyzer temp-directory prefix")
			Expect(message).NotTo(ContainSubstring("file://"))
			Expect(message).NotTo(ContainSubstring("node:internal"))
		})
	})

	When("a declared unstable export subpath resolves to a file that does not exist (a broken exports entry, not a missing one)", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("reports a broken-export-target load failure, distinct from a missing export entry, when the compiler's own package.json points at a file that doesn't exist", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			compilerDir := installRealTypescriptCompiler(repo, true)
			repointCompilerExportSubpath(compilerDir, "./unstable/fs", "./lib/does-not-exist.js")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(BeEmpty(), "a degraded compiler must never fabricate the layer violation it never actually reported")
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse())

			message := backendUnavailableDiagnosticMessage(report)
			Expect(message).To(ContainSubstring("failed to load typescript/unstable/fs from the resolved TypeScript compiler"), "expected the broken-export-target failure to surface, got: %s", message)

			Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
			Expect(message).NotTo(ContainSubstring(repo), "diagnostic must not contain the repository path")
			Expect(message).NotTo(ContainSubstring("coach-ts-analyzer-"), "diagnostic must not contain the analyzer temp-directory prefix")
			Expect(message).NotTo(ContainSubstring("file://"))
			Expect(message).NotTo(ContainSubstring("node:internal"))
		})
	})

	When("--project-config is invalid at the selected revision and --project-language is typescript", func() {
		It("still exits 2 with project_config_invalid, never reaching backend dispatch", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "project.json", "not valid json")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stdout).To(BeEmpty(), "never reaching backend dispatch means nothing is written to stdout")
			Expect(string(stderr)).NotTo(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project.json"))
		})
	})

	When("host Node major is 25 at runtime preparation", func() {
		It("exits 2 with empty stdout and node_unsupported, matching the same stub's --check-project gap code", func() {
			repo := newTempGitRepo()
			commitNativePackageGapFixture(repo)
			writeInstalledNativeTypescript(repo, "7.0.2")

			path := pathWithStubNode("v25.0.0")
			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("node_unsupported: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("node_missing"))
			Expect(string(stderr)).NotTo(ContainSubstring("node_below_minimum"))
			Expect(string(stderr)).NotTo(ContainSubstring("25"))
			Expect(string(stderr)).NotTo(ContainSubstring("{24, 26}"))
		})

		It("reports the same stub under --check-project as node_unsupported and does not emit node_untested", func() {
			repo := newTempGitRepo()
			commitNativePackageGapFixture(repo)
			writeInstalledNativeTypescript(repo, "7.0.2")

			path := pathWithStubNode("v25.0.0")

			jsonStdout, jsonStderr, jsonExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(jsonExit).To(Equal(0), "stderr: %s stdout: %s", jsonStderr, jsonStdout)

			var doc readinessResultDoc
			Expect(json.Unmarshal(jsonStdout, &doc)).To(Succeed(), "stdout: %s", jsonStdout)
			Expect(doc.Status).To(Equal("needs_prerequisite"))
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unsupported"))
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unsupported"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s stdout: %s", textStderr, textStdout)
			Expect(string(textStdout)).To(ContainSubstring("node_unsupported"))
			Expect(string(textStdout)).NotTo(ContainSubstring("node_untested"))
			Expect(string(textStdout)).NotTo(ContainSubstring("node_missing"))
		})
	})

	When("a wrong-version native package canary sits on the omit-tsserverPath walk", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("completes a real scan using the approved native path and never runs the canary", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			installRealTypescriptCompiler(repo, true)

			markerDir, err := os.MkdirTemp("", "coach-native-canary-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, markerDir)
			approvedMarker := filepath.Join(markerDir, "approved")
			canaryMarker := filepath.Join(markerDir, "canary")

			nativeName := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
			approvedExe := filepath.Join(repo, "node_modules", "@typescript", nativeName, "lib", "tsc")
			wrapExecutableWithMarker(approvedExe, approvedMarker)

			canaryExe := filepath.Join(repo, "node_modules", "typescript", "node_modules", "@typescript", nativeName, "lib", "tsc")
			plantCanaryExecutable(canaryExe, canaryMarker)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(HaveLen(1))

			_, approvedErr := os.Stat(approvedMarker)
			Expect(approvedErr).NotTo(HaveOccurred(), "approved native path wrapper must run")
			_, canaryErr := os.Stat(canaryMarker)
			Expect(os.IsNotExist(canaryErr)).To(BeTrue(), "canary on the omit-tsserverPath walk must not run")
		})
	})
})
