package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

var errRevisionListing = errors.New("exit status 128: fatal: unable to read tree 0123456789")

func listingFailsWithGitsWords(string) ([]string, error) { return nil, errRevisionListing }

func requireContains(t *testing.T, message string, words ...string) {
	t.Helper()
	for _, want := range words {
		if !strings.Contains(message, want) {
			t.Errorf("message %q; want it to contain %q", message, want)
		}
	}
}

// The revision's own listing is what says whether a policy is committed in another case. A listing git could
// not give leaves that unknown, and unknown is not "no": the advice would be to author or commit a policy
// that may already be there.
func assertUnansweredListing(t *testing.T, got *ConfigError, words ...string) {
	t.Helper()
	if got.Kind != KindUnreadable || got.PolicyCause != projectreadiness.CauseGitUnreadable || got.Remedy != projectreadiness.RemedyNone {
		t.Errorf("refusal = kind %v, cause %q, remedy %v; want unreadable, %q and no remedy: nothing is offered over a policy that may be there", got.Kind, got.PolicyCause, got.Remedy, projectreadiness.CauseGitUnreadable)
	}
	requireContains(t, got.Message, append([]string{"Coach could not check " + listingQuestion, "unable to read tree", "so it gives no advice"}, words...)...)
}

func TestADiscoveredPolicyIsNotCalledPlainlyMissingWhenTheRevisionCannotBeListed(t *testing.T) {
	got := wrongCaseOf(listingFailsWithGitsWords, ".lousy-agents/coach.json")

	assertUnansweredListing(t, configErrorOf(t, got), `nothing is committed at ".lousy-agents/coach.json"`)
}
