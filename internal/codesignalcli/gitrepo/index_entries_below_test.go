package gitrepo

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

// Whether anything is below a path takes one entry to answer, so the question is not held to the
// budget a policy document is: a listing that overruns the output budget answers it.
func TestIndexEntriesTakesAnOverrunOfTheBelowListingAsAnAnswer(t *testing.T) {
	overrun := &BoundError{Kind: BoundStdout, message: "git stdout exceeded the budget"}
	run := func(_ string, args ...string) ([]byte, error) {
		if slices.Contains(args, "--stage") {
			return nil, nil
		}
		return nil, overrun
	}

	got, err := IndexEntries(run, "repo", "pol.json")

	if err != nil || !got.HoldsBelow {
		t.Errorf("IndexEntries = %+v, %v; want HoldsBelow: a listing past the budget holds more than one entry", got, err)
	}
}

func TestIndexEntriesFailsClosedWhenTheBelowQuestionFailsForAnyOtherReason(t *testing.T) {
	cases := map[string]error{
		"a timeout":       &BoundError{Kind: BoundTimeout, message: "git ran past its time"},
		"too much stderr": &BoundError{Kind: BoundStderr, message: "git wrote too much to stderr"},
		"git failing":     errors.New("exit status 128"),
	}
	for name, failure := range cases {
		t.Run(name, func(t *testing.T) { requireTheBelowQuestionFailsWith(t, failure) })
	}
}

func requireTheBelowQuestionFailsWith(t *testing.T, failure error) {
	t.Helper()
	run := func(_ string, args ...string) ([]byte, error) {
		if slices.Contains(args, "--stage") {
			return nil, nil
		}
		return nil, failure
	}
	got, err := IndexEntries(run, "repo", "pol.json")
	var operational *OperationalError
	if !errors.As(err, &operational) || got.HoldsBelow {
		t.Errorf("IndexEntries = %+v, %v; want an *OperationalError and no answer", got, err)
	}
}

func TestIndexEntriesRefusesABelowListingThatNamesAnotherPath(t *testing.T) {
	run := answeringEach(nil, records("pol.jsonx/a.json"))

	got, err := IndexEntries(run, "repo", "pol.json")

	var operational *OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("IndexEntries = %+v, %v; want an *OperationalError: an entry for another path is not evidence about this one", got, err)
	}
}

// The entries for the path are asked for with the entries below it left out, and the question
// about what is below is asked only when the path has none of its own.
func TestIndexEntriesAsksTheExactQuestionFirstAndTheBelowQuestionOnlyWhenNeeded(t *testing.T) {
	var asked [][]string
	run := func(_ string, args ...string) ([]byte, error) {
		asked = append(asked, args)
		return nil, nil
	}

	if _, err := IndexEntries(run, "repo", "a[1]*.json"); err != nil {
		t.Fatalf("IndexEntries: %v", err)
	}

	want := [][]string{
		{"ls-files", "--stage", "-t", "-z", "--full-name", "--", ":(top,literal)a[1]*.json", ":(top,exclude,literal)a[1]*.json/"},
		{"ls-files", "-z", "--full-name", "--", ":(top,literal)a[1]*.json/"},
	}
	if !reflect.DeepEqual(asked, want) {
		t.Errorf("git was asked %v; want %v", asked, want)
	}
}
