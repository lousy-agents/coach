package main

import (
	"fmt"

	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectTsBackendAcceptanceTest_54() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_91() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_completesAnalysisWithoutTheSpyRunningWithoutTheA_97() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitRealTSLayerFixture(repo, version)
	installRealTypescriptCompiler(repo, true)

	markerDir, err := os.MkdirTemp("", "coach-ts-confinement-marker-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, markerDir)
	marker := filepath.Join(markerDir, "leaked")
	spy := filepath.Join(markerDir, "spy.cjs")
	Expect(os.WriteFile(spy, []byte("require('fs').writeFileSync("+fmt.Sprintf("%q", marker)+", 'leaked\\n');\n"), 0o644)).To(Succeed())

	listener := startRecordingProxyListener()
	DeferCleanup(listener.stop)
	proxyURL := listener.proxyURL()

	GinkgoT().Setenv("NODE_OPTIONS", "--require "+spy)
	GinkgoT().Setenv("HTTP_PROXY", proxyURL)
	GinkgoT().Setenv("HTTPS_PROXY", proxyURL)
	GinkgoT().Setenv("npm_config_registry", proxyURL+"/registry/")

	sampler := startAnalyzerEnvironSampler()
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	environs := sampler.halt()

	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

	_, statErr := os.Stat(marker)
	Expect(os.IsNotExist(statErr)).To(BeTrue(), "NODE_OPTIONS --require spy must not run in the analyzer child; marker %s exists", marker)

	Expect(environs).NotTo(BeEmpty(), "must observe the analyzer child (--compiler-module argv); a vacuous PID sample cannot prove HTTP(S)_PROXY was not forwarded")
	for pid, env := range environs {
		Expect(env).To(ContainSubstring("PATH="), "analyzer child pid %d environ was argv-only; PATH= is the AC-RUN-2 false-green guard", pid)
		Expect(env).NotTo(ContainSubstring("HTTP_PROXY="), "analyzer child pid %d inherited HTTP_PROXY", pid)
		Expect(env).NotTo(ContainSubstring("HTTPS_PROXY="), "analyzer child pid %d inherited HTTPS_PROXY", pid)
	}
	Expect(listener.snapshot()).To(BeEmpty(), "analyzer child must not dial the parent HTTP(S)_PROXY listener; hits: %v", listener.snapshot())

	report := decodeCoachReport(stdout)
	Expect(report.ProjectCoverage).NotTo(BeNil())
	Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
	Expect(report.ProjectChanges).To(HaveLen(1))
}

func body_projectTsBackendAcceptanceTest_144() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_172() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_producesNoSideEffectDuringAScanBecauseMiseProbes_337() {
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
}

func body_projectTsBackendAcceptanceTest_431() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_reportsAQualifiedIncompleteReportAtExit0NotExit2_437() {
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

	var found bool
	var message string
	for _, diag := range report.ProjectCoverage.Diagnostics {
		if diag.Code == projectmodel.DiagBackendUnavailable {
			found = true
			message = diag.Message
		}
	}
	Expect(found).To(BeTrue(), "expected a %s diagnostic in ProjectCoverage.Diagnostics, got %+v", projectmodel.DiagBackendUnavailable, report.ProjectCoverage.Diagnostics)
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
}

func body_projectTsBackendAcceptanceTest_499() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
