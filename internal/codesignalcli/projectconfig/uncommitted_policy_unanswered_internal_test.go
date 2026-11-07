package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// uncommittedPolicyFake is all of the git that UncommittedPolicy asks: whether the path lies in a submodule. Any other
// question is an error that names it, so a passing test shows nothing else was asked.
type uncommittedPolicyFake struct {
	submoduleErr error
	asked        []string
}

func (f *uncommittedPolicyFake) run(_ string, args ...string) ([]byte, error) {
	f.asked = append(f.asked, args[0])
	if args[0] == "ls-files" {
		return nil, f.submoduleErr
	}
	return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
}

// Whether the revision holds the path in another case is decided before a file on disk is called uncommitted, from the
// revision alone, so a policy that reaches this point needs only the commit: neither the revision's listing nor git's
// settings are asked again.
func TestAnUncommittedPolicyIsToldToCommitItAndNothingAboutCaseIsAsked(t *testing.T) {
	fake := &uncommittedPolicyFake{}

	message, cause, err := UncommittedPolicy(fake.run, "repo", "HEAD", "policies/P.json")

	if err != nil || cause != projectreadiness.CauseNotCommitted || !strings.Contains(message, "commit the file") {
		t.Errorf("UncommittedPolicy = %q, %q, %v; want the advice to commit the file", message, cause, err)
	}
	for _, question := range fake.asked {
		if question != "ls-files" {
			t.Errorf("git was asked %v; want the submodule question alone", fake.asked)
		}
	}
}

func TestAnUncommittedPolicyIsGivenNoAdviceWhenGitCannotAnswerWhetherItLiesInASubmodule(t *testing.T) {
	fake := &uncommittedPolicyFake{submoduleErr: errors.New("exit status 128: fatal: bad index")}

	message, cause, err := UncommittedPolicy(fake.run, "repo", "HEAD", "policies/P.json")

	if err == nil || cause != projectreadiness.CauseGitUnreadable {
		t.Errorf("UncommittedPolicy = cause %q, error %v; want the unanswered question, with the cause that says git could not answer", cause, err)
	}
	requireContains(t, message, "Coach could not check whether it lies inside a submodule", "so it gives no advice")
	if strings.Contains(message, "commit the file") {
		t.Errorf("message %q still tells the user to commit a file that may not be committable", message)
	}
}
