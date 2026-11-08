package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The policy document is the one read held to the policy's size limit: a committed file past it fails at once
// with the output bound, which is what tells the user the document is too large. The tree around it is
// listed under the listing budget, so what the revision's root holds is not mistaken for a document.
func TestTheGitRunnerHoldsTheDocumentToItsLimitAndNotTheListingOfTheTree(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "big.json", strings.Repeat("x", MaxBytes+1))
	run := NewGitRunner()

	_, err := run(repo, "show", "HEAD:big.json")

	var bound *gitrepo.BoundError
	if !errors.As(err, &bound) || bound.Kind != gitrepo.BoundStdout {
		t.Fatalf("show of a file one byte past the limit = %v; want the output bound", err)
	}
	if _, err := run(repo, "ls-tree", "--full-tree", "--name-only", "-z", "HEAD"); err != nil {
		t.Errorf("a listing of the tree failed: %v", err)
	}
}
