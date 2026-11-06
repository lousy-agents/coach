package tssetup

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The scan has already judged the named policy from the working tree and holds
// the error that says so. The readiness printed beside that error has to report
// the same verdict, not read the working tree a second time and possibly pass
// the file the scan just refused.
func TestWrapProjectConfigErrorWithReadinessReportsTheVerdictItWasGiven(t *testing.T) {
	const policyPath = "project.json"
	repo := gitfixture.Init(t)
	head := gitfixture.CommitFile(t, repo, policyPath, `{"schema_version":"1","roots":["."]}`+"\n")
	given := &projectconfig.ConfigError{
		Kind:            projectconfig.KindMissing,
		FromWorkingTree: true,
		Remedy:          projectreadiness.RemedyNone,
		Message:         "the scan could not use the policy",
	}

	wrapped := WrapProjectConfigErrorWithReadiness(given, repo, head, policyref.Named(policyPath))

	var withReadiness *ProjectConfigErrorWithReadiness
	if !errors.As(wrapped, &withReadiness) {
		t.Fatalf("got %v, want a *ProjectConfigErrorWithReadiness", wrapped)
	}
	policy := withReadiness.Readiness.Checks.Policy
	if policy.State != projectreadiness.Fail || policy.Code != projectreadiness.GapPolicyMissing || policy.Remedy != projectreadiness.RemedyNone {
		t.Errorf("readiness policy check = {State: %q, Code: %q, Remedy: %v}, want the given verdict {fail, %s, RemedyNone}; a pass here means the policy was read from disk again", policy.State, policy.Code, policy.Remedy, projectreadiness.GapPolicyMissing)
	}
}
