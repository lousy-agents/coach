package tssetup

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestCompilerSetupOfferCarriesTheRemedyForAPolicyThatBlocksIt(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks:      projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing, Remedy: projectreadiness.RemedyRestore}},
		NextActions: []projectreadiness.NextAction{{Kind: projectreadiness.NextActionRestorePolicy}},
	}

	result := RunCompilerSetupOffer(context.Background(), "", "rev", policyref.Named("pol.json"), "typescript_compiler_missing", readiness, nil, nil)

	if !result.PolicyRequired || result.PolicyCode != projectreadiness.GapPolicyMissing || result.PolicyRemedy != projectreadiness.RemedyRestore {
		t.Errorf("result = %+v, want PolicyRequired with policy_missing and RemedyRestore", result)
	}
}

func TestCompilerSetupOfferCarriesNoRemedyWhenNeitherCanHelp(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyInvalid, Remedy: projectreadiness.RemedyNone}},
	}

	result := RunCompilerSetupOffer(context.Background(), "", "rev", policyref.Named("pol.json"), "typescript_compiler_missing", readiness, nil, nil)

	if !result.PolicyRequired || result.PolicyRemedy != projectreadiness.RemedyNone {
		t.Errorf("result = %+v, want PolicyRequired with RemedyNone", result)
	}
}
