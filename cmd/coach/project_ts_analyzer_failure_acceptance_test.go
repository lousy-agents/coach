package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

const (
	analyzerTestHookFlagPrefix = "--coach-test-hook="
	analyzerTestHookCrash      = "crash-partway"
	analyzerTestHookHang       = "hang"
)

func buildTestHookCoach(hook string, shortenSidecarBudget bool) string {
	root := repositoryRoot()
	tmp, err := os.MkdirTemp("", "coach-analyzer-test-hook-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, tmp)

	replace := map[string]string{}

	runtimeSrc := filepath.Join(root, "internal", "codesignalcli", "project_ts_runtime.go")
	runtimeBytes, err := os.ReadFile(runtimeSrc)
	Expect(err).NotTo(HaveOccurred())
	patchedRuntime := strings.Replace(string(runtimeBytes),
		`"--native-package=" + compiler.NativePackagePath,`,
		`"--native-package=" + compiler.NativePackagePath,`+"\n\t\t\""+analyzerTestHookFlagPrefix+hook+`",`,
		1)
	Expect(patchedRuntime).NotTo(Equal(string(runtimeBytes)), "throwaway must spawn the analyzer through the argv test hook")
	runtimeDst := filepath.Join(tmp, "project_ts_runtime.go")
	Expect(os.WriteFile(runtimeDst, []byte(patchedRuntime), 0o644)).To(Succeed())
	replace[runtimeSrc] = runtimeDst

	if shortenSidecarBudget {
		backendSrc := filepath.Join(root, "internal", "codesignalcli", "project_ts_backend.go")
		backendBytes, err := os.ReadFile(backendSrc)
		Expect(err).NotTo(HaveOccurred())
		patchedBackend := strings.Replace(string(backendBytes),
			"const tsSidecarWallTime = goProjectBuildWallTime",
			"const tsSidecarWallTime = goProjectBuildWallTime / 20",
			1)
		Expect(patchedBackend).NotTo(Equal(string(backendBytes)), "throwaway must shorten the analyzer wall-clock budget")
		backendDst := filepath.Join(tmp, "project_ts_backend.go")
		Expect(os.WriteFile(backendDst, []byte(patchedBackend), 0o644)).To(Succeed())
		replace[backendSrc] = backendDst
	}

	overlay := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: replace}
	overlayPath := filepath.Join(tmp, "overlay.json")
	payload, err := json.Marshal(overlay)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(overlayPath, payload, 0o644)).To(Succeed())

	bin := filepath.Join(tmp, "coach")
	build := exec.Command("go", "build", "-overlay", overlayPath, "-o", bin, ".")
	build.Dir = filepath.Join(root, "cmd", "coach")
	out, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building the throwaway test-hook coach: %s", out)
	return bin
}

func runCoachBaselineWith(coachPath, repo string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(coachPath, repo, nil, append([]string{"codesignal", "--baseline"}, extraArgs...)...)
}

func expectQualifiedIncompleteAnalyzerFailure(coachPath, repo, expectedDiagnostic string) {
	jsonStdout, jsonStderr, jsonExit := runCoachBaselineWith(coachPath, repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	ExpectWithOffset(1, jsonExit).To(Equal(0), "AC-RUN-9 qualifies the report rather than failing the run; stderr: %s stdout: %s", jsonStderr, jsonStdout)

	report := decodeCoachReport(jsonStdout)
	ExpectWithOffset(1, report.ProjectChanges).To(BeEmpty(), "a failed analyzer must never fabricate the layer violation it never reported")
	ExpectWithOffset(1, report.ProjectCoverage).NotTo(BeNil())
	ExpectWithOffset(1, report.ProjectCoverage.Complete).To(BeFalse())

	var message string
	var found bool
	for _, diag := range report.ProjectCoverage.Diagnostics {
		if diag.Code == projectmodel.DiagBackendUnavailable {
			found = true
			message = diag.Message
		}
	}
	ExpectWithOffset(1, found).To(BeTrue(), "expected a %s diagnostic, got %+v", projectmodel.DiagBackendUnavailable, report.ProjectCoverage.Diagnostics)
	ExpectWithOffset(1, message).To(ContainSubstring(expectedDiagnostic), "expected the analyzer's own failure mode to surface, got: %s", message)

	textStdout, textStderr, textExit := runCoachBaselineWith(coachPath, repo, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
	ExpectWithOffset(1, textExit).To(Equal(0), "stderr: %s", textStderr)
	text := string(textStdout)
	ExpectWithOffset(1, text).NotTo(ContainSubstring("No active CodeSignal findings.\n"), "a failed analyzer must not render the unqualified clean-run verdict")
	ExpectWithOffset(1, text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))

	for _, rendered := range []string{string(jsonStdout), string(jsonStderr), text, string(textStderr)} {
		ExpectWithOffset(1, rendered).NotTo(ContainSubstring(repo), "must not expose the repository path: %s", rendered)
		ExpectWithOffset(1, rendered).NotTo(ContainSubstring("coach-ts-analyzer-"), "must not expose the analyzer temp-directory prefix: %s", rendered)
		ExpectWithOffset(1, rendered).NotTo(ContainSubstring("file://"), "must not expose a file URL: %s", rendered)
		ExpectWithOffset(1, rendered).NotTo(ContainSubstring("node:internal"), "must not expose a runtime stack frame: %s", rendered)
	}
}

var _ = Describe("coach codesignal --project-language typescript: analyzer failure after successful preflight (AC-RUN-9)", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	When("the analyzer crashes partway through analysis, after the resolved compiler and native package have loaded", func() {
		It("emits a qualified incomplete report at exit 0 that leaks no host path, file URL, or runtime stack frame", func() {
			repo := newTempGitRepo()
			commitRealTSLayerFixture(repo, realTypescriptVersion())
			installRealTypescriptCompiler(repo, true)

			expectQualifiedIncompleteAnalyzerFailure(buildTestHookCoach(analyzerTestHookCrash, false), repo, "ts sidecar exited")
		})
	})

	When("the analyzer hangs past the caller's wall-clock budget after the same successful preflight", func() {
		It("emits a qualified incomplete report at exit 0 that leaks no host path, file URL, or runtime stack frame", func() {
			repo := newTempGitRepo()
			commitRealTSLayerFixture(repo, realTypescriptVersion())
			installRealTypescriptCompiler(repo, true)

			expectQualifiedIncompleteAnalyzerFailure(buildTestHookCoach(analyzerTestHookHang, true), repo, "ts sidecar timed out")
		})
	})

	When("coach is built as shipped and the caller sets the one environment variable the analyzer honors", func() {
		It("completes the scan: the analyzer's environment is scrubbed, which is why its test hook has to be argv-level", func() {
			repo := newTempGitRepo()
			commitRealTSLayerFixture(repo, realTypescriptVersion())
			installRealTypescriptCompiler(repo, true)

			command := exec.Command(commandPath, "codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			command.Dir = repo
			command.Env = append(os.Environ(), "COACH_TS_SIDECAR_TEST_DELAY_MS=600000")
			var outBuf, errBuf bytes.Buffer
			command.Stdout = &outBuf
			command.Stderr = &errBuf
			Expect(command.Run()).To(Succeed(), "stderr: %s", errBuf.String())

			report := decodeCoachReport(outBuf.Bytes())
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "the shipped binary must complete the same scan the hooked throwaway degrades, diagnostics=%+v", report.ProjectCoverage.Diagnostics)
			Expect(report.ProjectChanges).NotTo(BeEmpty(), "this fixture carries a real forbidden edge, so a complete run reports it")
		})
	})
})
