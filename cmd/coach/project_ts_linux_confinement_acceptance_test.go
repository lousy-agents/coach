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
