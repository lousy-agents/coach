package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

// answeringExcept is a revision that answers every question of its tree but the one numbered failing, counted from
// one: the revision of a git that gave one answer and could not give the next.
func answeringExcept(revision fakeRevision, failing int) revisionEntries {
	asked := 0
	return func(directory string) ([]string, error) {
		asked++
		if asked == failing {
			return nil, errors.New("exit status 128: fatal: unable to read tree")
		}
		return revision.entries(directory)
	}
}

// A mismatch is worded from what the revision holds under each of its two spellings. When git answers one of
// those two questions and not the other, what the revision holds is not known, and a clause built from the half
// that was answered ("it holds neither", or a spelling to pass) would point the wrong way: advice built from the
// user's own argument has to be withheld, as it is when neither is answered.
func TestAMismatchIsNotWordedFromAHalfAnsweredRevision(t *testing.T) {
	revision := fakeRevision{"": {"cfg"}, "cfg": {"P.json"}}
	for failing, which := range map[int]string{1: "the first question fails", 2: "the second question fails"} {
		assertNoClauseFromAHalfAnsweredRevision(t, which, answeringExcept(revision, failing))
	}
}

func assertNoClauseFromAHalfAnsweredRevision(t *testing.T, name string, entries revisionEntries) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		mismatch := newCaseMismatch("cfg/p.json", []string{"cfg", "p.json"}, 1, "P.json")

		message := mismatch.config(entries).Message

		if strings.Contains(message, ": it holds") || strings.Contains(message, "; pass") || strings.Contains(message, "rename") {
			t.Errorf("message %q; want the two spellings and no advice, because the revision was only half asked", message)
		}
		if !strings.Contains(message, `is stored as "P.json" on disk, not "p.json"`) {
			t.Errorf("message %q; want the two spellings", message)
		}
	})
}
