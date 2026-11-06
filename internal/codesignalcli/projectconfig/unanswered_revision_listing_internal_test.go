package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

var errRevisionListing = errors.New("exit status 128: fatal: unable to read tree 0123456789")

func listingFailsWithGitsWords(string) ([]string, error) { return nil, errRevisionListing }

// The revision's own listing is what says whether a policy is committed in another case. A listing git could
// not give leaves that unknown, and unknown is not "no": the advice would be to author or commit a policy
// that may already be there.
func assertUnansweredListing(t *testing.T, got *ConfigError, words ...string) {
	t.Helper()
	if got == nil {
		t.Fatal("no refusal; want one that says git could not be asked")
	}
	if got.Kind != KindUnreadable || got.PolicyCause != projectreadiness.CauseGitUnreadable || got.Remedy != projectreadiness.RemedyNone {
		t.Errorf("refusal = kind %v, cause %q, remedy %v; want unreadable, %q and no remedy: nothing is offered over a policy that may be there", got.Kind, got.PolicyCause, got.Remedy, projectreadiness.CauseGitUnreadable)
	}
	for _, want := range append([]string{"Coach could not check whether the analyzed revision holds it in another case", "unable to read tree", "so it gives no advice"}, words...) {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message %q; want it to contain %q", got.Message, want)
		}
	}
}

func TestAPolicyMissingFromDiskIsNotCalledPlainlyMissingWhenTheRevisionCannotBeListed(t *testing.T) {
	got := respelledInRevision(listingFailsWithGitsWords, "cfg/P.json")

	assertUnansweredListing(t, got, `--project-config "cfg/P.json" is not on disk`)
}

func TestADiscoveredPolicyIsNotCalledPlainlyMissingWhenTheRevisionCannotBeListed(t *testing.T) {
	got := wrongCaseOf(listingFailsWithGitsWords, ".lousy-agents/coach.json")

	assertUnansweredListing(t, got, `nothing is committed at ".lousy-agents/coach.json"`)
}

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
		if f.ignoreCaseErr != nil {
			return nil, f.ignoreCaseErr
		}
		if f.ignoreCase == "" {
			return nil, gitSaysUnset
		}
		return []byte(f.ignoreCase + "\n"), nil
	case "ls-tree":
		return nil, f.listingErr
	}
	return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
}

func TestAnUncommittedPolicyIsGivenNoAdviceWhenGitCannotAnswerWhetherTheRevisionHoldsItInAnotherCase(t *testing.T) {
	cases := []struct {
		name  string
		fake  uncommittedPolicyFake
		words string
	}{
		{"the revision cannot be listed", uncommittedPolicyFake{ignoreCase: "true", listingErr: errRevisionListing}, "whether the analyzed revision holds it in another case"},
		{"git cannot say whether it ignores case", uncommittedPolicyFake{ignoreCaseErr: errors.New("exit status 128: fatal: bad config line 1"), listingErr: nil}, "whether git ignores case"},
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
	for _, want := range []string{"Coach could not check " + words, "so it gives no advice"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q; want it to contain %q", message, want)
		}
	}
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
			message, cause, err := UncommittedPolicy(fake.run, "repo", "HEAD", "policies/P.json")

			if err != nil || cause != projectreadiness.CauseNotCommitted || !strings.Contains(message, "commit the file") {
				t.Errorf("UncommittedPolicy = %q, %q, %v; want the advice to commit the file", message, cause, err)
			}
		})
	}
}
