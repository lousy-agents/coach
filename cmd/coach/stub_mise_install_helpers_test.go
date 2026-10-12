package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
