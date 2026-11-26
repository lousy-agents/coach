package evalgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func locateTSC(workingDir string) (string, error) {
	if named := os.Getenv("EVAL_TSC"); named != "" {
		absolute, err := filepath.Abs(named)
		if err != nil {
			return "", fmt.Errorf("EVAL_TSC names %s, which has no absolute path: %v", named, err)
		}
		return namedCompiler(absolute, named)
	}
	root, found := checkoutRoot(workingDir)
	if !found {
		return "", fmt.Errorf("no TypeScript compiler: set EVAL_TSC, or run from a checkout that has js/semantics/node_modules installed")
	}
	candidate := filepath.Join(root, "js", "semantics", "node_modules", ".bin", "tsc")
	return existing(candidate, fmt.Errorf("the pinned TypeScript compiler is not installed at %s: run `mise run js-install`, or set EVAL_TSC", candidate))
}

// namedCompiler accepts the tsc file itself or the typescript package directory that holds it.
func namedCompiler(absolute, named string) (string, error) {
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("EVAL_TSC names %s, which does not exist", named)
	}
	if !info.IsDir() {
		return absolute, nil
	}
	return existing(filepath.Join(absolute, "bin", "tsc"), fmt.Errorf("EVAL_TSC names the directory %s, which holds no bin/tsc: %s", named, entryPointHint))
}

const entryPointHint = "expected the tsc entry point (node_modules/.bin/tsc, or the typescript package directory)"

func existing(path string, missing error) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", missing
	}
	return path, nil
}

// checkoutRoot is the nearest directory at or above dir that holds the eval module, found through any symbolic link.
func checkoutRoot(dir string) (string, bool) {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	for current := dir; filepath.Dir(current) != current; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "eval", "go.mod")); err == nil {
			return current, true
		}
	}
	return "", false
}

func requirePinnedVersion(tsc string) error {
	output, err := exec.Command(tsc, "--version").CombinedOutput()
	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(output)), "Version"))
	if version == "" {
		return fmt.Errorf("%s prints no TypeScript version: %s", tsc, entryPointHint)
	}
	if err != nil || version != PinnedTypeScript {
		return fmt.Errorf("%s is TypeScript %q, but the generator needs exactly %s", tsc, version, PinnedTypeScript)
	}
	return nil
}
