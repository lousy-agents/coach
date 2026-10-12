package githubingest_test

import (
	"os/exec"
	"strings"
	"testing"
)

// AC-5.1: pkg/githubingest shall not import pkg/semantics.
func TestPackageBoundary_GithubingestDoesNotImportSemantics(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lousy-agents/coach/pkg/githubingest/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps github.com/lousy-agents/coach/pkg/githubingest/... failed: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(string(out), "lousy-agents/coach/pkg/semantics") {
		t.Fatalf("pkg/githubingest must not depend on pkg/semantics, but dependency list contained it:\n%s", out)
	}
}

// AC-1.1 regression guard: pkg/semantics shall not import the GitHub App
// dependencies introduced by this package (go-github, ghinstallation).
func TestPackageBoundary_SemanticsDoesNotImportGithubDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lousy-agents/coach/pkg/semantics/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps github.com/lousy-agents/coach/pkg/semantics/... failed: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(string(out), "go-github") {
		t.Fatalf("pkg/semantics must not depend on go-github, but dependency list contained it:\n%s", out)
	}
	if strings.Contains(string(out), "ghinstallation") {
		t.Fatalf("pkg/semantics must not depend on ghinstallation, but dependency list contained it:\n%s", out)
	}
}
