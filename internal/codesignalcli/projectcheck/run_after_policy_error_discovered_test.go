package projectcheck

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A discovered policy is judged at the revision, as Run judges it, whatever error the caller holds: nothing
// about the working tree decides it, and a policy that is not committed is answered with a new one, not with
// the "commit the file" a named policy gets.
func TestRunAfterPolicyErrorForADiscoveredPolicyIsRun(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "package.json", `{"name":"x"}`)

	result, err := RunAfterPolicyError(repo, "HEAD", policyref.Discovered(""), nil)

	if err != nil || result == nil {
		t.Fatalf("RunAfterPolicyError = %v, %v", result, err)
	}
	policy := result.Checks.Policy
	if policy.Code != projectreadiness.GapPolicyMissing || policy.Remedy != projectreadiness.RemedyAuthor || policy.PolicyCause != "" {
		t.Errorf("policy check = %+v; want policy_missing answered with a new policy and no cause", policy)
	}
}
