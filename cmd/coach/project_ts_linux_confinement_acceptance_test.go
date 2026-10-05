package main

import (
	"fmt"
	"runtime"

	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Linux file-syscall control of the TypeScript analyzer subtree", func() {
	When("a TypeScript --baseline scan is file-syscall-traced on Linux", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			if runtime.GOOS != "linux" {
				Skip(linuxConfinementElsewhereSkip)
			}
			ensureFileSyscallTracer()
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
		})

		It("completes a confined --baseline scan whose analyzer-subtree file syscalls stay inside the frozen allowlist and never observe the typeRoots decoy", func() {
			repo, decoy := newLinuxTypeRootsFixture()
			traceFile := filepath.Join(os.TempDir(), fmt.Sprintf("coach-d2-strace-confined-%d.log", os.Getpid()))
			DeferCleanup(func() { _ = os.Remove(traceFile) })

			execPath := probedNodeExecPath()
			stdout, stderr, exitCode := runCoachBaselineUnderStrace(repo, commandPath, traceFile)
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].RuleID).To(Equal("architecture.layer_violation"))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))

			recs := parseStraceFile(traceFile)
			Expect(recs).NotTo(BeEmpty(), "trace file non-empty only is not identity; parser must observe syscalls")
			id, ok := findAnalyzerIdentity(recs, execPath)
			Expect(ok).To(BeTrue(), "identity must be one execve/execveat record whose pathname equals ExecPath and whose argv contains --compiler-module= and --native-package=; first ExecPath execve is the version probe")
			judgeConfinedAnalyzerSubtree(recs, id, execPath, decoy, repo, false)
		})

		It("records the typeRoots decoy in the analyzer-subtree trace when the harness-built unconfined analyzer omits tsserverPath and restores listing fall-through", func() {
			repo, decoy := newLinuxTypeRootsFixture()
			traceFile := filepath.Join(os.TempDir(), fmt.Sprintf("coach-d2-strace-unconfined-%d.log", os.Getpid()))
			DeferCleanup(func() { _ = os.Remove(traceFile) })

			unconfined := buildUnconfinedAnalyzerCoach()
			execPath := probedNodeExecPath()
			stdout, stderr, exitCode := runCoachBaselineUnderStrace(repo, unconfined, traceFile)
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			recs := parseStraceFile(traceFile)
			Expect(recs).NotTo(BeEmpty())
			id, ok := findAnalyzerIdentity(recs, execPath)
			Expect(ok).To(BeTrue(), "unconfined identity must still be ExecPath plus --compiler-module= and --native-package=")
			judgeConfinedAnalyzerSubtree(recs, id, execPath, decoy, repo, true)
		})
	})
})

var _ = Describe("Linux network-namespace control of the TypeScript analyzer", func() {
	When("a fake compiler dials a harness listener outside and inside a new network namespace", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			if runtime.GOOS != "linux" {
				Skip(linuxConfinementElsewhereSkip)
			}
			ensureUnshareAvailable()
		})

		It("connects successfully outside any namespace and the listener snapshot is non-empty", func() {
			listener, host, port := startNamespaceDialListener()
			DeferCleanup(listener.stop)
			mod := writeFakeCompilerDialModule(host, port)
			stdout, stderr, err := runFakeCompilerDial(nil, mod)
			Expect(err).NotTo(HaveOccurred(), "outside dial must connect; stdout=%s stderr=%s", stdout, stderr)
			Eventually(listener.snapshot).ShouldNot(BeEmpty(), "listener empty as only proof is invalid; fake compiler must dial")
		})

		It("fails the identical dial inside unshare with a distinctive error the outside half does not produce", func() {
			expectNamespacedDialFailsDistinctly()
		})
	})

	When("the production confined analyzer runs a real --baseline scan under unshare", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			if runtime.GOOS != "linux" {
				Skip(linuxConfinementElsewhereSkip)
			}
			ensureUnshareAvailable()
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
		})

		It("emits the same complete report as the non-unshare baseline for that fixture", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			installRealTypescriptCompiler(repo, true)

			baseOut, baseErr, baseExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(baseExit).To(Equal(0), "stderr: %s stdout: %s", baseErr, baseOut)
			baseReport := decodeCoachReport(baseOut)
			Expect(baseReport.ProjectCoverage).NotTo(BeNil())
			Expect(baseReport.ProjectCoverage.Complete).To(BeTrue(), "%+v", baseReport.ProjectCoverage)
			Expect(baseReport.ProjectChanges).To(HaveLen(1))

			stdout, stderr, exitCode := runCoachBaselineUnderUnshare(repo)
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "must not compare only by exit 0; coverage %+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].RuleID).To(Equal(baseReport.ProjectChanges[0].RuleID))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("importer", baseReport.ProjectChanges[0].MachineEvidence["importer"]))
			Expect(report.ProjectChanges[0].MachineEvidence).To(HaveKeyWithValue("importee", baseReport.ProjectChanges[0].MachineEvidence["importee"]))
		})
	})
})

func expectNamespacedDialFailsDistinctly() {
	listener, host, port := startNamespaceDialListener()
	DeferCleanup(listener.stop)
	mod := writeFakeCompilerDialModule(host, port)
	hitsBefore := len(listener.snapshot())
	stdout, stderr, err := runFakeCompilerDial(unsharePrefix(), mod)
	Expect(err).To(HaveOccurred(), "removing unshare must fail this When because the inside dial then succeeds; stdout=%s stderr=%s", stdout, stderr)
	msg := strings.ToUpper(stderr + stdout + err.Error())
	if listener.loopbackFallback {
		Expect(msg).To(Or(ContainSubstring("ECONNREFUSED"), ContainSubstring("ENETUNREACH"), ContainSubstring("EHOSTUNREACH")),
			"inside dial distinctive error; ECONNREFUSED is never the sole signal: outside hit is required too. stdout=%s stderr=%s", stdout, stderr)
		Expect(hitsBefore).To(Equal(0), "inside half must not be the only observation")
		stdout2, stderr2, err2 := runFakeCompilerDial(nil, mod)
		Expect(err2).NotTo(HaveOccurred(), "outside hit is required so ECONNREFUSED is not the sole signal; stdout=%s stderr=%s", stdout2, stderr2)
		Expect(listener.snapshot()).NotTo(BeEmpty())
		return
	}
	Expect(msg).To(Or(ContainSubstring("ENETUNREACH"), ContainSubstring("EHOSTUNREACH")),
		"preferred inside error is ENETUNREACH or EHOSTUNREACH, not ECONNREFUSED; stdout=%s stderr=%s", stdout, stderr)
	Expect(listener.snapshot()).To(HaveLen(hitsBefore), "inside namespace must not reach the outside listener")
}

func judgeConfinedAnalyzerSubtree(recs []straceRecord, id straceRecord, execPath, decoy, repo string, requireDecoy bool) {
	Expect(id.Pathname).To(Equal(execPath))
	Expect(argvHasPrefix(id.Argv, compilerModuleArgPrefix)).To(BeTrue())
	Expect(argvHasPrefix(id.Argv, nativePackageArgPrefix)).To(BeTrue())

	allow, subtree := linuxAnalyzerAllowlist(recs, id, execPath)
	assertLinuxAllowlistNotOpenListing(allow, decoy, repo)
	Expect(subtree).To(HaveKey(id.PID), "analyzer identity PID must be the clone-tree root")

	decoyHits, leaks := scanAnalyzerSubtreeProbes(recs, subtree, allow, decoy, repo, requireDecoy)
	if requireDecoy {
		Expect(decoyHits).NotTo(BeEmpty(), "decoy must appear in an open/stat/getdents line of an analyzer-subtree PID")
		return
	}
	Expect(leaks).To(BeEmpty(), "successful analyzer-subtree probes outside frozen allowlist:\n%s", strings.Join(leaks, "\n"))
	Expect(decoyHits).To(BeEmpty(), "confined analyzer-subtree trace must not contain the typeRoots decoy; hits: %v", decoyHits)
}
