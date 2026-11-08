package projectcheck

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// When git cannot answer a question about a named policy, the check fails with an
// operational error. That error keeps git's own failure as its cause, so whatever
// reports it can tell a bound that tripped from a repository that is damaged, and the
// cause is git's text without Coach's wrapper around it. This is the policy gone from disk; a policy that is still
// on disk is covered by TestAPolicyOnDiskWhoseCommittedBlobCannotBeReadIsAnOperationalFailure.
func TestCheckExplicitPolicyOfAFileGoneFromDiskKeepsGitsOwnErrorWhenGitCannotReadTheCommittedPolicy(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	removeCommittedBlob(t, repo, "cfg/p.json")
	if err := os.RemoveAll(filepath.Join(repo, "cfg")); err != nil {
		t.Fatal(err)
	}

	_, _, err := checkExplicitPolicy(readinessGit(), repo, "HEAD", "cfg/p.json")

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
