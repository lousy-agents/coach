// Command acceptance-style-guard fails if any acceptance_test.go or
// *_acceptance_test.go under the module root does not import
// github.com/onsi/ginkgo/v2 and reference a Ginkgo suite/spec API, except an
// explicit allowlist (see internal/acceptancestyle).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/acceptancestyle"
)

func main() {
	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "acceptance-style-guard: %v\n", err)
		os.Exit(2)
	}

	violations, err := acceptancestyle.Check(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acceptance-style-guard: %v\n", err)
		os.Exit(2)
	}
	if len(violations) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "acceptance-style-guard: %d violation(s):\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", v.Path, v.Reason)
	}
	os.Exit(1)
}

// moduleRoot prefers go.mod over git rev-parse so the guard works without a
// git checkout and does not couple cmd/ to internal/codesignalcli.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found from %s upward", dir)
		}
		dir = parent
	}
}
