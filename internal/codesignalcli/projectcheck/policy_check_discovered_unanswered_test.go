package projectcheck

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

var errListingUnreadable = errors.New("git cannot list the tree")

// holdsNoBlobAndCannotListTheTree answers like a repository with a commit and nothing at the path, and fails
// the one question that says whether the revision holds the path in another case: the listing of a tree.
func holdsNoBlobAndCannotListTheTree(_ string, args ...string) ([]byte, error) {
	if slices.Contains(args, "--name-only") {
		return nil, errListingUnreadable
	}
	return nil, nil
}

// A discovered policy that is missing is called a gap to author over only if the revision does not hold it in
// another case. When git cannot say, that is an operational error that names the question, as it is for every
// other question git could not answer, and not a gap that offers a new policy over a file that may be there.
func TestADiscoveredPolicyIsNotCalledMissingWhenGitCannotSayWhetherTheRevisionHoldsItInAnotherCase(t *testing.T) {
	run := holdsNoBlobAndCannotListTheTree

	check, roots, err := checkDiscoveredPolicy(run, "repo", "HEAD", ".lousy-agents/coach.json")

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || !errors.Is(operational.Cause, errListingUnreadable) {
		t.Fatalf("checkDiscoveredPolicy = %+v, %v, %v; want an operational error that keeps git's own error", check, roots, err)
	}
	for _, want := range []string{"coach codesignal --check-project: ", ".lousy-agents/coach.json", "could not check whether the analyzed revision holds it in another case"} {
		if !strings.Contains(operational.Message, want) {
			t.Errorf("message %q; want it to contain %q", operational.Message, want)
		}
	}
	if strings.Contains(operational.Message, "--project-config") {
		t.Errorf("message %q names a flag the user did not pass: the policy was discovered", operational.Message)
	}
}
