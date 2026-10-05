package main

import (
	"encoding/json"
	"fmt"

	"os"
	"path/filepath"
	"runtime"

	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

const tsProjectTSConfigJSON = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`

const tsRealDbFile = "export const Name = 'db';\n"

const tsRealHandlersImportingDB = "import { Name } from \"../db/d\";\n\nexport function use(): string {\n  return Name;\n}\n"

// tsRealHandlersWithoutImport is the negative-control counterpart of
// tsRealHandlersImportingDB, used by no_findings_verdict_acceptance_test.go
// to build a "clean" fixture with no forbidden edge at all.
const tsRealHandlersWithoutImport = "export function use(): string {\n  return 'no import here';\n}\n"

const analyzerChildArgMarker = "--compiler-module="

type recordingProxyListener struct {
	addr string
	mu   sync.Mutex
	hits []string
	stop func()
}

type analyzerEnvironSampler struct {
	stop  chan struct{}
	done  chan struct{}
	mu    sync.Mutex
	byPID map[int]string
	seen  map[string]struct{}
}

var _ = Describe("coach codesignal --project-language typescript against the private embedded analyzer and a confined, host-resolved compiler (coach#326 Task 3)", func() {
	When("the analyzed repository vendors no js/semantics analyzer anywhere and declares a real, exactly-matching installed TypeScript compiler", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_54()
		})

		It("completes the analysis via the private materialized analyzer and confined compiler, reporting the real layer violation without leaking the compiler's absolute path", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			compilerDir := installRealTypescriptCompiler(repo, true)

			_, statErr := os.Stat(filepath.Join(repo, "js", "semantics"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "expected no vendored js/semantics directory anywhere in the analyzed repository")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)

			Expect(report.ProjectChanges).To(HaveLen(1))
			change := report.ProjectChanges[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.Kind).To(Equal("architecture.layer_violation"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("language", "typescript"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
			Expect(change.PrimaryAnchor.Path).To(Equal("pkg/handlers/h.ts"))

			Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
		})
	})

	When("the parent process spies via NODE_OPTIONS=--require and HTTP(S)_PROXY points at a recording listener", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_91()
		})

		It("completes analysis without the spy running, without the analyzer child inheriting HTTP(S)_PROXY, and without the listener accepting a connection", func() {
			body_projectTsBackendAcceptanceTest_completesAnalysisWithoutTheSpyRunningWithoutTheA_97()
		})
	})

	When("an uncommitted worktree edit would introduce a forbidden TypeScript layer edge that HEAD does not have", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_144()
		})

		It("analyzes only the Git snapshot and does not report the worktree-only violation", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)
			Expect(os.WriteFile(filepath.Join(repo, "pkg/handlers/h.ts"), []byte(tsRealHandlersImportingDB), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(BeEmpty(), "an uncommitted forbidden import must never become analysis input")
		})
	})

	When("diff mode introduces a forbidden TypeScript layer edge that did not exist at base", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_172()
		})

		It("builds distinct head and base project models sharing one PrepareTSRuntime call and classifies the ProjectChange as lifecycle introduced", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))

			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "head-side sidecar call must have succeeded")
		})
	})

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
			body_projectTsBackendAcceptanceTest_producesNoSideEffectDuringAScanBecauseMiseProbes_337()
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
			body_projectTsBackendAcceptanceTest_431()
		})

		It("reports a qualified incomplete report at exit 0, not exit 2, without leaking the compiler's absolute path", func() {
			body_projectTsBackendAcceptanceTest_reportsAQualifiedIncompleteReportAtExit0NotExit2_437()
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
			body_projectTsBackendAcceptanceTest_499()
		})

		It("reports the missing-unstable-export module-resolution failure specifically, not the pre-#326 generic sidecar-unavailable degrade or a crash", func() {
			body_projectTsBackendAcceptanceTest_reportsTheMissingUnstableExportModuleResolutionF_505()
		})
	})

	When("a declared unstable export subpath resolves to a file that does not exist (a broken exports entry, not a missing one)", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_541()
		})

		It("reports a broken-export-target load failure, distinct from a missing export entry, when the compiler's own package.json points at a file that doesn't exist", func() {
			body_projectTsBackendAcceptanceTest_reportsABrokenExportTargetLoadFailureDistinctFro_547()
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

	When("a recording node shim is first on PATH", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_597()
		})

		It("probes process.execPath and spawns that path so the shim log does not contain --compiler-module", func() {
			body_projectTsBackendAcceptanceTest_probesProcessExecPathAndSpawnsThatPathSoTheShimL_603()
		})
	})

	When("ambient PATH includes a sibling directory containing a shim", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_643()
		})

		It("sets the analyzer child PATH to the runtime directory with no extra components", func() {
			body_projectTsBackendAcceptanceTest_setsTheAnalyzerChildPATHToTheRuntimeDirectoryWit_649()
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
			body_projectTsBackendAcceptanceTest_728()
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

	When("TMPDIR contains typescript or package.json decoys", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			body_projectTsBackendAcceptanceTest_768()
		})

		It("does not change the scan result when $TMPDIR/node_modules/typescript is a decoy", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			installRealTypescriptCompiler(repo, true)

			decoy := filepath.Join(os.TempDir(), "node_modules", "typescript")
			Expect(os.MkdirAll(decoy, 0o755)).To(Succeed())
			DeferCleanup(os.RemoveAll, filepath.Join(os.TempDir(), "node_modules"))
			Expect(os.WriteFile(filepath.Join(decoy, "package.json"), []byte(`{"name":"typescript","version":"0.0.0"}`+"\n"), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(HaveLen(1))
		})

		It("does not change the scan result when $TMPDIR/package.json declares type commonjs", func() {
			body_projectTsBackendAcceptanceTest_doesNotChangeTheScanResultWhenTMPDIRPackageJsonD_792()
		})
	})
})

// tsPrismaClientPackageJSON/tsPrismaClientIndexTS mirror
// pkg/projectmodel/ts_sidecar_integration_acceptance_test.go's own
// vendor/prisma-client fixture: a package.json declaring the "@prisma/client"
// name (mirrored into the sidecar's virtual node_modules regardless of its
// real on-disk directory name) plus a PrismaClient class shaped to match
// REACHABILITY_SINK_CLASSES ("user.findMany").
const tsPrismaClientPackageJSON = `{"name":"@prisma/client","main":"index"}`

const tsPrismaClientIndexTS = "export class PrismaClient {\n  user = {\n    findMany(): Promise<unknown[]> {\n      return Promise.resolve([]);\n    },\n  };\n}\n"

// tsServiceNoopFile is a real file under pkg/service/ solely so
// tsLayerBypassRequiredConfigJSON's "service" layer prefix matches at least
// one file in the snapshot -- an unmatched prefix makes
// BuildTypeScriptLayerBypassFromModel treat the required layer as ambiguous
// (see tsLayerBypassRequiredConfigJSON's own doc comment) regardless of any
// bypass witness elsewhere.
const tsServiceNoopFile = "export const noop = 1;\n"

// tsHandlersBypassFile calls the pinned Prisma sink directly from
// pkg/handlers, never passing through pkg/service, reproducing a genuine
// architecture.layer_bypass witness under tsLayerBypassRequiredConfigJSON's
// required_layer.
const tsHandlersBypassFile = "import { PrismaClient } from \"@prisma/client\";\n\nconst prisma = new PrismaClient();\n\ninterface App {\n  get(path: string, handler: (req: unknown, res: unknown) => void): void;\n}\ndeclare const app: App;\n\nexport async function getUsersBypass(req: unknown, res: unknown): Promise<void> {\n  const users = await prisma.user.findMany();\n  console.log(users, req, res);\n}\napp.get(\"/users-bypass\", getUsersBypass);\n"

// tsHandlersReachabilityFile is a route handler with a fully resolved call
// path to the pinned Prisma sink, producing one possible_call_reachability
// ProjectFact.
const tsHandlersReachabilityFile = "import { PrismaClient } from \"@prisma/client\";\n\nconst prisma = new PrismaClient();\n\ninterface App {\n  get(path: string, handler: (req: unknown, res: unknown) => void): void;\n}\ndeclare const app: App;\n\nexport async function getUsers(req: unknown, res: unknown): Promise<void> {\n  const users = await prisma.user.findMany();\n  console.log(users, req, res);\n}\napp.get(\"/users\", getUsers);\n"

// tsHandlersLocalGapHelperFile/tsHandlersLocalGapFile reproduce a genuine,
// routine ts_reachability_local_call_not_followed_gap: a route handler
// delegating one hop into an imported local function this sidecar's depth-1
// walk does not itself follow, mirroring
// ts_sidecar_integration_acceptance_test.go's "getUsersCompliant" fixture.
const tsHandlersLocalGapHelperFile = "export function loadStuff(): unknown[] {\n  return [];\n}\n"

const tsHandlersLocalGapFile = "import { loadStuff } from \"./helper\";\n\ninterface App {\n  get(path: string, handler: (req: unknown, res: unknown) => void): void;\n}\ndeclare const app: App;\n\nexport function getStuff(req: unknown, res: unknown): void {\n  const items = loadStuff();\n  console.log(items, req, res);\n}\napp.get(\"/stuff\", getStuff);\n"

// tsLayerBypassRequiredConfigJSON declares handlers/db/service layers with
// service as required_layer, matching tsServiceNoopFile/tsHandlersBypassFile
// above -- the TypeScript analog of goLayerBypassPolicyConfigJSON
// (project_go_backend_acceptance_test.go).
const tsLayerBypassRequiredConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]},{"name":"service","prefixes":["pkg/service"]}],"forbidden_imports":[{"from":"handlers","to":"db"}],"required_layer":"service"}`

// tsLayerBypassAmbiguousConfigJSON names required_layer "service" with a
// prefix that matches no file anywhere in the snapshot, forcing
// BuildTypeScriptLayerBypassFromModel's ambiguous-layer guard (mirroring
// pkg/projectmodel/ts_layer_bypass_acceptance_test.go's own "the required
// layer's prefixes match no node anywhere in the snapshot" spec) regardless
// of the call graph.
const tsLayerBypassAmbiguousConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]},{"name":"service","prefixes":["nonexistent_service_dir"]}],"forbidden_imports":[{"from":"handlers","to":"db"}],"required_layer":"service"}`

// tsRootScopeGapTSConfigJSON reproduces SA-280-025's real candidate/analyzed
// root-scope mismatch (mirroring
// ts_sidecar_integration_acceptance_test.go's own "a tsconfig lists a JSON
// file as an explicit root file" spec): resolveJsonModule plus an explicit
// "files" entry accepts package.json into the compiler's Program as a root
// file, but js/semantics/src/project-sidecar/edges.ts's
// extractEdgesFromRootFile only ever visits .ts/.tsx root files, so
// package.json is a candidate the real compiler never actually analyzes.
// "include" is added alongside "files" (TypeScript unions the two) so the
// rest of the project's .ts sources are still discovered and analyzed
// normally, unlike the narrower pkg/projectmodel-level fixture this mirrors.
const tsRootScopeGapTSConfigJSON = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10","resolveJsonModule":true},"files":["package.json"],"include":["**/*.ts"]}`

// T7 (issue #331 Task 8): one analyzer response per revision must feed
// layer-violation, layer-bypass, and reachability-facts derivation alike,
// and incompleteness in each must fold into (or, for reachability, stay out
// of) the project-change lifecycle exactly as documented on
// tsProjectBackend.evaluateRevision (internal/codesignalcli/ts_project_revision.go).
var _ = Describe("coach codesignal --project-language typescript derives layer violations, layer bypass, and reachability facts from one analyzer response per revision (issue #331 Task 8 T7)", func() {
	BeforeEach(func() {
		body_projectTsBackendAcceptanceTest_889()
	})

	When("a baseline (single-revision) analysis runs", Label("ts-project-backend"), func() {
		It("invokes the analyzer exactly once while still deriving layer-violation, layer-bypass, and reachability facts from that one response (AC-1/AC-4/AC-11/AC-12)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "pkg/service/svc.ts", tsServiceNoopFile)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/bypass.ts", tsHandlersBypassFile)
			headSHA := commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
			installRealTypescriptCompiler(repo, true)

			sampler := startAnalyzerEnvironSampler()
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			environs := sampler.halt()
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(sampler.invocations()).To(Equal(1), "expected exactly one analyzer invocation for a baseline analysis, observed pids: %+v", environs)

			report := decodeCoachReport(stdout)
			ruleIDs := projectChangeRuleIDs(report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_violation"), "got %+v", report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_bypass"), "expected a layer-bypass ProjectChange derived from the same single analyzer response, got %+v", report.ProjectChanges)
			Expect(report.ProjectFacts).NotTo(BeEmpty(), "expected reachability facts derived from the same single analyzer response")
			Expect(report.ProjectFacts[0].Kind).To(Equal("possible_call_reachability"))
			assertReachabilityNeverSignalOrChange(report)

			textSampler := startAnalyzerEnvironSampler()
			textStdout, textStderr, textExitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
			textEnvirons := textSampler.halt()
			Expect(textExitCode).To(Equal(0), "stderr: %s stdout: %s", textStderr, textStdout)
			Expect(textSampler.invocations()).To(Equal(1), "expected exactly one analyzer invocation for the text-format rendering of the same baseline analysis, observed pids: %+v", textEnvirons)

			findingsSection, factsSection := splitTextFindingsAndFacts(string(textStdout))
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_violation"), "got %q", findingsSection)
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_bypass"), "got %q", findingsSection)
			Expect(findingsSection).NotTo(ContainSubstring("possible_call_reachability"), "reachability must never appear in the Signals/ProjectChanges findings section, got %q", findingsSection)
			Expect(factsSection).To(ContainSubstring("kind: possible_call_reachability"), "got %q", factsSection)
			Expect(factsSection).NotTo(ContainSubstring("rule_id:"), "the Facts section must never carry a rule_id, which would make a fact indistinguishable from a Signal/ProjectChange, got %q", factsSection)

			scopeSampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, tsLayerBypassRequiredConfigJSON)
			scopeEnvirons := scopeSampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(scopeSampler.invocations()).To(Equal(1), "expected exactly one analyzer invocation for the direct-result inspection of the same baseline analysis, observed pids: %+v", scopeEnvirons)

			resultRuleIDs := projectChangeRuleIDs(result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_violation"), "the same single-invocation result must carry the layer-violation change, got %+v", result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_bypass"), "the same single-invocation result must carry the layer-bypass change, got %+v", result.HeadChanges)
			Expect(result.Facts).NotTo(BeEmpty(), "the same single-invocation result must carry the reachability fact")
			Expect(result.Facts[0].Kind).To(Equal("possible_call_reachability"))
			Expect(result.HeadProjectScope).NotTo(BeNil(), "the same single-invocation result must carry project_scope (AC-11)")
			Expect(result.HeadModelCoverage).NotTo(BeNil(), "the same single-invocation result must carry model-phase coverage (AC-11)")
			Expect(result.HeadBypassCoverage).NotTo(BeNil(), "the same single-invocation result must carry bypass-phase coverage (AC-11)")
			Expect(result.HeadReachabilityCoverage).NotTo(BeNil(), "the same single-invocation result must carry reachability-phase coverage (AC-11)")
		})
	})

	When("a --base diff analyzes two revisions", Label("ts-project-backend"), func() {
		It("invokes the analyzer exactly twice, once per revision, each still deriving all three observation kinds from its own single response", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "pkg/service/svc.ts", tsServiceNoopFile)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/bypass.ts", tsHandlersBypassFile)
			baseSHA := commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
			headSHA := commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			installRealTypescriptCompiler(repo, true)

			sampler := startAnalyzerEnvironSampler()
			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			environs := sampler.halt()
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(sampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision (head + base), observed pids: %+v", environs)

			report := decodeCoachReport(stdout)
			ruleIDs := projectChangeRuleIDs(report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_violation"), "got %+v", report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_bypass"), "expected a layer-bypass ProjectChange present on both revisions, got %+v", report.ProjectChanges)
			Expect(report.ProjectFacts).NotTo(BeEmpty())
			assertReachabilityNeverSignalOrChange(report)

			textSampler := startAnalyzerEnvironSampler()
			textStdout, textStderr, textExitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
			textEnvirons := textSampler.halt()
			Expect(textExitCode).To(Equal(0), "stderr: %s stdout: %s", textStderr, textStdout)
			Expect(textSampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision for the text-format rendering of the same diff, observed pids: %+v", textEnvirons)

			findingsSection, factsSection := splitTextFindingsAndFacts(string(textStdout))
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_violation"), "got %q", findingsSection)
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_bypass"), "got %q", findingsSection)
			Expect(findingsSection).NotTo(ContainSubstring("possible_call_reachability"), "reachability must never appear in the Signals/ProjectChanges findings section, got %q", findingsSection)
			Expect(factsSection).To(ContainSubstring("kind: possible_call_reachability"), "got %q", factsSection)
			Expect(factsSection).NotTo(ContainSubstring("rule_id:"), "the Facts section must never carry a rule_id, which would make a fact indistinguishable from a Signal/ProjectChange, got %q", factsSection)

			scopeSampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, baseSHA, false, tsLayerBypassRequiredConfigJSON)
			scopeEnvirons := scopeSampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(scopeSampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision for the direct-result inspection of the same diff, observed pids: %+v", scopeEnvirons)

			resultRuleIDs := projectChangeRuleIDs(result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_violation"), "the same single Analyze() result must carry the head-side layer-violation change, got %+v", result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_bypass"), "the same single Analyze() result must carry the head-side layer-bypass change, got %+v", result.HeadChanges)
			Expect(result.Facts).NotTo(BeEmpty(), "the same single Analyze() result must carry the reachability fact")
			Expect(result.Facts[0].Kind).To(Equal("possible_call_reachability"))
			Expect(result.HeadProjectScope).NotTo(BeNil(), "the same single Analyze() result must carry head-side project_scope (AC-11)")
			Expect(result.BaseProjectScope).NotTo(BeNil(), "the same single Analyze() result must carry base-side project_scope (AC-11)")
			Expect(result.HeadModelCoverage).NotTo(BeNil(), "the same single Analyze() result must carry head-side model-phase coverage (AC-11)")
			Expect(result.BaseModelCoverage).NotTo(BeNil(), "the same single Analyze() result must carry base-side model-phase coverage (AC-11)")
			Expect(result.HeadBypassCoverage).NotTo(BeNil(), "the same single Analyze() result must carry head-side bypass-phase coverage (AC-11)")
			Expect(result.BaseBypassCoverage).NotTo(BeNil(), "the same single Analyze() result must carry base-side bypass-phase coverage (AC-11)")
			Expect(result.HeadReachabilityCoverage).NotTo(BeNil(), "the same single Analyze() result must carry head-side reachability-phase coverage (AC-11)")
			Expect(result.BaseReachabilityCoverage).NotTo(BeNil(), "the same single Analyze() result must carry base-side reachability-phase coverage (AC-11)")
		})
	})

	When("a required_layer is configured but its bypass search cannot resolve any file under that layer (an ambiguous, forced-incomplete search)", Label("ts-project-backend"), func() {
		It("degrades HeadCoverage to incomplete and every project-change lifecycle to unknown (AC-2/AC-14)", func() {
			body_projectTsBackendAcceptanceTest_degradesHeadCoverageToIncompleteAndEveryProjectC_1020()
		})
	})

	When("the analyzed repository has a routine, per-hop reachability gap but no model or bypass incompleteness", Label("ts-project-backend"), func() {
		It("keeps HeadCoverage complete and every layer-violation lifecycle determinate, surfacing the gap only via reachability facts/diagnostics (AC-3)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/reach.ts", tsHandlersReachabilityFile)
			commitFile(repo, "pkg/handlers/helper.ts", tsHandlersLocalGapHelperFile)
			commitFile(repo, "pkg/handlers/gap.ts", tsHandlersLocalGapFile)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark project coverage incomplete, got %+v", report.ProjectCoverage)
			Expect(containsProjectModelDiagnosticCode(report.ProjectCoverage.Diagnostics, "ts_reachability_local_call_not_followed_gap")).To(BeTrue(), "expected the routine reachability gap to surface on ProjectCoverage.Diagnostics, got %+v", report.ProjectCoverage.Diagnostics)

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].RuleID).To(Equal("architecture.layer_violation"))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("baseline"), "an unrelated reachability gap must never degrade an otherwise complete layer-violation finding's lifecycle")

			Expect(report.ProjectFacts).NotTo(BeEmpty(), "expected the resolved reachability edge to still surface as a fact")
			Expect(report.ProjectFacts[0].Kind).To(Equal("possible_call_reachability"))

			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")), "a routine reachability gap alone must never make the project-change lifecycle indeterminate")
		})
	})

	When("a required_layer is configured and the analyzed repository has a routine, per-hop reachability gap but no bypass-search incompleteness", Label("ts-project-backend"), func() {
		It("keeps ProjectCoverage complete and the layer-violation lifecycle determinate, and folds the gap diagnostic in exactly once (AC-3)", func() {
			body_projectTsBackendAcceptanceTest_keepsProjectCoverageCompleteAndTheLayerViolation_1095()
		})
	})

	When("a candidate file is accepted into the compiler's Program but never actually analyzed on the head revision (SA-280-025), with no bypass configured", Label("ts-project-backend"), func() {
		It("degrades HeadCoverage to incomplete and the layer-violation ProjectChange's lifecycle to unknown", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse(), "expected a real candidate/analyzed root-scope mismatch to mark project coverage incomplete, got %+v", report.ProjectCoverage)
			Expect(containsProjectModelDiagnosticCode(report.ProjectCoverage.Diagnostics, projectmodel.DiagRootScopeIncomplete)).To(BeTrue(), "got %+v", report.ProjectCoverage.Diagnostics)

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("unknown"), "model incompleteness on the analyzed revision must degrade the layer-violation lifecycle to unknown")

			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})
	})

	When("a --base diff has the SA-280-025 root-scope mismatch only on the base revision", Label("ts-project-backend"), func() {
		It("degrades the diff's project-change lifecycle to unknown even though the head revision's own coverage is complete", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "sanity: the head revision's own coverage must be complete, or this spec is not isolating the base-side failure it claims to")

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("unknown"), "base-side model incompleteness must still degrade the diff's project-change lifecycle to unknown")

			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})
	})

	When("a --base diff has the SA-280-025 root-scope mismatch only on the head revision, with the base revision fully complete", Label("ts-project-backend"), func() {
		It("degrades the diff's project-change lifecycle to unknown even though the base revision's own coverage is complete", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse(), "expected the head-side root-scope mismatch to mark head coverage incomplete, got %+v", report.ProjectCoverage)

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("unknown"), "head-side model incompleteness must degrade the diff's project-change lifecycle to unknown even though base coverage is complete")

			lifecycleMessage := diagnosticMessageForKind(report.Diagnostics, "project_lifecycle_indeterminate")
			Expect(lifecycleMessage).To(ContainSubstring("head coverage incomplete"), "expected the indeterminacy reason to name head coverage, got %q", lifecycleMessage)
			Expect(lifecycleMessage).NotTo(ContainSubstring("base coverage incomplete"), "the base revision is fully complete in this fixture; the indeterminacy reason must not blame it too, got %q", lifecycleMessage)
		})
	})
})

// tsHandlersExtraFile is a second real file under pkg/handlers/, alongside
// tsRealHandlersImportingDB, so a root scoped to pkg/handlers (see
// tsNestedRootsScopeConfigJSON) has a distinct, independently-verifiable
// file count from the outer "." root that also contains pkg/db/d.ts.
const tsHandlersExtraFile = "export const extra = 1;\n"

// tsUtilMiscFile lives under a prefix no layer in goLayerPolicyConfigJSON
// declares (only "handlers" and "db" are configured), reproducing AC-5's
// "a file outside every layer" fixture: it must still be counted as an
// ordinary candidate/analyzed file, without spuriously creating or
// expanding any layer's matched set.
const tsUtilMiscFile = "export const misc = 1;\n"

// tsNestedRootsScopeConfigJSON declares two roots where the second
// ("pkg/handlers") nests inside the first ("."), plus a third layer
// ("unused") whose prefix matches no file the fixtures below ever commit --
// reproducing SA-280-005/SA-280-025's independent per-root accounting and
// matched_layers/unmatched_layers split in one fixture.
const tsNestedRootsScopeConfigJSON = `{"schema_version":"1","roots":[".","pkg/handlers"],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]},{"name":"unused","prefixes":["pkg/does-not-exist"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

// T1 (issue #332 Task 9): ProjectScopeFromModel is wired into
// tsProjectBackend.evaluateRevision and its result carried per analyzed
// revision on ProjectBackendResult, derived from the same single analyzer
// response the layer-violation/layer-bypass/reachability evidence families
// above already share (AC-RUN-5's one-invocation-per-revision instrumentation
// applies here unchanged).
var _ = Describe("coach codesignal --project-language typescript carries project_scope on ProjectBackendResult, derived from the same analyzer response as the other evidence families (coach#332 Task 9 T1)", func() {
	BeforeEach(func() {
		body_projectTsBackendAcceptanceTest_1239()
	})

	When("a baseline analysis runs against a multi-root policy with a nested root", Label("ts-project-backend"), func() {
		It("derives HeadProjectScope with independent per-root candidate/analyzed counts, matched_layers, unmatched_layers, inclusion_rule, and pattern_set from one analyzer response (AC-3/AC-15/AC-25/AC-26)", func() {
			body_projectTsBackendAcceptanceTest_derivesHeadProjectScopeWithIndependentPerRootCan_1246()
		})
	})

	When("a --base diff analyzes two revisions under the same multi-root policy", Label("ts-project-backend"), func() {
		It("invokes the analyzer exactly twice, once per revision, and carries both HeadProjectScope and BaseProjectScope (AC-3/AC-RUN-5)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/handlers/tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile)
			baseSHA := commitFile(repo, "project.json", tsNestedRootsScopeConfigJSON)
			headSHA := commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile+"export const more = 2;\n")
			installRealTypescriptCompiler(repo, true)

			sampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, baseSHA, false, tsNestedRootsScopeConfigJSON)
			environs := sampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(sampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision (head + base), no second analyzer pass just to derive project_scope, observed pids: %+v", environs)

			Expect(result.HeadProjectScope).NotTo(BeNil(), "head-side project_scope must be carried")
			Expect(result.BaseProjectScope).NotTo(BeNil(), "base-side project_scope must be carried under --base")
			Expect(result.HeadProjectScope.Roots).To(HaveLen(2))
			Expect(result.BaseProjectScope.Roots).To(HaveLen(2))
		})
	})

	When("the tsRootScopeGapTSConfigJSON fixture accepts a candidate file into the compiler's Program that is never actually analyzed", Label("ts-project-backend"), func() {
		It("counts the unanalyzable candidate file in candidate_files but not analyzed_files, and names it in its own diagnostic (AC-5/AC-25)", func() {
			body_projectTsBackendAcceptanceTest_countsTheUnanalyzableCandidateFileInCandidateFil_1317()
		})
	})

	When("a fixture file matches none of the configured layers' prefixes", Label("ts-project-backend"), func() {
		It("counts the file as an ordinary candidate/analyzed file without it appearing in any layer's matched set (AC-5)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "pkg/util/misc.ts", tsUtilMiscFile)
			headSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, goLayerPolicyConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.HeadProjectScope).NotTo(BeNil())
			Expect(result.HeadProjectScope.Roots).To(HaveLen(1))
			root := result.HeadProjectScope.Roots[0]
			Expect(root.CandidateFiles).To(Equal(3), "expected d.ts, h.ts, and misc.ts (outside every configured layer) as candidates, got %+v", root)
			Expect(root.AnalyzedFiles).To(Equal(3), "got %+v", root)

			Expect(result.HeadProjectScope.MatchedLayers).To(ConsistOf("handlers", "db"), "a file outside every configured layer must not spuriously create or expand a layer match, got %+v", result.HeadProjectScope.MatchedLayers)
			Expect(result.HeadProjectScope.UnmatchedLayers).To(BeEmpty())
		})
	})
})

// T2 (issue #332 Task 9): ProjectBackendResult carries each analyzed
// revision's model/bypass/reachability Coverage independently of
// HeadCoverage/BaseCoverage's existing combined fold (PR #386), which stays
// unchanged. analyzeTSProjectBackend is reused from T1 above: these fields
// are not yet rendered through codesignal.Input/Report either, so
// ProjectBackendResult remains the most meaningful boundary to observe them.
var _ = Describe("coach codesignal --project-language typescript carries per-phase (model, bypass, reachability) coverage per revision on ProjectBackendResult, additive to the existing fold (coach#332 Task 9 T2)", func() {
	BeforeEach(func() {
		body_projectTsBackendAcceptanceTest_1385()
	})

	When("the tsRootScopeGapTSConfigJSON fixture accepts a candidate file into the compiler's Program that is never actually analyzed, with no bypass configured", Label("ts-project-backend"), func() {
		It("marks model-phase coverage incomplete while leaving the existing folded HeadCoverage exactly as it already was (SA-280-025, AC-9/AC-13/AC-18/AC-28)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			headSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, goLayerPolicyConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.HeadModelCoverage.Complete).To(BeFalse(), "an unanalyzable candidate file must never be reported as complete model-phase coverage, got %+v", result.HeadModelCoverage)
			Expect(containsProjectModelDiagnosticCode(result.HeadModelCoverage.Diagnostics, projectmodel.DiagRootScopeIncomplete)).To(BeTrue(), "got %+v", result.HeadModelCoverage.Diagnostics)

			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeFalse(), "the existing folded HeadCoverage must stay incomplete exactly as PR #386 already produces it")
			Expect(*result.HeadCoverage).To(Equal(*result.HeadModelCoverage), "with no bypass configured the existing fold is exactly the model coverage, unchanged by this task's additive fields")
		})
	})

	When("a required_layer is configured and its bypass search finds a genuine witness, with an unrelated routine reachability gap elsewhere in the snapshot", Label("ts-project-backend"), func() {
		It("carries the bypass search's own reachability-gap-excluded completeness (tsBypassCoverageForFold), not BuildTypeScriptLayerBypassFromModel's raw gap-folded Coverage, as a phase distinct from the existing folded HeadCoverage (AC-4/AC-16)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "pkg/service/svc.ts", tsServiceNoopFile)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/reach.ts", tsHandlersReachabilityFile)
			commitFile(repo, "pkg/handlers/helper.ts", tsHandlersLocalGapHelperFile)
			commitFile(repo, "pkg/handlers/gap.ts", tsHandlersLocalGapFile)
			headSHA := commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, tsLayerBypassRequiredConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.HeadBypassCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage).NotTo(BeNil())

			Expect(result.HeadBypassCoverage.Phase).To(Equal("ts_layer_bypass"), "the bypass-phase coverage must be the bypass search's own Coverage (tsBypassCoverageForFold), not the folded model Coverage, got %+v", result.HeadBypassCoverage)
			Expect(result.HeadBypassCoverage.Complete).To(BeTrue(), "tsBypassCoverageForFold must exclude the routine reachability-gap term from the bypass search's own completeness; BuildTypeScriptLayerBypassFromModel's raw Coverage folds the gap in via tsReachabilityHasGap and would report false here, got %+v", result.HeadBypassCoverage)
			Expect(result.HeadCoverage.Phase).To(Equal(result.HeadModelCoverage.Phase), "the existing folded HeadCoverage keeps the model's own Phase unchanged, per combineProjectCoverage's documented convention")
			Expect(*result.HeadBypassCoverage).NotTo(Equal(*result.HeadCoverage), "the bypass-phase coverage and the existing folded model+bypass HeadCoverage are different values")
		})
	})

	When("no required_layer is configured, and the analyzed repository has a routine, per-hop reachability gap but no model incompleteness", Label("ts-project-backend"), func() {
		It("reports bypass-phase coverage as exactly not_requested and lets reachability-phase coverage go incomplete independently of model-phase and the existing folded HeadCoverage, both of which stay complete (AC-4/AC-16)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/reach.ts", tsHandlersReachabilityFile)
			commitFile(repo, "pkg/handlers/helper.ts", tsHandlersLocalGapHelperFile)
			commitFile(repo, "pkg/handlers/gap.ts", tsHandlersLocalGapFile)
			headSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, goLayerPolicyConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.HeadBypassCoverage).NotTo(BeNil())
			Expect(result.HeadBypassCoverage.Phase).To(Equal("not_requested"), "no required_layer is configured, so the bypass phase must never claim it ran, got %+v", result.HeadBypassCoverage)
			Expect(result.HeadBypassCoverage.Complete).To(BeTrue(), "a phase that was never requested is not itself an incompleteness")

			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.HeadModelCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark model-phase coverage incomplete, got %+v", result.HeadModelCoverage)

			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark the existing folded HeadCoverage incomplete, got %+v", result.HeadCoverage)

			Expect(result.HeadReachabilityCoverage).NotTo(BeNil())
			Expect(result.HeadReachabilityCoverage.Complete).To(BeFalse(), "BuildTypeScriptReachabilityFromModel folds the routine gap into its own Coverage.Complete, independently of model-phase and the existing folded HeadCoverage, got %+v", result.HeadReachabilityCoverage)
		})
	})

	When("a --base diff has the SA-280-025 root-scope mismatch only on the base revision, with no bypass configured", Label("ts-project-backend"), func() {
		It("carries base-revision-specific model/bypass/reachability coverage independently of the head revision's own values (AC-4/AC-16)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			headSHA := commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, baseSHA, false, goLayerPolicyConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.BaseModelCoverage).NotTo(BeNil())
			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.BaseModelCoverage.Complete).To(BeFalse(), "the base revision's own unanalyzable candidate file must degrade base-phase model coverage independently of the head revision, got %+v", result.BaseModelCoverage)
			Expect(result.HeadModelCoverage.Complete).To(BeTrue(), "sanity: the head revision's own model coverage must be complete, or this spec is not isolating the base-side failure it claims to, got %+v", result.HeadModelCoverage)

			Expect(result.BaseBypassCoverage).NotTo(BeNil())
			Expect(result.HeadBypassCoverage).NotTo(BeNil())
			Expect(result.BaseBypassCoverage.Phase).To(Equal("not_requested"), "no required_layer is configured, so the base-side bypass phase must never claim it ran either, got %+v", result.BaseBypassCoverage)
			Expect(result.HeadBypassCoverage.Phase).To(Equal("not_requested"), "got %+v", result.HeadBypassCoverage)

			Expect(result.BaseReachabilityCoverage).NotTo(BeNil())
			Expect(result.HeadReachabilityCoverage).NotTo(BeNil())
			Expect(result.BaseReachabilityCoverage.Complete).To(BeFalse(), "tsReachabilityCoverage folds the base revision's own model.Coverage.Complete term into reachability-phase coverage, so the base-only root-scope mismatch must degrade it independently of the head revision, got %+v", result.BaseReachabilityCoverage)
			Expect(result.HeadReachabilityCoverage.Complete).To(BeTrue(), "sanity: the head revision's own reachability-phase coverage must be complete, or this spec is not isolating the base-side failure it claims to, got %+v", result.HeadReachabilityCoverage)
		})
	})
})
