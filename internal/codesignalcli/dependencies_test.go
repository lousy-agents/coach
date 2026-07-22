package codesignalcli

import (
	"os/exec"
	"strings"
	"testing"
)

// forbiddenDependencyPrefixes lists import paths (matched as exact matches or
// package-path prefixes) that must never appear in the dependency graph of
// cmd/coach or internal/codesignalcli. Their presence would mean the CLI
// pulls in a GitHub client or a general-purpose HTTP client, defeating the
// "works with zero external service configuration" guarantee.
var forbiddenDependencyPrefixes = []string{
	"github.com/lousy-agents/coach/pkg/githubingest",
	"github.com/google/go-github",
	"github.com/bradleyfalzon/ghinstallation",
	"net/http",
}

// isForbiddenDependency reports whether dep is exactly one of
// forbiddenDependencyPrefixes or a subpackage of one of them (e.g.
// "net/http/httptest" under "net/http", or
// "github.com/google/go-github/v89/github" under
// "github.com/google/go-github").
func isForbiddenDependency(dep string) (string, bool) {
	for _, prefix := range forbiddenDependencyPrefixes {
		if dep == prefix || strings.HasPrefix(dep, prefix+"/") {
			return prefix, true
		}
	}
	return "", false
}

// TestNoExternalDependencies proves an import-graph boundary at build time:
// neither cmd/coach nor internal/codesignalcli may (transitively) import a
// GitHub client, an installation-auth library, or net/http. It does NOT
// prove the OS or network is actually isolated at runtime -- see the
// offline acceptance scenario in cmd/coach/acceptance_test.go for that.
func TestNoExternalDependencies(t *testing.T) {
	packages := []string{
		"./../../cmd/coach/...",
		"./...",
	}

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
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
		})
	}
}
