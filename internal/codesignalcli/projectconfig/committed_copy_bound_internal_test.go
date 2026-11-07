package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// Only a copy that is over the size budget is called so. A read that stopped on the time or the stderr bound did
// not find a policy that is too large: it found nothing, and to call it oversize would offer a new policy over a
// reviewed one. It is a failure of the read, returned as one, and no verdict about the copy.
func TestACommittedCopyWhoseReadStoppedOnAnotherBoundIsAFailureAndNotOversize(t *testing.T) {
	cases := map[string]gitrepo.BoundErrorKind{"the time": gitrepo.BoundTimeout, "the stderr": gitrepo.BoundStderr}
	for name, kind := range cases {
		t.Run(name, func(t *testing.T) { requireTheFailedReadAndNoVerdict(t, kind) })
	}
}

func requireTheFailedReadAndNoVerdict(t *testing.T, kind gitrepo.BoundErrorKind) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return nil, &gitrepo.BoundError{Kind: kind} }

	problem, err := committedCopyProblem(run, "repo", "HEAD", "p.json")

	var bound *gitrepo.BoundError
	if !errors.As(err, &bound) || bound.Kind != kind || problem != "" {
		t.Errorf("committedCopyProblem = %q, %v; want the failed read and no verdict that the policy is over the size budget", problem, err)
	}
}

func TestACommittedCopyWhoseReadStoppedOnTheSizeBoundIsOverTheBudget(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return nil, &gitrepo.BoundError{Kind: gitrepo.BoundStdout} }

	problem, err := committedCopyProblem(run, "repo", "HEAD", "p.json")

	if err != nil || !strings.Contains(problem, "size budget") {
		t.Errorf("committedCopyProblem = %q, %v; want the size budget named and no failure", problem, err)
	}
}
