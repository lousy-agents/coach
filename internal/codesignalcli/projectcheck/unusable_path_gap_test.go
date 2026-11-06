package projectcheck

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A path that cannot name a policy is refused as a usage error before any readiness check runs, so no report
// carries a word for it. Were one to reach a check anyway, the gap has no way out and says so with the word for
// a state Coach has no other word for, and not with a word of its own that a consumer could come to match on.
func TestAGapFromAPathThatCannotNameAPolicyCarriesNoWordOfItsOwn(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")

	_, loadErr := projectconfig.LoadForReadiness(repo, "HEAD", "../x.json")

	check, _, err := policyGapFrom(loadErr)

	if err != nil || check.Code != projectreadiness.GapPolicyInvalid || check.Remedy != projectreadiness.RemedyNone {
		t.Fatalf("policyGapFrom = %+v, %v; want a policy_invalid gap that offers nothing", check, err)
	}
	if check.PolicyCause != projectreadiness.CauseUndetermined {
		t.Errorf("cause = %q; want %q", check.PolicyCause, projectreadiness.CauseUndetermined)
	}
}
