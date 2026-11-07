package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A read that tripped a bound names a way out or the reason it has none, never both and never neither,
// because both come from the one decision: the size budget is a policy a smaller one replaces, and a time
// or stderr bound is a read that failed.
func TestABoundNamesItsRemedyOrItsCauseNeverBothAndNeverNeither(t *testing.T) {
	cases := map[string]struct {
		kind   gitrepo.BoundErrorKind
		remedy projectreadiness.PolicyRemedy
		cause  string
	}{
		"the output budget": {gitrepo.BoundStdout, projectreadiness.RemedyAuthor, ""},
		"the time bound":    {gitrepo.BoundTimeout, projectreadiness.RemedyNone, "git_unreadable"},
		"the stderr bound":  {gitrepo.BoundStderr, projectreadiness.RemedyNone, "git_unreadable"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireBoundOutcome(t, c.kind, c.remedy, c.cause) })
	}
}

func requireBoundOutcome(t *testing.T, kind gitrepo.BoundErrorKind, remedy projectreadiness.PolicyRemedy, cause string) {
	t.Helper()
	gotRemedy, gotCause := boundOutcome(&gitrepo.BoundError{Kind: kind})
	if gotRemedy != remedy || string(gotCause) != cause {
		t.Errorf("boundOutcome(kind %d) = remedy %d, cause %q; want remedy %d, cause %q", kind, gotRemedy, gotCause, remedy, cause)
	}
}
