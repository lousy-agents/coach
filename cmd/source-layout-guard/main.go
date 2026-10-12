// Command source-layout-guard fails if any Go file under the module root is
// named, or declares identifiers named, by position or hash rather than by
// responsibility (see internal/sourcelayout and ADR-007).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/sourcelayout"
)

func main() {
	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "source-layout-guard: %v\n", err)
		os.Exit(2)
	}

	violations, err := sourcelayout.Check(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "source-layout-guard: %v\n", err)
		os.Exit(2)
	}
	if len(violations) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "source-layout-guard: %d violation(s):\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", v.Path, v.Reason)
	}
	os.Exit(1)
}

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
