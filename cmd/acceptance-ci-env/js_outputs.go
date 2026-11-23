package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// jsBuiltOutputs are the js/semantics build outputs the ts-project-backend
// specs need. CI's job builds them with `mise run js-install` before it runs
// those specs, and git ignores them, so the repository copy lacks them.
var jsBuiltOutputs = []string{"js/semantics/node_modules", "js/semantics/dist"}

func wantsTSProjectBackend(goTestArgs []string) bool {
	for _, arg := range goTestArgs {
		if strings.Contains(arg, "label-filter") && strings.Contains(arg, "ts-project-backend") {
			return true
		}
	}
	return false
}

// checkJSOutputs refuses a ts-project-backend run when the caller has not built
// js/semantics, rather than letting every spec skip for want of a compiler.
func checkJSOutputs() error {
	if _, err := os.Stat(jsBuiltOutputs[0]); err != nil {
		return fmt.Errorf("the ts-project-backend label needs %s, which CI's job builds before its tests; run `mise run js-install` first", jsBuiltOutputs[0])
	}
	return nil
}

// copyJSOutputs copies the built outputs that exist into the repository copy.
func copyJSOutputs(repoCopy string) ([]string, error) {
	var copied []string
	for _, output := range jsBuiltOutputs {
		if _, err := os.Stat(output); err != nil {
			continue
		}
		target := filepath.Join(repoCopy, output)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if out, err := exec.Command("cp", "-a", output, target).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("copying %s: %v: %s", output, err, out)
		}
		copied = append(copied, output)
	}
	return copied, nil
}
