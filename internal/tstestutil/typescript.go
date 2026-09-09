// Package tstestutil holds helpers shared by Ginkgo acceptance tests that
// copy this repository's installed TypeScript compiler into a fixture
// worktree. Production code must not import it.
package tstestutil

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// NPMArch maps Go's runtime.GOARCH to the npm/Node process.arch naming
// convention typescript's native platform package directories use (e.g.
// "amd64" -> "x64"), which differs from Go's own arch names.
func NPMArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return runtime.GOARCH
	}
}

func repoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func typescriptDir() string {
	return filepath.Join(repoRoot(), "js", "semantics", "node_modules", "typescript")
}

func nativeTypescriptDir() string {
	name := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, NPMArch())
	return filepath.Join(repoRoot(), "js", "semantics", "node_modules", "@typescript", name)
}

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

func hostNodeMajorOnPath() (int, bool) {
	path, err := exec.LookPath("node")
	if err != nil {
		return 0, false
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return 0, false
	}
	trimmed := strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
	majorPart, _, _ := strings.Cut(trimmed, ".")
	major, err := strconv.Atoi(majorPart)
	if err != nil {
		return 0, false
	}
	return major, true
}

// AllowedAnalysisNodeMajors is the set of Node majors
// prependAllowedAnalysisNodeToPath treats as already acceptable on the host
// PATH. It must be kept equal to codesignalcli.SupportedNodeMajors --
// tstestutil cannot import codesignalcli directly without an import cycle,
// so cmd/coach's TestTSTestutilAllowedNodeMajorsMatchesSupportedNodeMajors
// binds the two together.
var AllowedAnalysisNodeMajors = []int{24, 26}

func allowedAnalysisNodeMajor(major int) bool {
	for _, allowed := range AllowedAnalysisNodeMajors {
		if allowed == major {
			return true
		}
	}
	return false
}

func prependAllowedAnalysisNodeToPath() {
	if major, ok := hostNodeMajorOnPath(); ok && allowedAnalysisNodeMajor(major) {
		return
	}
	bin := filepath.Join(os.Getenv("HOME"), ".local", "share", "mise", "installs", "node", "24", "bin")
	if _, err := os.Stat(filepath.Join(bin, "node")); err != nil {
		return
	}
	GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TypeScriptVersion reads js/semantics' own installed typescript
// devDependency version directly from disk, so fixtures declaring an exact
// matching version in package.json cannot drift from the actual installed
// copy the suite copies.
func TypeScriptVersion() string {
	data, err := os.ReadFile(filepath.Join(typescriptDir(), "package.json"))
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
