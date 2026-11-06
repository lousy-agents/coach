package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

// maxGitErrorRunes is a bound, not a quota: what git said at exactly the bound is quoted whole.
func TestWhatGitSaidAtExactlyTheBoundIsNotCut(t *testing.T) {
	said := strings.Repeat("x", maxGitErrorRunes)

	got := newGitFailure("what the index holds", errors.New(said)).describe()

	if strings.Contains(got, "…") || !strings.Contains(got, said) {
		t.Errorf("describe = %q; want all %d characters of what git said and no cut mark", got, maxGitErrorRunes)
	}
}

func TestWhatGitSaidOverTheBoundIsCutAndMarked(t *testing.T) {
	said := strings.Repeat("x", maxGitErrorRunes+1)

	got := newGitFailure("what the index holds", errors.New(said)).describe()

	if !strings.Contains(got, strings.Repeat("x", maxGitErrorRunes)+"…") || strings.Contains(got, said) {
		t.Errorf("describe = %q; want the first %d characters and a cut mark", got, maxGitErrorRunes)
	}
}
