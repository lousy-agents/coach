package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const stubMiseInvocationLog = "mise-invocations.log"

const stubMiseCwdLog = "mise-probe-cwd.log"

// defaultStubMiseToolVersion is a supported mise-tool-version response
// (matching miseToolSupportedCalverYear), used as the default so this
// file's specs that predate mise-tool-version/config-hazard gating
// (SA-280-015/SA-280-045) keep resolving through mise exactly as before.
const defaultStubMiseToolVersion = "2026.9.5 linux-x64 (2026-09-10)"

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

// writeVersionedStubMiseScript extends writeStubMiseScript's contract
// with distinct, independently
// controllable responses for `mise --version` and `mise config ls -J`.
// That shared helper cannot do this itself: it echoes the same miseVersion
// argument for every invocation except `where`, which represents a
// TypeScript version in every spec that already depends on it, not a
// mise-tool version -- reusing it here would make every existing mise-origin
// compiler-aggregation spec collide with the new mise-tool-version gate.
//
// tsVersion seeds the `where`-fixture on disk (a real installed compiler at
// that version) but is otherwise unused unless whereVersion is requested;
// globalConfigVersion is what `config get tools.npm:typescript -g` reports
// (empty means "not configured", matching detectGlobalMiseTypescriptVersion's
// contract); toolVersionOutput is what `--version` reports (empty means the
// probe fails, modeling an undetectable mise-tool version); configLsJSON is
// the raw `config ls -J` response.
func writeVersionedStubMiseScript(tsVersion, toolVersionOutput, configLsJSON, globalConfigVersion string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	installDir := filepath.Join(dir, "install")
	if tsVersion != "" {
		Expect(os.MkdirAll(filepath.Join(installDir, "node_modules", "typescript"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(installDir, "node_modules", "typescript", "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", tsVersion)), 0o644)).To(Succeed())
		nativeUnscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
		nativeDir := filepath.Join(installDir, "node_modules", "@typescript", nativeUnscoped)
		Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+nativeUnscoped, tsVersion)), 0o644)).To(Succeed())
	}

	versionBranch := "exit 1"
	if toolVersionOutput != "" {
		versionBranch = fmt.Sprintf("echo %q; exit 0", toolVersionOutput)
	}
	configGetBranch := "exit 1"
	if globalConfigVersion != "" {
		configGetBranch = fmt.Sprintf("echo %q; exit 0", globalConfigVersion)
	}
	script := fmt.Sprintf(
		"#!/bin/sh\necho \"$PWD\" >> %q\necho \"$@\" >> %q\n"+
			"if [ \"$1\" = \"--version\" ]; then %s; fi\n"+
			"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo %q; exit 0; fi\n"+
			"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then %s; fi\n"+
			"if [ \"$1\" = \"where\" ]; then echo %q; exit 0; fi\n"+
			"exit 1\n",
		filepath.Join(dir, stubMiseCwdLog), filepath.Join(dir, stubMiseInvocationLog),
		versionBranch, configLsJSON, configGetBranch, installDir,
	)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func pathWithVersionedStubMise(nodeVersion, tsVersion, toolVersionOutput, configLsJSON, globalConfigVersion string) (path, miseDir string) {
	miseDir = writeVersionedStubMiseScript(tsVersion, toolVersionOutput, configLsJSON, globalConfigVersion)
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
	return path, miseDir
}

// pathWithStubMiseDefaultTool is a drop-in replacement for the compiler-
// aggregation specs' former use of the shared pathWithStubNodeAndMise helper: it answers
// `mise --version` with a supported version and `mise config ls -J` with an
// empty (hazard-free) config list, and otherwise reports tsVersion for both
// `config get` and `where`, exactly like the shared helper did.
func pathWithStubMiseDefaultTool(nodeVersion, tsVersion string) (path, miseDir string) {
	return pathWithVersionedStubMise(nodeVersion, tsVersion, defaultStubMiseToolVersion, "[]", tsVersion)
}

// installInvocationCount counts miseDir's logged invocations that began
// with "install", for the scan's single-use-confirmation proof: a second,
// unread "install" answer left in the pty must never cause a second `mise
// install` to run within the same coach invocation.
func installInvocationCount(miseDir string) int {
	count := 0
	for _, line := range readStubMiseInvocations(miseDir) {
		if strings.HasPrefix(line, "install ") {
			count++
		}
	}
	return count
}
