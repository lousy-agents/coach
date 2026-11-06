package gitrepo

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// indexQuestions answers the three listings IndexEntries can ask, each by what it is: the exact one
// (--stage, with the entries below the path left out), the one about what is below the path (no --stage), and
// the one for a submodule's own entry (--stage, nothing left out). A nil answer means "nothing listed"; a
// non-nil failure fails that question alone.
type indexQuestions struct {
	exact, below, named []byte
	failExact           error
	failBelow           error
	failNamed           error
}

func (q indexQuestions) run(_ string, args ...string) ([]byte, error) {
	switch {
	case !slices.Contains(args, "--stage"):
		return q.below, q.failBelow
	case slices.ContainsFunc(args, func(arg string) bool { return strings.HasPrefix(arg, ":(top,exclude") }):
		return q.exact, q.failExact
	}
	return q.named, q.failNamed
}

// A failure of one index question is never read as an index that holds nothing, whichever of the others
// would have answered: each question is failed alone, so a swallowed error cannot be hidden by another
// question that reports the same one.
func TestIndexEntriesFailsClosedWhicheverQuestionFailsAlone(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]indexQuestions{
		"the exact question":                          {failExact: boom},
		"the question about what is below":            {failBelow: boom},
		"the submodule's own entry, once it is named": {below: records("sub"), failNamed: boom},
	}
	for name, questions := range cases {
		t.Run(name, func(t *testing.T) { requireTheFailureReaches(t, questions, boom) })
	}
}

func requireTheFailureReaches(t *testing.T, questions indexQuestions, boom error) {
	t.Helper()
	got, err := IndexEntries(questions.run, "repo", "sub")

	var operational *OperationalError
	if !errors.As(err, &operational) || !errors.Is(operational.Cause, boom) {
		t.Errorf("IndexEntries = %+v, %v; want an *OperationalError caused by the failed question", got, err)
	}
}

// A submodule is found whole: the listing of what is below returns its own path, and the one question that
// does not leave it out returns its entry.
func TestIndexEntriesReadsASubmodulesEntryFromTheQuestionThatKeepsIt(t *testing.T) {
	questions := indexQuestions{below: records("sub"), named: records("H 160000 " + listingBlob + " 0\tsub")}

	got, err := IndexEntries(questions.run, "repo", "sub")

	if err != nil || len(got.Entries) != 1 || got.Entries[0].Mode != "160000" || got.HoldsBelow {
		t.Errorf("IndexEntries = %+v, %v; want the submodule's one 160000 entry", got, err)
	}
}
