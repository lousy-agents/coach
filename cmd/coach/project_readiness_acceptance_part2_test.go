package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// miseInvocationsIncludeInstall reports whether any invocation logged at
// miseDir began with "install". Unlike readStubMiseInvocations, a missing
// log file (no invocation at all yet) is not a test failure here -- it
// simply means no install happened, which is exactly what a "no mutation"
// assertion needs to tolerate.
func miseInvocationsIncludeInstall(miseDir string) bool {
	data, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "install ") {
			return true
		}
	}
	return false
}

func writeInstalledTypescriptCompilerUnder(repo, relDir, version string) {
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); err != nil {
		commitFile(repo, ".gitignore", "node_modules\n")
	}
	manifest := "node_modules/typescript/package.json"
	if relDir != "." && relDir != "" {
		manifest = relDir + "/" + manifest
	}
	writeWorktreeFile(repo, manifest, fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version))
}

func gapCodes(doc readinessResultDoc) []string {
	codes := make([]string, len(doc.Gaps))
	for i, g := range doc.Gaps {
		codes[i] = g.Code
	}
	return codes
}

func nextActionKinds(doc readinessResultDoc) []string {
	kinds := make([]string, len(doc.NextActions))
	for i, a := range doc.NextActions {
		kinds[i] = a.Kind
	}
	return kinds
}

func writeInstalledNativeTypescriptUnder(repo, relDir, version string) {
	unscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
	manifest := filepath.Join("node_modules", "@typescript", unscoped, "package.json")
	if relDir != "." && relDir != "" {
		manifest = relDir + "/" + manifest
	}
	writeWorktreeFile(repo, manifest, fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+unscoped, version))
}

// writeStubNodeScript writes an executable `node` script into a fresh temp
// directory that always prints version regardless of its arguments, and
// returns that directory. checkNodeReadiness's detectHostNodeMajor shells
// out to whatever `node` is first on the child process's PATH, so a spec
// that wants a specific, host-independent Node major must control PATH with
// a stub rather than depend on whatever Node happens to be installed.
func writeStubNodeScript(version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo %s; exit 0; fi\nif [ \"$1\" = \"-p\" ]; then echo \"$0\"; exit 0; fi\necho %s\n", version, version)
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathExcludingToolchain strips every real node/npm/mise directory, so a spec
// on this PATH has no global-mise candidate however the host is configured.
// The package managers are stripped alongside them because
// checkPackageManager probes whichever npm/pnpm/bun/yarn the child can
// resolve: leaving the host's own installation reachable would make a
// package-manager classification depend on which manager this machine happens
// to have.
func pathExcludingToolchain() string {
	return pathExcludingExecutables("node", "npm", "mise", "pnpm", "bun", "yarn")
}

func pathWithStubNode(version string) string {
	return writeStubNodeScript(version) + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeStubPackageManagerScript writes an executable `kind` script into a
// fresh temp directory that prints version on `--version` and exits non-zero
// on anything else, and returns that directory. It never installs anything:
// no spec drives a real install through a stub.
func writeStubPackageManagerScript(kind, version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubmanager-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo %s; exit 0; fi\nexit 1\n", version)
	Expect(os.WriteFile(filepath.Join(dir, kind), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithStubNodeAndPackageManager returns pathWithStubNode's PATH with a
// stub `kind` reporting managerVersion ahead of it, so a spec controls the
// version checks.package_manager classifies rather than inheriting the
// host's.
func pathWithStubNodeAndPackageManager(nodeVersion, kind, managerVersion string) string {
	return writeStubPackageManagerScript(kind, managerVersion) + string(os.PathListSeparator) + pathWithStubNode(nodeVersion)
}

// writeRecordingStubPackageManagerScript extends
// writeStubPackageManagerScript with a record of the working directory and
// the environment variable names each invocation actually saw, so a spec can
// assert how the probe confined the subprocess rather than only what it
// returned.
func writeRecordingStubPackageManagerScript(kind, version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-recordingmanager-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\necho \"$PWD\" >> %q\nenv | sed 's/=.*//' >> %q\n"+
		"if [ \"$1\" = \"--version\" ]; then echo %s; exit 0; fi\nexit 1\n",
		filepath.Join(dir, stubPackageManagerCwdLog), filepath.Join(dir, stubPackageManagerEnvLog), version)
	Expect(os.WriteFile(filepath.Join(dir, kind), []byte(script), 0o755)).To(Succeed())
	return dir
}

func readStubPackageManagerCwds(managerDir string) []string {
	return readStubPackageManagerLog(managerDir, stubPackageManagerCwdLog)
}

func readStubPackageManagerEnv(managerDir string) []string {
	return readStubPackageManagerLog(managerDir, stubPackageManagerEnvLog)
}

func readStubPackageManagerLog(managerDir, name string) []string {
	data, err := os.ReadFile(filepath.Join(managerDir, name))
	Expect(err).NotTo(HaveOccurred(), "expected the stub package manager at %s to have recorded %s", managerDir, name)
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// pathWithoutNode names pathExcludingToolchain from the perspective of the
// node_missing specs: with no node reachable, checkNodeReadiness reports
// node_missing regardless of the host's actual Node installation.
func pathWithoutNode() string {
	return pathExcludingToolchain()
}

// requireStubNodeVersion is the belt-and-suspenders probe mirroring
// node_absent_acceptance_test.go's pattern: it proves path's stub node is
// genuinely the one that would be resolved and reports exactly version, so
// a deterministic result below cannot be a false green caused by some other
// node still being reachable.
func requireStubNodeVersion(path, wantVersion string) {
	probe := exec.Command("sh", "-c", "node --version")
	probe.Env = []string{"PATH=" + path}
	output, err := probe.Output()
	Expect(err).NotTo(HaveOccurred(), "expected the stub node to be reachable on %q", path)
	Expect(strings.TrimSpace(string(output))).To(Equal(wantVersion))
}

// requireNodeUnreachable mirrors node_absent_acceptance_test.go's probe: it
// proves neither node nor npm resolves on path, so a deterministic
// node_missing result below cannot be a false green.
func requireNodeUnreachable(path string) {
	probe := exec.Command("sh", "-c", "command -v node || command -v npm")
	probe.Env = []string{"PATH=" + path}
	Expect(probe.Run()).To(HaveOccurred(), "expected neither node nor npm to be found on %q", path)
}

// runCoachCheckProjectEnv runs `coach codesignal [args...]` in repo with a
// caller-controlled PATH (plus the host's HOME, so git can find its global
// config), returning raw stdout/stderr without assuming success. Unlike
// runCoachSuggest, it does not inherit the test process's ambient
// environment: checkNodeReadiness shells out to whatever `node` is first on
// the child's PATH, so a deterministic node-dependent spec must control that
// PATH.
func runCoachCheckProjectEnv(workingDir, path string, args ...string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(commandPath, workingDir, stubToolchainEnv(path), append([]string{"codesignal"}, args...)...)
}

// corruptCommittedBlob deletes path's loose object file after it has been
// committed, leaving the commit/tree objects (and thus revision resolution)
// intact while making the blob itself unreadable.
func corruptCommittedBlob(repo, path string) {
	revCmd := exec.Command("git", "rev-parse", "HEAD:"+path)
	revCmd.Dir = repo
	output, err := revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	blobSHA := strings.TrimSpace(string(output))
	Expect(blobSHA).To(HaveLen(40))

	objectPath := filepath.Join(repo, ".git", "objects", blobSHA[:2], blobSHA[2:])
	_, statErr := os.Stat(objectPath)
	Expect(statErr).NotTo(HaveOccurred(), "expected a loose object at %s -- was the fixture repo gc'd?", objectPath)
	Expect(os.Remove(objectPath)).To(Succeed())
}

// corruptCommittedTree deletes dirPath's own subtree loose object after it
// has been committed, leaving the parent tree/commit objects (and thus
// revision resolution) intact while making a path underneath dirPath
// unresolvable. Unlike corruptCommittedBlob (which corrupts the leaf blob
// itself), this exercises a git-plumbing call that only walks tree objects
// without ever opening blob content.
func corruptCommittedTree(repo, dirPath string) {
	revCmd := exec.Command("git", "rev-parse", "HEAD:"+dirPath)
	revCmd.Dir = repo
	output, err := revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	treeSHA := strings.TrimSpace(string(output))
	Expect(treeSHA).To(HaveLen(40))

	objectPath := filepath.Join(repo, ".git", "objects", treeSHA[:2], treeSHA[2:])
	_, statErr := os.Stat(objectPath)
	Expect(statErr).NotTo(HaveOccurred(), "expected a loose object at %s -- was the fixture repo gc'd?", objectPath)
	Expect(os.Remove(objectPath)).To(Succeed())
}

// writeHangingNodeScript writes an executable `node` script that never
// terminates on its own, returning the directory containing it. `exec sleep
// N` replaces the shell's own process image, so killing the script's PID
// (as a context deadline does) kills the sleep directly instead of leaving
// it as an orphaned child.
func writeHangingNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-hangnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\nexec sleep 30\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithHangingNode returns a PATH whose first entry is a stub `node`
// that hangs indefinitely on `--version`, with every directory containing a
// real node/npm/mise executable removed so the stub is the only "node" the
// child process can resolve.
func pathWithHangingNode() string {
	return writeHangingNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeFailingNodeScript writes an executable `node` script that exits
// non-zero on any invocation without printing a parsable version, returning
// the directory containing it. This drives detectHostNodeMajor's
// exitErr != nil branch specifically, distinct from a timeout (hangs, never
// exits) or an unparsable-but-successful probe (exits 0 with junk output).
func writeFailingNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-failnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\nexit 3\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithFailingNode returns a PATH whose first entry is a stub `node`
// that exits non-zero on `--version` without printing output, with every
// directory containing a real node/npm/mise executable removed so the stub
// is the only "node" the child process can resolve.
func pathWithFailingNode() string {
	return writeFailingNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeUnstartableNodeScript writes an executable file named `node` whose
// shebang names an interpreter that does not exist, returning the directory
// containing it. exec.Cmd.Start() resolves the name via LookPath (it is
// executable, so LookPath succeeds) but the subsequent fork/exec fails,
// distinct from writeFailingNodeScript's case (the process starts and exits
// non-zero) and driving detectHostNodeMajor's cmd.Start() failure path
// specifically.
func writeUnstartableNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-unstartnode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/nonexistent/interpreter\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithUnstartableNode returns a PATH whose first entry is a stub `node`
// that resolves via LookPath but fails to start (a shebang naming a missing
// interpreter), with every directory containing a real node/npm/mise
// executable removed so the stub is the only "node" the child process can
// resolve.
func pathWithUnstartableNode() string {
	return writeUnstartableNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeOversizedUnparsableNodeScript writes an executable `node` script
// whose `--version` output is a non-parsable blob at maxNodeVersionProbeOutput
// (4 KiB) -- the largest detectHostNodeMajor's own probe budget allows
// without erroring -- returning the directory containing it. This drives
// nodeUnverifiableDetail's rawVersion-embedding branch with the widest input
// it can actually receive, rather than a short literal like "weird-build-2024".
func writeOversizedUnparsableNodeScript() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-bignode-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\nhead -c 4096 </dev/zero | tr '\\0' x\n"
	Expect(os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// pathWithOversizedUnparsableNode returns a PATH whose first entry is a stub
// `node` printing a 4 KiB unparsable blob on any invocation (including
// `--version`), with every directory containing a real node/npm/mise
// executable removed so the stub is the only "node" the child process can
// resolve.
func pathWithOversizedUnparsableNode() string {
	return writeOversizedUnparsableNodeScript() + string(os.PathListSeparator) + pathExcludingToolchain()
}

// writeStubMiseScript answers every mise invocation with version and
// records each invocation's argv and working directory, so a spec can pin
// both the outcome and the read-only command that produced it.
func writeStubMiseScript(version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	installDir := filepath.Join(dir, "install")
	Expect(os.MkdirAll(filepath.Join(installDir, "node_modules", "typescript"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(installDir, "node_modules", "typescript", "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version)), 0o644)).To(Succeed())
	nativeUnscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
	nativeDir := filepath.Join(installDir, "node_modules", "@typescript", nativeUnscoped)
	Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+nativeUnscoped, version)), 0o644)).To(Succeed())

	script := fmt.Sprintf("#!/bin/sh\necho \"$PWD\" >> %q\necho \"$@\" >> %q\n"+
		"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
		"if [ \"$1\" = \"where\" ]; then echo %q; exit 0; fi\necho %s\n", filepath.Join(dir, stubMiseCwdLog), filepath.Join(dir, stubMiseInvocationLog), installDir, version)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func readStubMiseInvocations(miseDir string) []string {
	data, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog))
	Expect(err).NotTo(HaveOccurred(), "expected the stub mise at %s to have recorded at least one invocation", miseDir)
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func readStubMiseCwds(miseDir string) []string {
	data, err := os.ReadFile(filepath.Join(miseDir, stubMiseCwdLog))
	Expect(err).NotTo(HaveOccurred(), "expected the stub mise at %s to have recorded probe working directories", miseDir)
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// pathWithStubNodeAndMise returns a PATH whose first two entries are a stub
// `node` reporting nodeVersion and a stub `mise` reporting miseVersion
// (regardless of its arguments), with every directory containing a real
// node/npm/mise executable removed, plus the stub mise's own directory so a
// spec can inspect its recorded invocations via readStubMiseInvocations.
func pathWithStubNodeAndMise(nodeVersion, miseVersion string) (path, miseDir string) {
	miseDir = writeStubMiseScript(miseVersion)
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
	return path, miseDir
}

// writeWorktreeFile writes name with contents directly into the worktree at
// repo without committing or `git add`ing it. The compiler check reads
// package.json/mise.toml/node_modules as host-readiness state of the
// worktree, never the Git snapshot, so these fixtures deliberately stay
// uncommitted -- proving the resolver reads the worktree directly rather
// than depending on anything reaching HEAD.
func writeWorktreeFile(repo, name, contents string) {
	full := filepath.Join(repo, name)
	Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
	Expect(os.WriteFile(full, []byte(contents), 0o644)).To(Succeed())
}

func writeInstalledTypescript(repo, version string) {
	writeInstalledTypescriptUnder(repo, ".", version)
}

func writeInstalledTypescriptCompilerOnly(repo, version string) {
	writeInstalledTypescriptCompilerUnder(repo, ".", version)
}

func writeInstalledTypescriptUnder(repo, relDir, version string) {
	writeInstalledTypescriptCompilerUnder(repo, relDir, version)
	writeInstalledNativeTypescriptUnder(repo, relDir, version)
}

// writeStatefulStubMiseScript mirrors writeStubMiseScript's shape but models
// mise's own real pre/post-install state transition, offline: `mise where`
// fails until an `install` invocation for toolSpec has actually run
// (moving a pre-staged fixture into place), so a spec can prove that a
// post-install rerun genuinely observes a state change caused by the
// install it confirmed, without a real network-dependent mise install.
// Every invocation's argv is logged exactly like writeStubMiseScript's, so
// readStubMiseInvocations/miseInvocationsIncludeInstall work identically.
func writeStatefulStubMiseScript(version string) (dir string) {
	dir, err := os.MkdirTemp("", "coach-acceptance-statefulmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	staging := filepath.Join(dir, "staging")
	Expect(os.MkdirAll(filepath.Join(staging, "node_modules", "typescript"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(staging, "node_modules", "typescript", "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version)), 0o644)).To(Succeed())
	nativeUnscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
	nativeDir := filepath.Join(staging, "node_modules", "@typescript", nativeUnscoped)
	Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+nativeUnscoped, version)), 0o644)).To(Succeed())

	installDir := filepath.Join(dir, "install")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n"+
		"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then exit 1; fi\n"+
		"if [ \"$1\" = \"install\" ]; then mv %q %q; exit 0; fi\n"+
		"if [ \"$1\" = \"where\" ]; then if [ -d %q ]; then echo %q; exit 0; else exit 1; fi; fi\n"+
		"echo %s\n",
		filepath.Join(dir, stubMiseInvocationLog), staging, installDir, installDir, installDir, version)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func pathWithStatefulStubNodeAndMise(nodeVersion, tsVersion string) (path, miseDir string) {
	miseDir = writeStatefulStubMiseScript(tsVersion)
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
	return path, miseDir
}

// writeStatefulStubMiseScriptGlobalAware extends writeStatefulStubMiseScript's
// pre/post-install state transition (`mise where` failing until `install`
// has actually moved the staged fixture into place) with an always-succeeding
// `mise config get tools.npm:typescript -g`. The production install path
// never runs `mise use -g`, so `config get -g` cannot start succeeding as a
// side effect of that install; the faithful pre-install state this models is
// "the global mise config already declares npm:typescript@<version> but it
// is not yet installed".
func writeStatefulStubMiseScriptGlobalAware(version string) (dir string) {
	dir, err := os.MkdirTemp("", "coach-acceptance-statefulmiseglobal-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	staging := filepath.Join(dir, "staging")
	Expect(os.MkdirAll(filepath.Join(staging, "node_modules", "typescript"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(staging, "node_modules", "typescript", "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version)), 0o644)).To(Succeed())
	nativeUnscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
	nativeDir := filepath.Join(staging, "node_modules", "@typescript", nativeUnscoped)
	Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+nativeUnscoped, version)), 0o644)).To(Succeed())

	installDir := filepath.Join(dir, "install")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n"+
		"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then echo %s; exit 0; fi\n"+
		"if [ \"$1\" = \"install\" ]; then mv %q %q; exit 0; fi\n"+
		"if [ \"$1\" = \"where\" ]; then if [ -d %q ]; then echo %q; exit 0; else exit 1; fi; fi\n"+
		"exit 1\n",
		filepath.Join(dir, stubMiseInvocationLog), version, staging, installDir, installDir, installDir)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func pathWithStatefulStubNodeAndMiseGlobalAware(nodeVersion, tsVersion string) (path, miseDir string) {
	miseDir = writeStatefulStubMiseScriptGlobalAware(tsVersion)
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
	return path, miseDir
}

// writeFailingInstallStubMiseScript mirrors writeStatefulStubMiseScript's
// shape, but its `install` subcommand always exits 1 without ever moving
// the staged fixture into place, modeling a genuine `mise install` failure
// rather than a declined/cancelled selection.
func writeFailingInstallStubMiseScript() (dir string) {
	dir, err := os.MkdirTemp("", "coach-acceptance-failinstallmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n"+
		"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then exit 1; fi\n"+
		"if [ \"$1\" = \"install\" ]; then exit 1; fi\n",
		filepath.Join(dir, stubMiseInvocationLog))
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func pathWithFailingInstallStubNodeAndMise(nodeVersion string) (path, miseDir string) {
	miseDir = writeFailingInstallStubMiseScript()
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
	return path, miseDir
}

// writeIneligibleInstallStubMiseScript mirrors writeFailingInstallStubMiseScript's
// shape, but its `install` subcommand exits 0 without ever moving a staged
// fixture into place, and `where` always exits 1 -- modeling a genuine mise
// exit-zero install whose freshly-installed compiler is never actually
// locatable, so classifyCompilerCandidate classifies it compilerClassAbsent
// rather than eligible. This is deliberately distinct from
// writeFailingInstallStubMiseScript's own always-exit-1 `install`: that
// models the subprocess itself failing, this models the subprocess
// succeeding while verification still fails.
func writeIneligibleInstallStubMiseScript() (dir string) {
	dir, err := os.MkdirTemp("", "coach-acceptance-ineligibleinstallmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n"+
		"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then exit 1; fi\n"+
		"if [ \"$1\" = \"install\" ]; then exit 0; fi\n"+
		"if [ \"$1\" = \"where\" ]; then exit 1; fi\n",
		filepath.Join(dir, stubMiseInvocationLog))
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func pathWithIneligibleInstallStubNodeAndMise(nodeVersion string) (path, miseDir string) {
	miseDir = writeIneligibleInstallStubMiseScript()
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
	return path, miseDir
}

// gitStatusPorcelain reports repo's worktree status, so a spec can prove no
// file inside it was created, modified, or removed between two points in
// time: a failed install must never leave Coach itself having touched the
// repository.
func gitStatusPorcelain(repo string) string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	output, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return string(output)
}

// noSupportedCompilerRepo commits a minimal TypeScript-shaped, policy-ready
// repository with no installed or declared compiler at all, so
// checks.compiler fails with typescript_compiler_missing and neither mise
// scope has anything configured -- the fixture every prepare_compiler mise
// spec below starts from.
func noSupportedCompilerRepo() string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
	return repo
}
