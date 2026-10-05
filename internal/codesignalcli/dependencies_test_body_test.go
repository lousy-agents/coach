package codesignalcli

import (
	"os/exec"
	"strings"
	"testing"
)

func body_dependenciesTest_47(t *testing.T, pkg string) {
	cmd := exec.Command("go", "list", "-deps", pkg)
	cmd.Dir = "."
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list -deps %s: %v: %s", pkg, err, exitErr.Stderr)
		}
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}

	deps := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, dep := range deps {
		if prefix, forbidden := isForbiddenDependency(dep); forbidden {
			t.Errorf("go list -deps %s: found forbidden dependency %q (matches denylisted prefix %q)", pkg, dep, prefix)
		}
	}
}
