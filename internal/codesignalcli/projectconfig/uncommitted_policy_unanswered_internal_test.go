package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// uncommittedPolicyFake answers what UncommittedPolicy asks: that the path lies in no submodule, what
// core.ignorecase is, and what the revision's tree holds.
type uncommittedPolicyFake struct {
	ignoreCase    string
	ignoreCaseErr error
	listingErr    error
}

func (f uncommittedPolicyFake) run(_ string, args ...string) ([]byte, error) {
	switch args[0] {
	case "ls-files":
		return nil, nil
	case "config":
		return f.answerIgnoreCase()
	case "ls-tree":
		return nil, f.listingErr
	}
	return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
}

func (f uncommittedPolicyFake) answerIgnoreCase() ([]byte, error) {
	if f.ignoreCaseErr != nil {
		return nil, f.ignoreCaseErr
	}
	if f.ignoreCase == "" {
		return nil, gitSaysUnset
	}
	return []byte(f.ignoreCase + "\n"), nil
}

func TestAnUncommittedPolicyIsGivenNoAdviceWhenGitCannotAnswerWhetherTheRevisionHoldsItInAnotherCase(t *testing.T) {
	cases := []struct {
		name  string
		fake  uncommittedPolicyFake
		words string
	}{
		{"the revision cannot be listed", uncommittedPolicyFake{ignoreCase: "true", listingErr: errRevisionListing}, listingQuestion},
		{"git cannot say whether it ignores case", uncommittedPolicyFake{ignoreCaseErr: errors.New("exit status 128: fatal: bad config line 1")}, "whether git ignores case"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertUncommittedPolicyUnanswered(t, c.fake, c.words)
		})
	}
}

func assertUncommittedPolicyUnanswered(t *testing.T, fake uncommittedPolicyFake, words string) {
	t.Helper()

	message, cause, err := UncommittedPolicy(fake.run, "repo", "HEAD", "policies/P.json")

	if err == nil || cause != projectreadiness.CauseGitUnreadable {
		t.Errorf("UncommittedPolicy = cause %q, error %v; want the unanswered question, with the cause that says git could not answer", cause, err)
	}
	requireContains(t, message, "Coach could not check "+words, "so it gives no advice")
	if strings.Contains(message, "commit the file") {
		t.Errorf("message %q still tells the user to commit a file that may already be there under another case", message)
	}
}

// An unset core.ignorecase is git's plain "no": the repository tells cases apart, and committing is right.
func TestAnUncommittedPolicyIsToldToCommitItWhenGitSaysItDoesNotIgnoreCase(t *testing.T) {
	cases := map[string]uncommittedPolicyFake{
		"core.ignorecase is unset": {},
		"core.ignorecase is false": {ignoreCase: "false"},
	}
	for name, fake := range cases {
		t.Run(name, func(t *testing.T) {
			assertToldToCommit(t, fake)
		})
	}
}

func assertToldToCommit(t *testing.T, fake uncommittedPolicyFake) {
	t.Helper()

	message, cause, err := UncommittedPolicy(fake.run, "repo", "HEAD", "policies/P.json")

	if err != nil || cause != projectreadiness.CauseNotCommitted || !strings.Contains(message, "commit the file") {
		t.Errorf("UncommittedPolicy = %q, %q, %v; want the advice to commit the file", message, cause, err)
	}
}
