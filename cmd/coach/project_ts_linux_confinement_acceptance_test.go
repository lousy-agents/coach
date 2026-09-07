package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

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

func newLinuxTypeRootsFixture() (repo, decoy string) {
	repo = newTempGitRepo()
	decoyDir, err := os.MkdirTemp("", "coach-d2-typeroots-decoy-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, decoyDir)
	Expect(os.MkdirAll(filepath.Join(decoyDir, "child"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(decoyDir, "child", "index.d.ts"), []byte("export {};\n"), 0o644)).To(Succeed())
	decoy = decoyDir

	version := realTypescriptVersion()
	tsconfig := fmt.Sprintf(`{"compilerOptions":{"module":"commonjs","moduleResolution":"node10","typeRoots":[%q],"types":["child"]}}`, filepath.ToSlash(decoy))
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsconfig+"\n")
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	installRealTypescriptCompiler(repo, true)
	return repo, decoy
}

func probedNodeExecPath() string {
	out, err := exec.Command("node", "-p", "process.execPath").Output()
	Expect(err).NotTo(HaveOccurred())
	path := strings.TrimSpace(string(out))
	Expect(filepath.IsAbs(path)).To(BeTrue(), "probed ExecPath must be absolute, got %q", path)
	return path
}

func ensureFileSyscallTracer() {
	if _, err := exec.LookPath("strace"); err == nil {
		return
	}
	_ = exec.Command("sudo", "apt-get", "update").Run()
	if err := exec.Command("sudo", "apt-get", "install", "-y", "strace").Run(); err != nil {
		Fail(fileSyscallTracerUnavailable)
	}
	if _, err := exec.LookPath("strace"); err != nil {
		Fail(fileSyscallTracerUnavailable)
	}
}

func runCoachBaselineUnderStrace(repo, coachPath, traceFile string) (stdout, stderr []byte, exitCode int) {
	args := []string{
		"-f",
		"-s", "65535",
		"-y",
		"-e", "trace=" + linuxStraceTraceExpr(),
		"-o", traceFile,
		"--",
		coachPath,
		"codesignal", "--baseline",
		"--project-config", "project.json",
		"--project-language", "typescript",
		"--format=json",
	}
	cmd := exec.Command("strace", args...)
	cmd.Dir = repo
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = []byte(outBuf.String())
	stderr = []byte(errBuf.String())
	if err == nil {
		return stdout, stderr, 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return stdout, stderr, exitErr.ExitCode()
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

func linuxAnalyzerAllowlist(recs []straceRecord, id straceRecord, execPath string) (linuxAllowlist, map[int]struct{}) {
	compilerDir := argvValue(id.Argv, compilerModuleArgPrefix)
	nativeDir := argvValue(id.Argv, nativePackageArgPrefix)
	shim := ""
	if len(id.Argv) > 1 {
		shim = id.Argv[1]
	}
	Expect(shim).NotTo(BeEmpty(), "analyzer shim path missing from identity argv")
	allow := frozenLinuxAllowlist(filepath.Dir(execPath), compilerDir, nativeDir, filepath.Dir(shim), id.PID)
	subtree := analyzerSubtreePIDs(recs, id.PID)
	for pid := range subtree {
		allow.prefixes = append(allow.prefixes, fmt.Sprintf("/proc/%d", pid))
	}
	return allow, subtree
}

func assertLinuxAllowlistNotOpenListing(allow linuxAllowlist, decoy, repo string) {
	Expect(linuxPathAllowed(allow, decoy)).To(BeFalse(), "decoy and HOME are asserted not in the frozen allowlist before judgment")
	if home := os.Getenv("HOME"); home != "" {
		Expect(linuxPathAllowed(allow, home)).To(BeFalse(), "decoy and HOME are asserted not in the frozen allowlist before judgment")
	}
	if repo == "" {
		return
	}
	Expect(linuxPathAllowed(allow, repo)).To(BeFalse(), "fixture repository root is not allowed for opens and listings")
	Expect(linuxPathAllowed(allow, filepath.Join(repo, "node_modules"))).To(BeFalse(), "fixture repository node_modules is not allowed for opens and listings")
}

func scanAnalyzerSubtreeProbes(recs []straceRecord, subtree map[int]struct{}, allow linuxAllowlist, decoy, repo string, requireDecoy bool) (decoyHits, leaks []string) {
	for _, rec := range recs {
		if _, in := subtree[rec.PID]; !in {
			continue
		}
		decoyHits = append(decoyHits, decoyHitsOn(rec, decoy)...)
		if requireDecoy {
			continue
		}
		if _, mut := linuxMutationSyscalls[rec.Syscall]; mut {
			Fail(fmt.Sprintf("mutation syscall in confined analyzer subtree: %s", rec.Raw))
		}
		leaks = append(leaks, probeLeaksOn(allow, rec, repo)...)
	}
	return decoyHits, leaks
}

func decoyHitsOn(rec straceRecord, decoy string) []string {
	var hits []string
	for _, p := range rec.Paths {
		if pathHasPrefix(p, decoy) {
			hits = append(hits, rec.Raw)
		}
	}
	return hits
}

func probeLeaksOn(allow linuxAllowlist, rec straceRecord, repo string) []string {
	if _, probe := linuxProbeSyscalls[rec.Syscall]; !probe {
		return nil
	}
	if !straceSucceeded(rec) {
		return nil
	}
	var leaks []string
	for _, p := range rec.Paths {
		if linuxProbeAllowed(allow, rec.Syscall, p) {
			continue
		}
		failIfRepoOpenOrListing(rec, repo, p)
		leaks = append(leaks, fmt.Sprintf("%s (path %s)", rec.Raw, p))
	}
	return leaks
}

func failIfRepoOpenOrListing(rec straceRecord, repo, path string) {
	_, openOrListing := linuxOpenOrListingSyscalls[rec.Syscall]
	if repo != "" && openOrListing && pathHasPrefix(path, repo) {
		Fail(fmt.Sprintf("successful open or listing under the fixture repository root; do not add an allowlist entry: %s (path %s)", rec.Raw, path))
	}
}

func pathHasPrefix(path, prefix string) bool {
	clean := filepath.Clean(path)
	p := filepath.Clean(prefix)
	return clean == p || strings.HasPrefix(clean, p+string(os.PathSeparator))
}

func buildUnconfinedAnalyzerCoach() string {
	root := repositoryRoot()
	tmp, err := os.MkdirTemp("", "coach-unconfined-analyzer-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, tmp)

	vfsSrc := filepath.Join(root, "internal", "codesignalcli", "tsanalyzerasset", "project-sidecar", "vfs.js")
	analyzeSrc := filepath.Join(root, "internal", "codesignalcli", "tsanalyzerasset", "project-sidecar", "analyze.js")
	runtimeSrc := filepath.Join(root, "internal", "codesignalcli", "project_ts_runtime.go")

	vfsDst := filepath.Join(tmp, "vfs.js")
	analyzeDst := filepath.Join(tmp, "analyze.js")
	runtimeDst := filepath.Join(tmp, "project_ts_runtime.go")

	vfs, err := os.ReadFile(vfsSrc)
	Expect(err).NotTo(HaveOccurred())
	listingBlock := `        getAccessibleEntries: (directoryName) => {
            const result = base.getAccessibleEntries ? base.getAccessibleEntries(directoryName) : undefined;
            return result === undefined ? { files: [], directories: [] } : result;
        },`
	hostListing := `        getAccessibleEntries: (directoryName) => {
            const result = base.getAccessibleEntries ? base.getAccessibleEntries(directoryName) : undefined;
            if (result !== undefined) return result;
            try {
                const names = readdirSync(directoryName, { withFileTypes: true });
                return {
                    files: names.filter((d) => d.isFile() || d.isSymbolicLink()).map((d) => d.name),
                    directories: names.filter((d) => d.isDirectory()).map((d) => d.name),
                };
            } catch {
                return { files: [], directories: [] };
            }
        },`
	patchedVFS := "import { readdirSync } from \"node:fs\";\n" + strings.Replace(string(vfs), listingBlock, hostListing, 1)
	Expect(patchedVFS).NotTo(Equal(string(vfs)), "unconfined analyzer must restore host filesystem listing fall-through")
	Expect(strings.Contains(patchedVFS, listingBlock)).To(BeFalse(), "confined empty-listing wrapper must be gone")
	Expect(os.WriteFile(vfsDst, []byte(patchedVFS), 0o644)).To(Succeed())

	analyze, err := os.ReadFile(analyzeSrc)
	Expect(err).NotTo(HaveOccurred())
	patchedAnalyze := strings.Replace(string(analyze), "return new ApiCtor({ fs: snapshot.fs, tsserverPath });", "return new ApiCtor({ fs: snapshot.fs });", 1)
	Expect(patchedAnalyze).NotTo(Equal(string(analyze)), "unconfined analyzer must omit tsserverPath")
	snapshotLine := "    const snapshot = buildProjectSnapshot(opts.files, opts.compiler.createVirtualFileSystem);\n"
	typeRootsWalk := snapshotLine + `    for (const f of opts.files) {
        if (!f.path.endsWith("tsconfig.json")) continue;
        try {
            const cfg = JSON.parse(Buffer.from(f.content_b64, "base64").toString("utf8"));
            for (const root of cfg.compilerOptions?.typeRoots ?? []) {
                snapshot.fs.getAccessibleEntries?.(root);
            }
        } catch { }
    }
`
	Expect(strings.Contains(patchedAnalyze, snapshotLine)).To(BeTrue())
	patchedAnalyze = strings.Replace(patchedAnalyze, snapshotLine, typeRootsWalk, 1)
	Expect(os.WriteFile(analyzeDst, []byte(patchedAnalyze), 0o644)).To(Succeed())

	runtimeBytes, err := os.ReadFile(runtimeSrc)
	Expect(err).NotTo(HaveOccurred())
	patchedRuntime := strings.Replace(string(runtimeBytes),
		`"--native-package=" + compiler.NativePackagePath,`,
		`"--native-package=" + compiler.NativePackagePath,`+"\n\t\t\""+unconfinedArgvHook+`",`,
		1)
	Expect(patchedRuntime).NotTo(Equal(string(runtimeBytes)), "throwaway must spawn through the go test-only argv hook")
	Expect(os.WriteFile(runtimeDst, []byte(patchedRuntime), 0o644)).To(Succeed())

	overlay := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{
		vfsSrc:     vfsDst,
		analyzeSrc: analyzeDst,
		runtimeSrc: runtimeDst,
	}}
	overlayPath := filepath.Join(tmp, "overlay.json")
	payload, err := json.Marshal(overlay)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(overlayPath, payload, 0o644)).To(Succeed())

	bin := filepath.Join(tmp, "coach")
	build := exec.Command("go", "build", "-a", "-overlay", overlayPath, "-o", bin, ".")
	build.Dir = filepath.Join(root, "cmd", "coach")
	out, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building throwaway unconfined analyzer: %s", out)
	Expect(bin).NotTo(HavePrefix(filepath.Join(root, "dist")), "throwaway must not be a GoReleaser dist path")
	Expect(bin).NotTo(ContainSubstring("tsanalyzerasset"), "throwaway must not be the checked-in embed")
	return bin
}

type namespaceDialListener struct {
	hits             []string
	mu               sync.Mutex
	stop             func()
	loopbackFallback bool
}

func (l *namespaceDialListener) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.hits))
	copy(out, l.hits)
	return out
}

func startNamespaceDialListener() (*namespaceDialListener, string, int) {
	rec := &namespaceDialListener{}
	host, ok := nonLoopbackIPv4()
	addr := host + ":0"
	if !ok {
		addr = "127.0.0.1:0"
		rec.loopbackFallback = true
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil && !rec.loopbackFallback {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		rec.loopbackFallback = true
	}
	Expect(err).NotTo(HaveOccurred())
	tcpAddr := ln.Addr().(*net.TCPAddr)
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
				rec.mu.Lock()
				rec.hits = append(rec.hits, c.RemoteAddr().String())
				rec.mu.Unlock()
				_ = c.SetDeadline(time.Now().Add(2 * time.Second))
				_, _ = io.Copy(io.Discard, c)
			}(conn)
		}
	}()
	rec.stop = func() {
		_ = ln.Close()
		<-acceptDone
		inflight.Wait()
	}
	return rec, tcpAddr.IP.String(), tcpAddr.Port
}

func nonLoopbackIPv4() (string, bool) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", false
	}
	for _, iface := range ifaces {
		if ip, ok := ifaceNonLoopbackIPv4(iface); ok {
			return ip, true
		}
	}
	return "", false
}

func ifaceNonLoopbackIPv4(iface net.Interface) (string, bool) {
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
		return "", false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", false
	}
	return firstNonLoopbackIPv4(addrs)
}

func firstNonLoopbackIPv4(addrs []net.Addr) (string, bool) {
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil {
			continue
		}
		return ip.String(), true
	}
	return "", false
}

func writeFakeCompilerDialModule(host string, port int) string {
	dir, err := os.MkdirTemp("", "coach-d2-fake-compiler-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	dial := fmt.Sprintf("import net from 'node:net';\ntry {\n  await new Promise((resolve, reject) => {\n    const socket = net.connect({host:%q, port:%d}, () => { socket.end(); resolve(); });\n    socket.on('error', reject);\n  });\n} catch (err) {\n  console.error(err.code || err.message);\n  process.exitCode = 1;\n}\n", host, port)
	Expect(os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"fake-ts-compiler","type":"module","main":"dial.js"}`+"\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(dir, "dial.js"), []byte(dial), 0o644)).To(Succeed())
	return dir
}

func fakeCompilerLoader() string {
	dir, err := os.MkdirTemp("", "coach-d2-fake-loader-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	src := "import { pathToFileURL } from 'node:url';\nconst flag = process.argv.find((a)=>a.startsWith('--compiler-module='));\nif (!flag) { process.stderr.write('missing --compiler-module=\\n'); process.exit(2); }\nawait import(pathToFileURL(flag.slice('--compiler-module='.length)+'/dial.js').href);\n"
	path := filepath.Join(dir, "load.mjs")
	Expect(os.WriteFile(path, []byte(src), 0o644)).To(Succeed())
	return path
}

func runFakeCompilerDial(prefix []string, moduleDir string) (stdout, stderr string, err error) {
	loader := fakeCompilerLoader()
	args := append(append([]string{}, prefix...), probedNodeExecPath(), loader, "--compiler-module="+moduleDir)
	cmd := exec.Command(args[0], args[1:]...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func unsharePrefix() []string {
	return append([]string{}, namespaceUnsharePrefix...)
}

var namespaceUnsharePrefix []string

func writeUnshareGitHome() string {
	home, err := os.MkdirTemp("", "coach-unshare-home-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, home)
	Expect(os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[safe]\n\tdirectory = *\n"), 0o644)).To(Succeed())
	return home
}

func runCoachBaselineUnderUnshare(repo string) (stdout, stderr []byte, exitCode int) {
	prefix := append(append([]string{}, unsharePrefix()...), unshareEnvWrapper(unsharePathEnv(probedNodeExecPath(), os.Getenv("PATH")), writeUnshareGitHome(), os.Getenv("TMPDIR"))...)
	args := append(append([]string{}, prefix...), commandPath, "codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repo
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = []byte(outBuf.String())
	stderr = []byte(errBuf.String())
	if err == nil {
		return stdout, stderr, 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return stdout, stderr, exitErr.ExitCode()
}

func ensureUnshareAvailable() {
	if _, err := exec.LookPath("unshare"); err != nil {
		Fail(unshareUnavailable)
	}
	if exec.Command("unshare", "-rn", "--", "true").Run() == nil {
		namespaceUnsharePrefix = []string{"unshare", "-rn", "--"}
		return
	}
	if exec.Command("sudo", "unshare", "-n", "--", "true").Run() == nil {
		namespaceUnsharePrefix = []string{"sudo", "unshare", "-n", "--"}
		return
	}
	Fail(unshareUnavailable)
}
