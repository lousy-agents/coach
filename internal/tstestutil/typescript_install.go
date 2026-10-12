package tstestutil

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	. "github.com/onsi/gomega"
)

var (
	realCompilerOnce sync.Once
	realCompilerSkip string
)

// EnsureTypeScriptCompilerAvailable memoizes whether this repository's
// js/semantics/node_modules/typescript devDependency (plus its matching
// native platform package) is installed, so specs that copy that compiler
// into a fixture can skip in a Go-only CI leg instead of failing hard.
// On success it prepends mise's Node 24 bin to PATH when the host node is
// not major 24 or 26. Callers must be Ginkgo specs (it uses GinkgoT().Setenv).
func EnsureTypeScriptCompilerAvailable() (skipReason string) {
	realCompilerOnce.Do(func() {
		if _, err := exec.LookPath("node"); err != nil {
			realCompilerSkip = fmt.Sprintf("node not found on PATH; skipping specs requiring the real installed TypeScript compiler (%s)", err)
			return
		}
		tsPkgJSON := filepath.Join(typescriptDir(), "package.json")
		if _, err := os.Stat(tsPkgJSON); err != nil {
			realCompilerSkip = fmt.Sprintf("%s not found (run `npm ci` in js/semantics); skipping specs requiring the real installed TypeScript compiler (%s)", tsPkgJSON, err)
			return
		}
		nativePkgJSON := filepath.Join(nativeTypescriptDir(), "package.json")
		if _, err := os.Stat(nativePkgJSON); err != nil {
			realCompilerSkip = fmt.Sprintf("%s not found (run `npm ci` in js/semantics); skipping specs requiring the real installed TypeScript compiler (%s)", nativePkgJSON, err)
			return
		}
	})
	if realCompilerSkip != "" {
		return realCompilerSkip
	}
	prependAllowedAnalysisNodeToPath()
	return ""
}

// InstallTypeScriptCompiler copies this repository's own installed
// js/semantics node_modules/typescript devDependency into repo's own
// node_modules/typescript, uncommitted. PrepareTSRuntime's compiler
// resolution reads worktree filesystem state directly, never a Git
// snapshot, so this need not be tracked by Git. When includeNativePackage
// is true it also copies the matching native platform package.
func InstallTypeScriptCompiler(repo string, includeNativePackage bool) (packageDir string) {
	packageDir = filepath.Join(repo, "node_modules", "typescript")
	copyFileTree(typescriptDir(), packageDir)
	if includeNativePackage {
		nativeName := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, NPMArch())
		nativeDest := filepath.Join(repo, "node_modules", "@typescript", nativeName)
		copyFileTree(nativeTypescriptDir(), nativeDest)
	}
	return packageDir
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
