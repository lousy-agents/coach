package main

import (
	"fmt"

	"os"
	"os/exec"
	"path/filepath"

	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectTsBackendAcceptanceTest_reportsTheMissingUnstableExportModuleResolutionF_505() {
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

	var found bool
	var message string
	for _, diag := range report.ProjectCoverage.Diagnostics {
		if diag.Code == projectmodel.DiagBackendUnavailable {
			found = true
			message = diag.Message
		}
	}
	Expect(found).To(BeTrue(), "expected a %s diagnostic in ProjectCoverage.Diagnostics, got %+v", projectmodel.DiagBackendUnavailable, report.ProjectCoverage.Diagnostics)
	Expect(message).To(ContainSubstring(`does not declare a "./unstable/fs" export`), "expected the missing-unstable-API failure to surface, got: %s", message)

	Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
	Expect(message).NotTo(ContainSubstring(repo), "diagnostic must not contain the repository path")
	Expect(message).NotTo(ContainSubstring("coach-ts-analyzer-"), "diagnostic must not contain the analyzer temp-directory prefix")
	Expect(message).NotTo(ContainSubstring("file://"))
	Expect(message).NotTo(ContainSubstring("node:internal"))
}

func body_projectTsBackendAcceptanceTest_541() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_reportsABrokenExportTargetLoadFailureDistinctFro_547() {
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

	var found bool
	var message string
	for _, diag := range report.ProjectCoverage.Diagnostics {
		if diag.Code == projectmodel.DiagBackendUnavailable {
			found = true
			message = diag.Message
		}
	}
	Expect(found).To(BeTrue(), "expected a %s diagnostic in ProjectCoverage.Diagnostics, got %+v", projectmodel.DiagBackendUnavailable, report.ProjectCoverage.Diagnostics)
	Expect(message).To(ContainSubstring("failed to load typescript/unstable/fs from the resolved TypeScript compiler"), "expected the broken-export-target failure to surface, got: %s", message)

	Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
	Expect(message).NotTo(ContainSubstring(repo), "diagnostic must not contain the repository path")
	Expect(message).NotTo(ContainSubstring("coach-ts-analyzer-"), "diagnostic must not contain the analyzer temp-directory prefix")
	Expect(message).NotTo(ContainSubstring("file://"))
	Expect(message).NotTo(ContainSubstring("node:internal"))
}

func body_projectTsBackendAcceptanceTest_597() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_probesProcessExecPathAndSpawnsThatPathSoTheShimL_603() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitRealTSLayerFixture(repo, version)
	installRealTypescriptCompiler(repo, true)

	realNode, err := exec.LookPath("node")
	Expect(err).NotTo(HaveOccurred())
	probe := exec.Command(realNode, "-p", "process.execPath")
	probed, err := probe.Output()
	Expect(err).NotTo(HaveOccurred())
	execPath := strings.TrimSpace(string(probed))

	shimDir, err := os.MkdirTemp("", "coach-recording-node-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, shimDir)
	logPath := filepath.Join(shimDir, "shim.log")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\nexec %q \"$@\"\n", logPath, realNode)
	Expect(os.WriteFile(filepath.Join(shimDir, "node"), []byte(script), 0o755)).To(Succeed())

	path := shimDir + string(os.PathListSeparator) + pathExcludingExecutables("node")
	sampler := startAnalyzerEnvironSampler()
	stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	environs := sampler.halt()
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

	logBytes, readErr := os.ReadFile(logPath)
	Expect(readErr).NotTo(HaveOccurred())
	Expect(string(logBytes)).NotTo(ContainSubstring("--compiler-module"), "analyzer child must be the probed execPath, not the PATH shim; log=%s", logBytes)

	Expect(environs).NotTo(BeEmpty(), "must observe the analyzer child")
	for _, env := range environs {
		childPath := pathValueFromEnviron(env)
		Expect(childPath).To(Equal(filepath.Dir(execPath)), "child PATH must equal the runtime directory exactly, got %q env=%q", childPath, env)
		Expect(childPath).NotTo(ContainSubstring(shimDir), "recording shim directory must be absent from child PATH")
	}
}

func body_projectTsBackendAcceptanceTest_643() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_setsTheAnalyzerChildPATHToTheRuntimeDirectoryWit_649() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitRealTSLayerFixture(repo, version)
	installRealTypescriptCompiler(repo, true)

	realNode, err := exec.LookPath("node")
	Expect(err).NotTo(HaveOccurred())
	probe := exec.Command(realNode, "-p", "process.execPath")
	probed, err := probe.Output()
	Expect(err).NotTo(HaveOccurred())
	runtimeDir := filepath.Dir(strings.TrimSpace(string(probed)))

	sibling, err := os.MkdirTemp("", "coach-path-sibling-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, sibling)
	Expect(os.WriteFile(filepath.Join(sibling, "node"), []byte("#!/bin/sh\necho sibling-shim\n"), 0o755)).To(Succeed())

	path := os.Getenv("PATH")
	GinkgoT().Setenv("PATH", path+string(os.PathListSeparator)+sibling)

	sampler := startAnalyzerEnvironSampler()
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	environs := sampler.halt()
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
	Expect(environs).NotTo(BeEmpty(), "must observe the analyzer child")
	for _, env := range environs {
		childPath := pathValueFromEnviron(env)
		Expect(childPath).To(Equal(runtimeDir), "child PATH must equal the runtime directory exactly, got %q env=%q", childPath, env)
		Expect(strings.Split(childPath, string(os.PathListSeparator))).To(Equal([]string{runtimeDir}))
		Expect(childPath).NotTo(ContainSubstring(sibling), "planted sibling must be absent from child PATH")
	}
}

func body_projectTsBackendAcceptanceTest_728() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_768() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
