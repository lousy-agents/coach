package evalgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PinnedTypeScript is the compiler version a TypeScript program is validated with: the native port that
// js/semantics pins.
const PinnedTypeScript = "7.0.2"

// TypeScriptTools are the programs that validate a TypeScript program.
type TypeScriptTools struct {
	Node, TSC string
}

// ResolveTypeScriptTools finds node on PATH and the pinned compiler: EVAL_TSC if it is set, otherwise the one
// installed in js/semantics of the checkout that holds workingDir. Global compilers are never used, because they
// differ between machines.
func ResolveTypeScriptTools(workingDir string) (TypeScriptTools, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return TypeScriptTools{}, fmt.Errorf("node is not on PATH: TypeScript programs are validated with node --test")
	}
	tsc, err := locateTSC(workingDir)
	if err != nil {
		return TypeScriptTools{}, err
	}
	if err := requirePinnedVersion(tsc); err != nil {
		return TypeScriptTools{}, err
	}
	return TypeScriptTools{Node: node, TSC: tsc}, nil
}

func locateTSC(workingDir string) (string, error) {
	if named := os.Getenv("EVAL_TSC"); named != "" {
		return existing(named, fmt.Errorf("EVAL_TSC names %s, which does not exist", named))
	}
	root, found := checkoutRoot(workingDir)
	if !found {
		return "", fmt.Errorf("no TypeScript compiler: set EVAL_TSC, or run from a checkout that has js/semantics/node_modules installed")
	}
	candidate := filepath.Join(root, "js", "semantics", "node_modules", ".bin", "tsc")
	return existing(candidate, fmt.Errorf("the pinned TypeScript compiler is not installed at %s: run `mise run js-install`, or set EVAL_TSC", candidate))
}

func existing(path string, missing error) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", missing
	}
	return path, nil
}

// checkoutRoot is the nearest directory at or above dir that holds the eval module.
func checkoutRoot(dir string) (string, bool) {
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
	if err != nil || version != PinnedTypeScript {
		return fmt.Errorf("%s is TypeScript %q, but the generator needs exactly %s", tsc, version, PinnedTypeScript)
	}
	return nil
}
