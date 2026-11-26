package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
)

// checkTypeScriptToolchain returns the checkout's pinned compiler, or one error that says what is wrong and what to do.
func checkTypeScriptToolchain(root string) (string, error) {
	tsc := filepath.Join(root, "js", "semantics", "node_modules", ".bin", "tsc")
	if _, err := os.Stat(tsc); err != nil {
		return "", fmt.Errorf("the pinned TypeScript compiler is missing at %s: js/semantics/node_modules is not installed or is being replaced; run `mise run js-install`, and do not run it while the specs run", tsc)
	}
	output, _ := exec.Command(tsc, "--version").CombinedOutput()
	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(output)), "Version"))
	if version != pinnedTypeScript {
		return "", fmt.Errorf("the compiler at %s is TypeScript %q, but the specs need exactly %s: run `mise run js-install`", tsc, version, pinnedTypeScript)
	}
	return tsc, nil
}

// snapshotCompiler copies the typescript package that tsc belongs to, with the platform packages it resolves beside
// it, into a node_modules of its own under into, and returns the copy's tsc, so an install into the checkout during a
// run cannot change the compiler the specs use.
func snapshotCompiler(tsc, into string) (string, error) {
	resolved, err := filepath.EvalSymlinks(tsc)
	if err != nil {
		return "", err
	}
	packageDir := filepath.Dir(filepath.Dir(resolved))
	installed := filepath.Dir(packageDir)
	copied := filepath.Join(into, "node_modules")
	if err := os.MkdirAll(copied, 0o755); err != nil {
		return "", err
	}
	for _, name := range []string{filepath.Base(packageDir), "@typescript"} {
		if output, err := exec.Command("cp", "-R", filepath.Join(installed, name), copied).CombinedOutput(); err != nil {
			return "", fmt.Errorf("copying %s: %v: %s", name, err, output)
		}
	}
	return filepath.Join(copied, filepath.Base(packageDir), "bin", filepath.Base(resolved)), nil
}

var (
	compilerOnce     sync.Once
	suiteCompiler    string
	compilerSnapshot string
)

// suiteTypeScriptCompiler checks the toolchain on first use and aborts the whole run with that one message when it is
// not usable, instead of letting every TypeScript spec fail on its own.
func suiteTypeScriptCompiler() string {
	compilerOnce.Do(func() {
		tsc, err := checkTypeScriptToolchain(repositoryRoot())
		if err != nil {
			AbortSuite(err.Error())
		}
		var dir string
		dir, err = os.MkdirTemp("", "coach-eval-tsc")
		if err == nil {
			compilerSnapshot = dir
			suiteCompiler, err = snapshotCompiler(tsc, dir)
		}
		if err != nil {
			AbortSuite(err.Error())
		}
	})
	return suiteCompiler
}
