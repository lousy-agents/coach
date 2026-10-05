package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --project-language typescript against the private embedded analyzer and a confined, host-resolved compiler (coach#326 Task 3)", func() {
	When("the analyzed repository vendors no js/semantics analyzer anywhere and declares a real, exactly-matching installed TypeScript compiler", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
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
			skipWithoutRealTypeScriptCompiler()
		})

		It("completes analysis without the spy running, without the analyzer child inheriting HTTP(S)_PROXY, and without the listener accepting a connection", func() {
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
		})
	})

	When("an uncommitted worktree edit would introduce a forbidden TypeScript layer edge that HEAD does not have", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
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
			skipWithoutRealTypeScriptCompiler()
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

	When("a recording node shim is first on PATH", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("probes process.execPath and spawns that path so the shim log does not contain --compiler-module", func() {
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
		})
	})

	When("ambient PATH includes a sibling directory containing a shim", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("sets the analyzer child PATH to the runtime directory with no extra components", func() {
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
		})
	})

	When("TMPDIR contains typescript or package.json decoys", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
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
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			installRealTypescriptCompiler(repo, true)

			pkg := filepath.Join(os.TempDir(), "package.json")
			_, existed := os.Stat(pkg)
			if existed == nil {
				prev, err := os.ReadFile(pkg)
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = os.WriteFile(pkg, prev, 0o644) })
			} else {
				DeferCleanup(os.Remove, pkg)
			}
			Expect(os.WriteFile(pkg, []byte(`{"type":"commonjs"}`+"\n"), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(HaveLen(1))
		})
	})
})
