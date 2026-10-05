package tstestutil

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"

	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

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
func allowedAnalysisNodeMajor(major int) bool {
	for _, allowed := range AllowedAnalysisNodeMajors {
		if allowed == major {
			return true
		}
	}
	return false
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
