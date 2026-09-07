package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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
	args := append([]string{"codesignal", "--baseline"}, extraArgs...)
	command := exec.Command(commandPath, args...)
	command.Dir = repo
	command.Env = []string{"PATH=" + path, "HOME=" + os.Getenv("HOME")}
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	err := command.Run()
	if err == nil {
		return outBuf.Bytes(), errBuf.Bytes(), 0
	}

	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
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

func analyzerChildPIDsFromProc() ([]int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
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
		if bytes.Contains(data, []byte(analyzerChildArgMarker)) {
			pids = append(pids, pid)
		}
	}
	return pids, true
}

func analyzerChildPIDsFromPS() []int {
	out, err := exec.Command("ps", "-axww", "-o", "pid=,args=").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, analyzerChildArgMarker) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

func readProcessEnviron(pid int) (string, bool) {
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ"); err == nil {
		return strings.ReplaceAll(string(data), "\x00", "\n"), true
	}
	out, err := exec.Command("ps", "-wwwE", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
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
		It("exits 2 with empty stdout and one stderr line naming typescript_version_mismatch and the --check-project invocation", func() {
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_mismatch: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
		})
	})

	When("two selected roots pin disagreeing exact typescript versions", func() {
		It("exits 2 with empty stdout and one stderr line naming typescript_version_conflict and the --check-project invocation", func() {
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_conflict: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
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
		It("exits 2 with empty stdout and node_missing, not node_unsupported or node_below_minimum", func() {
			repo := newTempGitRepo()
			commitNativePackageGapFixture(repo)
			writeInstalledNativeTypescript(repo, "7.0.2")

			path := pathWithStubNode("v25.0.0")
			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
			Expect(strings.TrimSpace(string(stderr))).To(Equal("node_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(string(stderr)).NotTo(ContainSubstring("node_unsupported"))
			Expect(string(stderr)).NotTo(ContainSubstring("node_below_minimum"))
			Expect(string(stderr)).NotTo(ContainSubstring("25"))
			Expect(string(stderr)).NotTo(ContainSubstring("{24, 26}"))
		})

		It("still accepts the same stub under --check-project as node_untested and does not emit node_unsupported", func() {
			repo := newTempGitRepo()
			commitNativePackageGapFixture(repo)
			writeInstalledNativeTypescript(repo, "7.0.2")

			path := pathWithStubNode("v25.0.0")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(string(stdout)).To(ContainSubstring("node_untested"))
			Expect(string(stdout)).NotTo(ContainSubstring("node_unsupported"))
			Expect(string(stdout)).NotTo(ContainSubstring("node_missing"))
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
