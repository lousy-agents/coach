package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/tstestutil"
)

// skipWithoutRealTypeScriptCompiler skips the current spec when this
// repository's own installed js/semantics TypeScript compiler, which the
// real-backend specs copy into their fixtures, is unavailable.
func skipWithoutRealTypeScriptCompiler() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
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

func nativeTypescriptPackageLookupName() string {
	return fmt.Sprintf("@typescript/typescript-%s-%s", runtime.GOOS, npmArchName())
}

func writeInstalledNativeTypescript(repo, version string) {
	writeInstalledNativeTypescriptUnder(repo, ".", version)
}
