package main

import (
	"bufio"
	"bytes"
	"context"
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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/tstestutil"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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

func codesignalArgsFromRemediationLine(stderr []byte) []string {
	line := strings.TrimSpace(string(stderr))
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

func pathValueFromEnviron(env string) string {
	for _, part := range strings.FieldsFunc(env, func(r rune) bool { return r == '\n' || r == '\x00' }) {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "PATH=") {
			return strings.TrimSpace(strings.TrimPrefix(part, "PATH="))
		}
		if i := strings.Index(part, "PATH="); i >= 0 {
			rest := part[i+5:]
			if j := strings.IndexAny(rest, " \t"); j >= 0 {
				return rest[:j]
			}
			return rest
		}
	}
	if i := strings.Index(env, "PATH="); i >= 0 {
		rest := env[i+5:]
		if j := strings.IndexAny(rest, " \n\t"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
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

const tsProjectTSConfigJSON = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`

const tsRealDbFile = "export const Name = 'db';\n"

const tsRealHandlersImportingDB = "import { Name } from \"../db/d\";\n\nexport function use(): string {\n  return Name;\n}\n"

// tsRealHandlersWithoutImport is the negative-control counterpart of
// tsRealHandlersImportingDB, used by no_findings_verdict_acceptance_test.go
// to build a "clean" fixture with no forbidden edge at all.
const tsRealHandlersWithoutImport = "export function use(): string {\n  return 'no import here';\n}\n"

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

const analyzerChildArgMarker = "--compiler-module="

type recordingProxyListener struct {
	addr string
	mu   sync.Mutex
	hits []string
	stop func()
}

func startRecordingProxyListener() *recordingProxyListener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())

	rec := &recordingProxyListener{addr: ln.Addr().String()}
	var inflight sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			inflight.Add(1)
			go func(c net.Conn) {
				defer inflight.Done()
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(2 * time.Second))
				line, _ := bufio.NewReader(c).ReadString('\n')
				rec.mu.Lock()
				rec.hits = append(rec.hits, strings.TrimSpace(line))
				rec.mu.Unlock()
			}(conn)
		}
	}()
	rec.stop = func() {
		_ = ln.Close()
		<-acceptDone
		inflight.Wait()
	}
	return rec
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

type analyzerEnvironSampler struct {
	stop  chan struct{}
	done  chan struct{}
	mu    sync.Mutex
	byPID map[int]string
}

func startAnalyzerEnvironSampler() *analyzerEnvironSampler {
	s := &analyzerEnvironSampler{
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		byPID: make(map[int]string),
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				s.capture()
				return
			case <-ticker.C:
				s.capture()
			}
		}
	}()
	return s
}

func (s *analyzerEnvironSampler) halt() map[int]string {
	close(s.stop)
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int]string, len(s.byPID))
	for pid, env := range s.byPID {
		out[pid] = env
	}
	return out
}

func (s *analyzerEnvironSampler) capture() {
	for _, pid := range analyzerChildPIDs() {
		env, ok := readProcessEnviron(pid)
		if !ok {
			continue
		}
		s.mu.Lock()
		s.byPID[pid] = env
		s.mu.Unlock()
	}
}

func analyzerChildPIDs() []int {
	if pids, ok := analyzerChildPIDsFromProc(); ok {
		return pids
	}
	return analyzerChildPIDsFromPS()
}

// analyzerChildPIDsFromProc restricts matches to descendants of this test
// binary's own process. go test ./... runs internal/codesignalcli and
// pkg/projectmodel acceptance suites concurrently, and they spawn their own
// analyzer children with the same --compiler-module= marker; without the
// ancestry check those foreign pids get counted alongside this package's,
// inflating the per-invocation counts these specs assert against.
func analyzerChildPIDsFromProc() ([]int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err != nil {
			continue
		}
		if !bytes.Contains(data, []byte(analyzerChildArgMarker)) {
			continue
		}
		if !isDescendantOfProcess(pid, self) {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, true
}

// isDescendantOfProcess reports whether pid's parent chain, read from
// /proc/<pid>/stat, reaches ancestor before hitting PID 1 or a read failure.
func isDescendantOfProcess(pid, ancestor int) bool {
	seen := make(map[int]bool)
	for {
		if pid == ancestor {
			return true
		}
		if pid <= 1 || seen[pid] {
			return false
		}
		seen[pid] = true
		ppid, ok := processParentPID(pid)
		if !ok {
			return false
		}
		pid = ppid
	}
}

// processParentPID reads a process's parent PID from /proc/<pid>/stat. The
// comm field can itself contain spaces and parentheses, so the parse anchors
// on the stat format's guaranteed last ')' rather than splitting on spaces.
func processParentPID(pid int) (int, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	idx := bytes.LastIndexByte(data, ')')
	if idx < 0 || idx+2 >= len(data) {
		return 0, false
	}
	fields := strings.Fields(string(data[idx+2:]))
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return ppid, true
}

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
		if strings.Contains(args, analyzerChildArgMarker) {
			candidates = append(candidates, pid)
		}
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

// isDescendantOfProcessTree is isDescendantOfProcess's variant for a
// pre-collected pid->ppid map, used where re-reading each ancestor's state
// (as /proc allows) is not available.
func isDescendantOfProcessTree(pid, ancestor int, parents map[int]int) bool {
	seen := make(map[int]bool)
	for {
		if pid == ancestor {
			return true
		}
		if pid <= 1 || seen[pid] {
			return false
		}
		seen[pid] = true
		ppid, ok := parents[pid]
		if !ok {
			return false
		}
		pid = ppid
	}
}

// readProcessEnviron reports a process's environment, or false when none
// could be observed. A successful read of zero bytes is not an observation:
// the kernel returns an empty environ for a task that has already torn down
// its address space, so a child that exits between being listed and being
// read would otherwise be recorded as having no PATH at all.
func readProcessEnviron(pid int) (string, bool) {
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ"); err == nil {
		if len(data) == 0 {
			return "", false
		}
		return strings.ReplaceAll(string(data), "\x00", "\n"), true
	}
	// Darwin: `ps -E -o command=` is argv only. BSD `ps eww` appends the
	// environment after the command so PATH= is observable.
	out, err := exec.Command("ps", "eww", "-p", strconv.Itoa(pid)).Output()
	if err != nil || !strings.Contains(string(out), "PATH=") {
		return "", false
	}
	return string(out), true
}

var _ = Describe("coach codesignal --project-language typescript against the private embedded analyzer and a confined, host-resolved compiler (coach#326 Task 3)", func() {
	When("the analyzed repository vendors no js/semantics analyzer anywhere and declares a real, exactly-matching installed TypeScript compiler", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_mismatch (.@5.4.0): run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
			Expect(strings.Count(strings.TrimSpace(string(stderr)), "\n")).To(Equal(0), "the refusal stays one line, got %q", stderr)
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_conflict (apps/web@7.0.2,apps/api@5.4.0): run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
			Expect(strings.Count(strings.TrimSpace(string(stderr)), "\n")).To(Equal(0), "the refusal stays one line, got %q", stderr)
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
			Expect(codesignalcli.NativeTypescriptPackageName()).To(Equal(nativeTypescriptPackageLookupName()))
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
			Expect(codesignalcli.NativeTypescriptPackageName()).To(Equal(packageName))

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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
		})
	})

	When("a declared unstable export subpath resolves to a file that does not exist (a broken exports entry, not a missing one)", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
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

func containsProjectModelDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) bool {
	for _, d := range diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

// countProjectModelDiagnosticCode asserts a model diagnostic is folded into
// the reported ProjectCoverage exactly once (see tsBypassCoverageForFold in
// internal/codesignalcli/project_ts_backend.go), not once per fold.
func countProjectModelDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) int {
	count := 0
	for _, d := range diagnostics {
		if d.Code == code {
			count++
		}
	}
	return count
}

// diagnosticMessageForKind returns the Message of the first report
// diagnostic matching kind, or "" if none matches.
func diagnosticMessageForKind(diagnostics []codesignal.Diagnostic, kind string) string {
	for _, d := range diagnostics {
		if d.Kind == kind {
			return d.Message
		}
	}
	return ""
}

func projectChangeRuleIDs(changes []codesignal.ProjectChange) map[string]bool {
	seen := map[string]bool{}
	for _, change := range changes {
		seen[change.RuleID] = true
	}
	return seen
}

// AC-12.
func assertReachabilityNeverSignalOrChange(report *codesignal.Report) {
	for _, signal := range report.Signals {
		Expect(signal.RuleID).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a Signal, got %+v", signal)
		Expect(signal.Kind).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a Signal, got %+v", signal)
	}
	for _, change := range report.ProjectChanges {
		Expect(change.RuleID).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a ProjectChange, got %+v", change)
		Expect(change.Kind).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a ProjectChange, got %+v", change)
	}
}

// splitTextFindingsAndFacts splits RenderText's output at its "\nFacts:\n"
// section marker (render.go's renderProjectFacts), so a spec can assert
// separately about the findings section (Signals + "Project findings:"
// ProjectChanges) and everything from "Facts:" onward: RenderText writes
// renderProjectFacts, renderDiagnosticsSection, renderCoverageSection, and
// renderProjectCoverageSection in that order with no further section
// markers this helper splits on, so factsSection is "Facts: through end of
// output", not ProjectFacts alone.
func splitTextFindingsAndFacts(text string) (findingsSection, factsSection string) {
	idx := strings.Index(text, "\nFacts:\n")
	ExpectWithOffset(1, idx).To(BeNumerically(">", 0), "expected a \"Facts:\" section in text output, got %q", text)
	return text[:idx], text[idx:]
}

// T7 (issue #331 Task 8): one analyzer response per revision must feed
// layer-violation, layer-bypass, and reachability-facts derivation alike,
// and incompleteness in each must fold into (or, for reachability, stay out
// of) the project-change lifecycle exactly as documented on
// tsProjectBackend.evaluateRevision (internal/codesignalcli/project_ts_backend.go).
var _ = Describe("coach codesignal --project-language typescript derives layer violations, layer bypass, and reachability facts from one analyzer response per revision (issue #331 Task 8 T7)", func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
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
			Expect(len(environs)).To(Equal(1), "expected exactly one analyzer invocation for a baseline analysis, observed pids: %+v", environs)

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
			Expect(len(textEnvirons)).To(Equal(1), "expected exactly one analyzer invocation for the text-format rendering of the same baseline analysis, observed pids: %+v", textEnvirons)

			findingsSection, factsSection := splitTextFindingsAndFacts(string(textStdout))
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_violation"), "got %q", findingsSection)
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_bypass"), "got %q", findingsSection)
			Expect(findingsSection).NotTo(ContainSubstring("possible_call_reachability"), "reachability must never appear in the Signals/ProjectChanges findings section, got %q", findingsSection)
			Expect(factsSection).To(ContainSubstring("kind: possible_call_reachability"), "got %q", factsSection)
			Expect(factsSection).NotTo(ContainSubstring("rule_id:"), "the Facts section must never carry a rule_id, which would make a fact indistinguishable from a Signal/ProjectChange, got %q", factsSection)

			// AC-11: a single-invocation ProjectBackendResult for the same
			// fixture must itself carry HeadChanges, Facts, project_scope, and
			// all three phase-coverage observations together, proving one
			// per-revision analyzer response backs all five jointly rather than
			// each being checked against a coincidentally-matching, separately
			// derived value.
			scopeSampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, tsLayerBypassRequiredConfigJSON)
			scopeEnvirons := scopeSampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(scopeEnvirons)).To(Equal(1), "expected exactly one analyzer invocation for the direct-result inspection of the same baseline analysis, observed pids: %+v", scopeEnvirons)

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
			Expect(len(environs)).To(Equal(2), "expected exactly one analyzer invocation per revision (head + base), observed pids: %+v", environs)

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
			Expect(len(textEnvirons)).To(Equal(2), "expected exactly one analyzer invocation per revision for the text-format rendering of the same diff, observed pids: %+v", textEnvirons)

			findingsSection, factsSection := splitTextFindingsAndFacts(string(textStdout))
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_violation"), "got %q", findingsSection)
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_bypass"), "got %q", findingsSection)
			Expect(findingsSection).NotTo(ContainSubstring("possible_call_reachability"), "reachability must never appear in the Signals/ProjectChanges findings section, got %q", findingsSection)
			Expect(factsSection).To(ContainSubstring("kind: possible_call_reachability"), "got %q", factsSection)
			Expect(factsSection).NotTo(ContainSubstring("rule_id:"), "the Facts section must never carry a rule_id, which would make a fact indistinguishable from a Signal/ProjectChange, got %q", factsSection)

			// AC-11: a single Analyze() call/ProjectBackendResult for the same
			// diff (2 analyzer invocations total, one per revision) must itself
			// carry HeadChanges, Facts, and both revisions' project_scope and
			// all three phase-coverage observations together.
			scopeSampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, baseSHA, false, tsLayerBypassRequiredConfigJSON)
			scopeEnvirons := scopeSampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(scopeEnvirons)).To(Equal(2), "expected exactly one analyzer invocation per revision for the direct-result inspection of the same diff, observed pids: %+v", scopeEnvirons)

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
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			// tsHandlersBypassFile gives this fixture a genuine bypass
			// candidate (a fully resolvable handler->sink path) that the
			// ambiguous required_layer must still suppress, unlike
			// commitRealTSLayerFixture alone, which has no sink/source pair
			// at all and would pass this spec's suppression assertions
			// vacuously regardless of whether suppression actually works.
			commitFile(repo, "pkg/handlers/bypass.ts", tsHandlersBypassFile)
			commitFile(repo, "project.json", tsLayerBypassAmbiguousConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse(), "an ambiguous, forced-incomplete bypass search must degrade the reported project coverage")

			Expect(report.ProjectChanges).NotTo(BeEmpty())
			for _, change := range report.ProjectChanges {
				Expect(string(change.Lifecycle)).To(Equal("unknown"), "a requested but incomplete bypass search must degrade every project-change lifecycle to unknown, got %+v", change)
			}

			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_layer_bypass_coverage_incomplete")))

			// AC-7/AC-17: an unresolved bypass search (ambiguous required
			// layer, so BuildTypeScriptLayerBypassFromModel's search never
			// reaches a fully-classified, LayerBypassConfidenceHigh witness --
			// the only confidence value the TS/Go backends ever produce, see
			// ts_layer_bypass.go's tsLayerBypassSearchFromSource (its
			// Confidence: LayerBypassConfidenceHigh assignment) -- must never
			// surface an architecture.layer_bypass entry.
			jsonRuleIDs := projectChangeRuleIDs(report.ProjectChanges)
			Expect(jsonRuleIDs).NotTo(HaveKey("architecture.layer_bypass"), "an unresolved bypass search must stay suppressed in JSON, got %+v", report.ProjectChanges)

			textStdout, textStderr, textExitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
			Expect(textExitCode).To(Equal(0), "stderr: %s stdout: %s", textStderr, textStdout)
			Expect(string(textStdout)).NotTo(ContainSubstring("architecture.layer_bypass"), "an unresolved bypass search must stay suppressed in text too, got %q", textStdout)
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
			commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark project coverage incomplete even when a bypass search ran, got %+v", report.ProjectCoverage)
			Expect(countProjectModelDiagnosticCode(report.ProjectCoverage.Diagnostics, "ts_reachability_local_call_not_followed_gap")).To(Equal(1), "expected the routine reachability gap diagnostic to be folded into ProjectCoverage exactly once, got %+v", report.ProjectCoverage.Diagnostics)

			ruleIDs := projectChangeRuleIDs(report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_violation"), "got %+v", report.ProjectChanges)
			for _, change := range report.ProjectChanges {
				if change.RuleID != "architecture.layer_violation" {
					continue
				}
				Expect(string(change.Lifecycle)).To(Equal("baseline"), "an unrelated reachability gap must never degrade an otherwise complete layer-violation finding's lifecycle when a bypass search also ran, got %+v", change)
			}
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

	// AC-24/AC-EVD-4: the mirror of the base-side case immediately above --
	// codesignal.projectLifecycleState checks input.ProjectCoverage (head)
	// and input.BaseProjectCoverage (base) in two separate conditions
	// (pkg/codesignal/codesignal.go), so proving indeterminacy from
	// head-side incompleteness alone exercises a distinct branch from the
	// base-side case, not a coincidentally-identical outcome from the same
	// condition.
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

			// Pins this spec to the head-side branch of projectLifecycleState
			// it claims to exercise, not the base side, which this fixture
			// commits complete with tsProjectTSConfigJSON before baseSHA:
			// projectLifecycleDiagnosticMessage (pkg/codesignal/codesignal.go)
			// only ever mentions "base coverage incomplete" when the base side
			// itself was incomplete.
			lifecycleMessage := diagnosticMessageForKind(report.Diagnostics, "project_lifecycle_indeterminate")
			Expect(lifecycleMessage).To(ContainSubstring("head coverage incomplete"), "expected the indeterminacy reason to name head coverage, got %q", lifecycleMessage)
			Expect(lifecycleMessage).NotTo(ContainSubstring("base coverage incomplete"), "the base revision is fully complete in this fixture; the indeterminacy reason must not blame it too, got %q", lifecycleMessage)
		})
	})
})

// analyzeTSProjectBackend calls the exported tsProjectBackend contract
// (NewTSProjectBackend/ProjectBackend.Analyze) directly, in-process, rather
// than through the compiled coach binary: project_scope is not yet rendered
// through codesignal.Input/Report (issue #332 Task 10's job, not Task 9
// T1's), so ProjectBackendResult -- the public contract at this boundary --
// is the most meaningful place to observe HeadProjectScope/BaseProjectScope.
// Calling Analyze in-process still spawns the real analyzer subprocess
// (BuildTypeScriptModelViaSidecar), and the analyzer child is still a
// descendant of this test binary, so startAnalyzerEnvironSampler's
// descendant-restricted PID scan observes it exactly as it would through the
// compiled binary.
func analyzeTSProjectBackend(dir, headRevision, baseRevision string, baseline bool, configJSON string) (*codesignalcli.ProjectBackendResult, error) {
	config := json.RawMessage(configJSON)
	backend := codesignalcli.NewTSProjectBackend()
	return backend.Analyze(context.Background(), codesignalcli.ProjectBackendRequest{
		Dir:          dir,
		HeadRevision: headRevision,
		BaseRevision: baseRevision,
		Baseline:     baseline,
		ConfigPath:   "project.json",
		Config:       config,
		ConfigDigest: codesignalcli.ConfigDigest(config),
		Language:     "typescript",
	})
}

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
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	When("a baseline analysis runs against a multi-root policy with a nested root", Label("ts-project-backend"), func() {
		It("derives HeadProjectScope with independent per-root candidate/analyzed counts, matched_layers, unmatched_layers, inclusion_rule, and pattern_set from one analyzer response (AC-3/AC-15/AC-25/AC-26)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/handlers/tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile)
			headSHA := commitFile(repo, "project.json", tsNestedRootsScopeConfigJSON)
			installRealTypescriptCompiler(repo, true)

			sampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, tsNestedRootsScopeConfigJSON)
			environs := sampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(environs)).To(Equal(1), "expected exactly one analyzer invocation for a baseline analysis (AC-RUN-5), observed pids: %+v", environs)

			Expect(result.HeadProjectScope).NotTo(BeNil())
			scope := *result.HeadProjectScope
			Expect(scope.InclusionRule).To(Equal(projectmodel.InclusionRuleTSConfigIncludesNoTestClassification))
			Expect(scope.PatternSet).To(Equal(projectmodel.TSReachabilityAlgorithm))
			Expect(scope.Roots).To(HaveLen(2), "got %+v", scope.Roots)

			byRoot := map[string]projectmodel.ProjectScopeRoot{}
			for _, r := range scope.Roots {
				byRoot[r.Root] = r
			}
			rootDot, ok := byRoot["."]
			Expect(ok).To(BeTrue(), "expected a root_scope entry for \".\", got %+v", scope.Roots)
			Expect(rootDot.CandidateFiles).To(Equal(3), "expected d.ts, h.ts, extra.ts under \".\", got %+v", rootDot)
			Expect(rootDot.AnalyzedFiles).To(Equal(3), "got %+v", rootDot)

			rootHandlers, ok := byRoot["pkg/handlers"]
			Expect(ok).To(BeTrue(), "expected a root_scope entry for pkg/handlers, got %+v", scope.Roots)
			Expect(rootHandlers.CandidateFiles).To(Equal(2), "expected h.ts, extra.ts under pkg/handlers, counted independently from \".\", got %+v", rootHandlers)
			Expect(rootHandlers.AnalyzedFiles).To(Equal(2), "got %+v", rootHandlers)

			Expect(scope.MatchedLayers).To(ConsistOf("handlers", "db"), "got %+v", scope.MatchedLayers)
			Expect(scope.UnmatchedLayers).To(ConsistOf("unused"), "a layer whose prefix matches no analyzed file must land in unmatched_layers, got %+v", scope.UnmatchedLayers)
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
			Expect(len(environs)).To(Equal(2), "expected exactly one analyzer invocation per revision (head + base), no second analyzer pass just to derive project_scope, observed pids: %+v", environs)

			Expect(result.HeadProjectScope).NotTo(BeNil(), "head-side project_scope must be carried")
			Expect(result.BaseProjectScope).NotTo(BeNil(), "base-side project_scope must be carried under --base")
			Expect(result.HeadProjectScope.Roots).To(HaveLen(2))
			Expect(result.BaseProjectScope.Roots).To(HaveLen(2))
		})
	})

	When("the tsRootScopeGapTSConfigJSON fixture accepts a candidate file into the compiler's Program that is never actually analyzed", Label("ts-project-backend"), func() {
		It("counts the unanalyzable candidate file in candidate_files but not analyzed_files, and names it in its own diagnostic (AC-5/AC-25)", func() {
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

			Expect(result.HeadProjectScope).NotTo(BeNil())
			Expect(result.HeadProjectScope.Roots).To(HaveLen(1))
			root := result.HeadProjectScope.Roots[0]
			Expect(root.Root).To(Equal("."))
			Expect(root.CandidateFiles).To(Equal(3), "expected package.json, d.ts, and h.ts as candidates, got %+v", root)
			Expect(root.AnalyzedFiles).To(Equal(2), "expected package.json to be counted as a candidate but never actually analyzed, got %+v", root)

			Expect(result.HeadCoverage).NotTo(BeNil())
			var found bool
			var message string
			for _, diag := range result.HeadCoverage.Diagnostics {
				if diag.Code == projectmodel.DiagRootScopeIncomplete {
					found = true
					message = diag.Message
				}
			}
			Expect(found).To(BeTrue(), "expected a %s diagnostic, got %+v", projectmodel.DiagRootScopeIncomplete, result.HeadCoverage.Diagnostics)
			Expect(message).To(ContainSubstring("package.json"), "the unanalyzable candidate file must be named in its own diagnostic, got %q", message)
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
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
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
