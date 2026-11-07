package projectcheck

import (
	"errors"
	"slices"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

var errNoAnswer = errors.New("git has no answer")

// holdsNoBlobAndCannotSayWhatElse answers like a repository that has a commit and no blob at the
// path, and fails the one question that tells a directory from nothing.
func holdsNoBlobAndCannotSayWhatElse(_ string, args ...string) ([]byte, error) {
	if slices.Contains(args, "-z") {
		return nil, errNoAnswer
	}
	return nil, nil
}

// A path with no blob is missing only if nothing else is there, and when git cannot say what else
// is there the answer is an operational error, not a gap that offers a new policy over a directory.
func TestAPathWithNoBlobIsNotCalledMissingWhenGitCannotSayWhatElseIsThere(t *testing.T) {
	run := holdsNoBlobAndCannotSayWhatElse

	check, _, err := checkPolicy(run, "repo", "HEAD", nonFilePolicy)

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("checkPolicy = %+v, %v; want an *OperationalError", check, err)
	}
}

// A rejection of the committed policy is a gap with the way out it offers, and anything else that
// stopped the read is an error the caller reports, never a gap.
func TestOnlyARejectionOfThePolicyBecomesAGap(t *testing.T) {
	invalid := &projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyAuthor, Message: "not valid JSON"}
	operational := &gitrepo.OperationalError{Message: "git failed"}

	check, _, err := policyGapFrom(invalid)
	if err != nil || check.Code != projectreadiness.GapPolicyInvalid || check.Remedy != projectreadiness.RemedyAuthor || check.PolicyCause != "" {
		t.Errorf("policyGapFrom(invalid content) = %+v, %v; want a policy_invalid gap that offers a new policy and names no cause", check, err)
	}
	if _, _, err := policyGapFrom(operational); !errors.Is(err, operational) {
		t.Errorf("policyGapFrom(operational error) = %v; want the error itself", err)
	}
}
