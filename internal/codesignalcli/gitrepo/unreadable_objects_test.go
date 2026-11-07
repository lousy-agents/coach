package gitrepo

import (
	"strings"
	"testing"
)

const (
	damagedObject = "0187f3b09d66cef7911083ad49f890f5a6589589"
	absentObject  = "73483eb558c9c61bf4071282f539d481349db693"
)

// `git cat-file --batch` answers "<id> missing" for an object that is not in the store and for one that is and
// cannot be read. The second is told apart by the error git prints beside it, and only an error that names the
// object counts: a warning is not an error, and an error about another object says nothing of this one.
func TestAnObjectGitNamesInAnErrorIsDamagedAndNotMissing(t *testing.T) {
	outcome := objectOutcome{missing: []string{damagedObject, absentObject}}

	outcome.separateUnreadable("error: inflate: data stream error (incorrect header check)\nerror: unable to unpack " + damagedObject + " header\n")

	if len(outcome.unreadable) != 1 || outcome.unreadable[0] != damagedObject {
		t.Errorf("unreadable = %v; want only the object the error names", outcome.unreadable)
	}
	if len(outcome.missing) != 1 || outcome.missing[0] != absentObject {
		t.Errorf("missing = %v; want the object no error names to stay missing", outcome.missing)
	}
	if !strings.Contains(outcome.said, "incorrect header check") || !strings.Contains(outcome.said, "unable to unpack "+damagedObject) {
		t.Errorf("said = %q; want git's own words", outcome.said)
	}
}

func TestAWarningIsNotAnErrorAndAnErrorAboutAnotherObjectLeavesThisOneMissing(t *testing.T) {
	cases := []struct{ name, stderr string }{
		{"a warning that names it", "warning: lazy fetching disabled; some objects may not be available: " + absentObject + "\n"},
		{"an error that names another object", "error: unable to unpack " + damagedObject + " header\n"},
		{"nothing at all", ""},
		{"a line that only contains the word error", "this is not an error: " + absentObject + "\n"},
	}
	for _, c := range cases {
		assertStaysMissing(t, c.name, c.stderr)
	}
}

func assertStaysMissing(t *testing.T, name, stderr string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		outcome := objectOutcome{missing: []string{absentObject}}

		outcome.separateUnreadable(stderr)

		if len(outcome.unreadable) != 0 || len(outcome.missing) != 1 || outcome.said != "" {
			t.Errorf("outcome = %+v; want the object to stay missing and nothing said", outcome)
		}
	})
}

func TestWhatGitSaidAboutADamagedObjectIsCutAndMadeSafeForATerminal(t *testing.T) {
	outcome := objectOutcome{missing: []string{damagedObject}}

	outcome.separateUnreadable("error: unable to unpack " + damagedObject + " header\nerror: " + strings.Repeat("x", 1000) + "\nfatal: path \x1b]0;PWNED\x07\n")

	if len([]rune(outcome.said)) > maxSaidRunes+1 {
		t.Errorf("said is %d characters; want it cut near %d", len([]rune(outcome.said)), maxSaidRunes)
	}
	if strings.ContainsAny(outcome.said, "\x1b\x07") {
		t.Errorf("said = %q holds a control character git printed", outcome.said)
	}
}

func TestAFatalLineThatNamesAnObjectMakesItDamagedToo(t *testing.T) {
	outcome := objectOutcome{missing: []string{damagedObject}}

	outcome.separateUnreadable("fatal: loose object " + damagedObject + " (stored in .git/objects/01/87f3) is corrupt\n")

	if len(outcome.unreadable) != 1 || len(outcome.missing) != 0 || !strings.Contains(outcome.said, "is corrupt") {
		t.Errorf("outcome = %+v; want the object damaged and git's words kept", outcome)
	}
}
