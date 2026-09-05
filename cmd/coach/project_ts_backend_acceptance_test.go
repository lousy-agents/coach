package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// repoRootFromThisFile locates the repository root relative to this test
// file's own path, mirroring pkg/projectmodel's own
// ts_sidecar_integration_acceptance_test.go convention.
func repoRootFromThisFile() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// npmArchName maps Go's runtime.GOARCH to the npm/Node process.arch naming
// convention typescript's native platform package directories use (e.g.
// "amd64" -> "x64"), which differs from Go's own arch names.
func npmArchName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return runtime.GOARCH
	}
}

func jsSemanticsRealTypescriptDir() string {
	return filepath.Join(repoRootFromThisFile(), "js", "semantics", "node_modules", "typescript")
}

func jsSemanticsRealNativeTypescriptDir() string {
	name := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
	return filepath.Join(repoRootFromThisFile(), "js", "semantics", "node_modules", "@typescript", name)
}

var (
	realTSCompilerOnce sync.Once
	realTSCompilerSkip string
)

// ensureRealTypeScriptCompilerAvailable memoizes whether this repository's
// own js/semantics/node_modules/typescript devDependency (plus its matching
// native platform package) is installed, so the specs below -- which copy
// that installed compiler into a fixture repository -- can skip gracefully
// in a Go-only CI leg (verify installs only Go, no `npm ci`) instead of
// failing hard. Mirrors
// pkg/projectmodel/ts_sidecar_integration_acceptance_test.go's
// ensureRealTSSidecarBinary contract.
func ensureRealTypeScriptCompilerAvailable() (skipReason string) {
	realTSCompilerOnce.Do(func() {
		if _, err := exec.LookPath("node"); err != nil {
			realTSCompilerSkip = fmt.Sprintf("node not found on PATH; skipping specs requiring the real installed TypeScript compiler (%s)", err)
			return
		}
		tsPkgJSON := filepath.Join(jsSemanticsRealTypescriptDir(), "package.json")
		if _, err := os.Stat(tsPkgJSON); err != nil {
			realTSCompilerSkip = fmt.Sprintf("%s not found (run `npm ci` in js/semantics); skipping specs requiring the real installed TypeScript compiler (%s)", tsPkgJSON, err)
			return
		}
		nativePkgJSON := filepath.Join(jsSemanticsRealNativeTypescriptDir(), "package.json")
		if _, err := os.Stat(nativePkgJSON); err != nil {
			realTSCompilerSkip = fmt.Sprintf("%s not found (run `npm ci` in js/semantics); skipping specs requiring the real installed TypeScript compiler (%s)", nativePkgJSON, err)
			return
		}
	})
	return realTSCompilerSkip
}

// realTypescriptVersion reads js/semantics' own installed typescript
// devDependency version directly from disk, so fixtures declaring an exact
// matching version in package.json cannot drift from the actual installed
// copy this suite copies.
func realTypescriptVersion() string {
	data, err := os.ReadFile(filepath.Join(jsSemanticsRealTypescriptDir(), "package.json"))
	Expect(err).NotTo(HaveOccurred())
	var manifest struct {
		Version string `json:"version"`
	}
	Expect(json.Unmarshal(data, &manifest)).To(Succeed())
	Expect(manifest.Version).NotTo(BeEmpty())
	return manifest.Version
}

func copyFileTree(src, dst string) {
	Expect(filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		mode := os.FileMode(0o644)
		if info, infoErr := d.Info(); infoErr == nil && info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		return os.WriteFile(target, content, mode)
	})).To(Succeed())
}

// installRealTypescriptCompiler copies this repository's own installed
// js/semantics node_modules/typescript devDependency into repo's own
// node_modules/typescript, uncommitted. PrepareTSRuntime's compiler
// resolution reads worktree filesystem state directly, never a Git
// snapshot (see project_ts_compiler_resolve.go's resolveCompilerForRuntime
// doc comment), so this need not be tracked by Git -- mirroring
// js/semantics's own setupAlternateCompiler test helper
// (js/semantics/test/project-sidecar.test.ts) one layer up, at the CLI
// boundary.
func installRealTypescriptCompiler(repo string, includeNativePackage bool) (packageDir string) {
	packageDir = filepath.Join(repo, "node_modules", "typescript")
	copyFileTree(jsSemanticsRealTypescriptDir(), packageDir)
	if includeNativePackage {
		nativeName := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
		nativeDest := filepath.Join(repo, "node_modules", "@typescript", nativeName)
		copyFileTree(jsSemanticsRealNativeTypescriptDir(), nativeDest)
	}
	return packageDir
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

// runCoachCodesignalBaselineEnv runs `coach codesignal --baseline` with a
// caller-controlled PATH (and no other inherited environment beyond HOME),
// mirroring project_readiness_acceptance_test.go's runCoachCheckProjectEnv
// one layer up at the full analysis boundary rather than --check-project
// alone: it lets a spec deterministically hide (or fake) node/mise on PATH
// regardless of what the host running this suite happens to have installed.
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

// analyzerChildArgMarker uniquely identifies the TypeScript analyzer child
// argv (see tsProjectBackend.evaluateRevision). Host probes such as
// `node --version` inherit the parent environ and must not be sampled:
// treating them as the analyzer is an AC-RUN-2 false-red.
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
	out, err := exec.Command("ps", "-wwE", "-p", strconv.Itoa(pid)).Output()
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

	When("the parent process has NODE_OPTIONS=--require pointing at a spy and HTTP(S)_PROXY pointing at a recording listener", Label("ts-project-backend"), func() {
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --check-project --project-language typescript --project-config project.json"))
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
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --check-project --project-language typescript --project-config project.json"))
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
			if _, err := os.Stat(filepath.Join(miseDir, stubMiseCwdLog)); err == nil {
				for _, cwd := range readStubMiseCwds(miseDir) {
					Expect(cwd).NotTo(Equal(repo), "mise probes must not run with the analyzed repository as cwd, got %q", cwd)
				}
			}
		})
	})

	When("the resolved compiler is missing its required native platform package", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
				Skip(reason)
			}
		})

		It("reports the native-backend startup failure specifically, distinguishing it from the pre-#326 generic sidecar-unavailable degrade, without leaking the compiler's absolute path", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			compilerDir := installRealTypescriptCompiler(repo, false)

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
			Expect(message).To(ContainSubstring("failed to start ts sidecar analysis backend"), "expected the native-platform-package startup failure to surface, got: %s", message)

			Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
			Expect(message).NotTo(ContainSubstring(repo), "diagnostic must not contain the repository path")
			Expect(message).NotTo(ContainSubstring("coach-ts-analyzer-"), "diagnostic must not contain the analyzer temp-directory prefix")
			Expect(message).NotTo(ContainSubstring("file://"))
			Expect(message).NotTo(ContainSubstring("node:internal"))
		})

		It("does not embed the native compiler executable path when that executable is missing from an otherwise resolved compiler", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			compilerDir := installRealTypescriptCompiler(repo, true)
			nativeName := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
			nativeDest := filepath.Join(repo, "node_modules", "@typescript", nativeName)
			Expect(os.Remove(filepath.Join(nativeDest, "lib", "tsc"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			Expect(string(stdout)).NotTo(ContainSubstring(compilerDir), "the resolved compiler's absolute filesystem path must never appear in the serialized report")
			Expect(string(stdout)).NotTo(ContainSubstring(nativeDest), "the native compiler executable path must never appear in the serialized report")
		})

		It("renders a qualified verdict, not the unqualified clean-run sentence", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitRealTSLayerFixture(repo, version)
			installRealTypescriptCompiler(repo, false)

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
})
