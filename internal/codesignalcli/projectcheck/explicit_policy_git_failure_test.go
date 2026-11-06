package projectcheck

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// When git cannot answer a question about a named policy, the check fails with an
// operational error. That error keeps git's own failure as its cause, so whatever
// reports it can tell a bound that tripped from a repository that is damaged, and the
// cause is git's text without Coach's wrapper around it.
func TestCheckExplicitPolicyKeepsGitsOwnErrorWhenGitCannotReadTheCommittedPolicy(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	blob, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD:cfg/p.json").Output()
	if err != nil {
		t.Fatal(err)
	}
	object := strings.TrimSpace(string(blob))
	if err := os.Remove(filepath.Join(repo, ".git", "objects", object[:2], object[2:])); err != nil {
		t.Fatalf("the loose object must exist in a fresh repository: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(repo, "cfg")); err != nil {
		t.Fatal(err)
	}

	_, _, err = checkExplicitPolicy(repo, "HEAD", "cfg/p.json")

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Fatalf("error = %v; want an *OperationalError", err)
	}
	if operational.Cause == nil {
		t.Fatalf("OperationalError %q has no Cause: git's own error was flattened into text", operational.Message)
	}
	if strings.Contains(operational.Cause.Error(), "coach codesignal") {
		t.Errorf("Cause %q carries Coach's own wrapper around git's error", operational.Cause)
	}
}
