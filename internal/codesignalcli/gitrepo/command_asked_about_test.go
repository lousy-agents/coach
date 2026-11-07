package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

// A bound error is put after the subcommand that did not answer, so "git execution timed out" says which
// question it was. A question of one word is a question, and a read with no arguments has none to name.
func TestAnAskedAboutOneWordQuestionIsNamed(t *testing.T) {
	bound := &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}

	err := askedAbout([]string{"version"}, bound)

	if err == nil || !strings.Contains(err.Error(), "git version") || !errors.Is(err, bound) {
		t.Errorf("askedAbout(version) = %v; want it to name git version and keep the bound error", err)
	}
}

func TestAReadWithNoArgumentsKeepsItsBoundErrorAndIsNotAPanic(t *testing.T) {
	bound := &BoundError{Kind: BoundTimeout}

	for _, args := range [][]string{nil, {}} {
		if err := askedAbout(args, bound); err != error(bound) {
			t.Errorf("askedAbout(%v) = %v; want the bound error as it is", args, err)
		}
	}
}

func TestAnErrorThatIsNotABoundIsReturnedAsItIs(t *testing.T) {
	boom := errors.New("exit status 128")

	if err := askedAbout([]string{"show", "HEAD:a"}, boom); err != error(boom) {
		t.Errorf("askedAbout = %v; want the error unchanged", err)
	}
}
