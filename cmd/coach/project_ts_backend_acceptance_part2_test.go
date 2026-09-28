package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/tstestutil"
)

// analyzerChildPIDsFromPS is the non-/proc fallback (e.g. Darwin), applying
// the same ancestry restriction as analyzerChildPIDsFromProc via ppid=.
func analyzerChildPIDsFromPS() []int {
	out, err := exec.Command("ps", "-axww", "-o", "pid=,ppid=,args=").Output()
	if err != nil {
		return nil
	}
	parents := make(map[int]int)
	var candidates []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		parents[pid] = ppid
		args := strings.Join(fields[2:], " ")
		(&siganalyzerChildPIDsFromPSS10{args: args, candidates: &candidates, pid: pid}).call()

	}
	self := os.Getpid()
	var pids []int
	for _, pid := range candidates {
		if isDescendantOfProcessTree(pid, self, parents) {
			pids = append(pids, pid)
		}
	}
	return pids
}

func startRecordingProxyListener() *recordingProxyListener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())

	rec := &recordingProxyListener{addr: ln.Addr().String()}
	var inflight sync.WaitGroup
	acceptDone := make(chan struct{})
	go (&sigstartRecordingProxyListener39957725{acceptDone: acceptDone, inflight: inflight, ln: ln, rec: rec}).call()
	rec.stop = func() {
		_ = ln.Close()
		<-acceptDone
		inflight.Wait()
	}
	return rec
}

func npmArchName() string {
	return tstestutil.NPMArch()
}

func ensureRealTypeScriptCompilerAvailable() string {
	return tstestutil.EnsureTypeScriptCompilerAvailable()
}

func realTypescriptVersion() string {
	return tstestutil.TypeScriptVersion()
}

func installRealTypescriptCompiler(repo string, includeNativePackage bool) string {
	return tstestutil.InstallTypeScriptCompiler(repo, includeNativePackage)
}

// breakCompilerExportSubpath removes subpath from packageDir/package.json's
// exports map, mirroring js/semantics's own setupAlternateCompiler
// removeExportSubpath option: it reproduces a resolved compiler whose
// package.json no longer declares one of the "./unstable/*" exports the
// analyzer's loadCompiler requires.
func breakCompilerExportSubpath(packageDir, subpath string) {
	pkgPath := filepath.Join(packageDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	Expect(err).NotTo(HaveOccurred())
	var pkg map[string]any
	Expect(json.Unmarshal(data, &pkg)).To(Succeed())
	exportsField, ok := pkg["exports"].(map[string]any)
	Expect(ok).To(BeTrue(), "expected an exports map in %s", pkgPath)
	_, has := exportsField[subpath]
	Expect(has).To(BeTrue(), "expected %q in %s exports", subpath, pkgPath)
	delete(exportsField, subpath)
	out, err := json.MarshalIndent(pkg, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(pkgPath, out, 0o644)).To(Succeed())
}

// repointCompilerExportSubpath rewrites packageDir/package.json's exports
// map so subpath resolves to target -- a realistic partially-installed or
// pruned compiler (e.g. `npm prune`-style tree shaking that removed a file
// an exports entry still names), as distinct from breakCompilerExportSubpath
// above, which removes the exports entry entirely. Node's own module
// resolution failure for the missing target embeds target's own resolved
// absolute path in its Error.message, which is the round-2 leak vector this
// spec's compilerDir assertion guards against.
func repointCompilerExportSubpath(packageDir, subpath, target string) {
	pkgPath := filepath.Join(packageDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	Expect(err).NotTo(HaveOccurred())
	var pkg map[string]any
	Expect(json.Unmarshal(data, &pkg)).To(Succeed())
	exportsField, ok := pkg["exports"].(map[string]any)
	Expect(ok).To(BeTrue(), "expected an exports map in %s", pkgPath)
	_, has := exportsField[subpath]
	Expect(has).To(BeTrue(), "expected %q in %s exports", subpath, pkgPath)
	exportsField[subpath] = target
	out, err := json.MarshalIndent(pkg, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(pkgPath, out, 0o644)).To(Succeed())
}

func runCoachCodesignalBaselineEnv(repo, path string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(commandPath, repo, stubToolchainEnv(path), append([]string{"codesignal", "--baseline"}, extraArgs...)...)
}

// codesignalArgsFromRemediationLine reads only stderr's first line: since
// AC-SET-9 (#330), a no-controlling-terminal scan appends a second
// interactive-setup line after the fit-check invocation this parses.
func codesignalArgsFromRemediationLine(stderr []byte) []string {
	firstLine, _, _ := strings.Cut(string(stderr), "\n")
	line := strings.TrimSpace(firstLine)
	_, invocation, found := strings.Cut(line, ": run ")
	Expect(found).To(BeTrue(), "stderr must print a runnable fit-check invocation, got %q", line)
	fields := strings.Fields(invocation)
	Expect(len(fields)).To(BeNumerically(">=", 3), "printed invocation must be coach codesignal <flags>, got %q", invocation)
	Expect(fields[0]).To(Equal("coach"), "printed invocation must start with coach so a customer can run it unchanged, got %q", invocation)
	Expect(fields[1]).To(Equal("codesignal"), "printed invocation must invoke codesignal, got %q", invocation)
	return fields[2:]
}

func nativeTypescriptPackageLookupName() string {
	return fmt.Sprintf("@typescript/typescript-%s-%s", runtime.GOOS, npmArchName())
}

func writeInstalledNativeTypescript(repo, version string) {
	writeInstalledNativeTypescriptUnder(repo, ".", version)
}

func commitNativePackageGapFixture(repo string) {
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
	writeInstalledTypescriptCompilerOnly(repo, "7.0.2")
}

func wrapExecutableWithMarker(exe, marker string) {
	real := exe + ".real"
	Expect(os.Rename(exe, real)).To(Succeed())
	script := fmt.Sprintf("#!/bin/sh\nprintf planted > %q\nexec %q \"$@\"\n", marker, real)
	Expect(os.WriteFile(exe, []byte(script), 0o755)).To(Succeed())
}

func plantCanaryExecutable(exe, marker string) {
	Expect(os.MkdirAll(filepath.Dir(exe), 0o755)).To(Succeed())
	script := fmt.Sprintf("#!/bin/sh\nprintf canary > %q\nexit 1\n", marker)
	Expect(os.WriteFile(exe, []byte(script), 0o755)).To(Succeed())
}

// expectTypescriptCompilerMissingScan's fixtures declare no package manager
// and no mise scope at all, so readiness's own AvailableSetupChoices menu
// offers nothing installable: the appended --prepare-compiler command is
// correctly withheld (O2) rather than naming a command that would only open
// to report it had nothing to do.
func expectTypescriptCompilerMissingScan(stdout, stderr []byte, exitCode int) {
	Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
	Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
	Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
	Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
	Expect(string(stderr)).NotTo(ContainSubstring("@typescript/"))
}

func tsRealCompilerPackageJSON(version string) string {
	return fmt.Sprintf(`{"devDependencies":{"typescript":%q}}`, version)
}

// commitRealTSLayerFixture commits the shared handlers/db/policy fixture
// every spec below builds on: an actual value-level import from
// pkg/handlers into pkg/db, forbidden by goLayerPolicyConfigJSON
// (project_go_backend_acceptance_test.go). version is committed into
// package.json so the "project" compiler-resolution origin's manifest/
// installed-version match succeeds once installRealTypescriptCompiler
// copies the matching compiler onto disk.
func commitRealTSLayerFixture(repo, version string) {
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
}

func (r *recordingProxyListener) proxyURL() string {
	return "http://" + r.addr
}

func (r *recordingProxyListener) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.hits))
	copy(out, r.hits)
	return out
}

func (s *analyzerEnvironSampler) invocations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
}

// splitTextFindingsAndFacts splits RenderText's output at its "\nFacts:\n"
// section marker (render.go's renderProjectFacts), so a spec can assert
// separately about the findings section (Signals + "Project findings:"
// ProjectChanges) and everything from "Facts:" onward: RenderText writes
// renderProjectFacts, renderDiagnosticsSection, renderCoverageSection, and
// renderProjectCoverageSection in that order with no further section
// markers this helper splits on, so factsSection is "Facts: through end of
// output", not ProjectFacts alone.
//
// Callers pair this with a JSON-decoded assertion on the same fixture first:
// the JSON checks are the structural source of truth, and the text-format
// checks this helper supports only confirm the text renderer doesn't
// diverge from what JSON already proved, not an independent proof.
func splitTextFindingsAndFacts(text string) (findingsSection, factsSection string) {
	idx := strings.Index(text, "\nFacts:\n")
	ExpectWithOffset(1, idx).To(BeNumerically(">", 0), "expected a \"Facts:\" section in text output, got %q", text)
	return text[:idx], text[idx:]
}
