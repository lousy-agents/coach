package projectcheck

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// An error loading the discovered canonical policy is either a rejection of its content, which readiness
// classifies again with its own reader, or something that stopped the read, which is returned as it is.
func TestOnlyARejectionOfTheCanonicalPolicyIsClassifiedAgainByReadiness(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"invalid content":                     {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid}, true},
		"a read that tripped a bound":         {&projectconfig.ConfigError{Kind: projectconfig.KindUnreadable}, true},
		"a policy not found":                  {&projectconfig.ConfigError{Kind: projectconfig.KindNotFound}, false},
		"a policy missing from disk":          {&projectconfig.ConfigError{Kind: projectconfig.KindMissing}, false},
		"an operational error":                {&gitrepo.OperationalError{Message: "git failed"}, false},
		"an error that is not about a policy": {errors.New("boom"), false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireRecheck(t, c.err, c.want) })
	}
}

func requireRecheck(t *testing.T, err error, want bool) {
	t.Helper()
	if got := canonicalRecheckedByReadiness(err); got != want {
		t.Errorf("canonicalRecheckedByReadiness(%v) = %v, want %v", err, got, want)
	}
}

// A failure that readiness does not classify again stops the selection and is returned; one it does classify
// again is not returned, and the canonical path is the policy to check.
func TestAFailureReadinessDoesNotClassifyAgainStopsTheSelection(t *testing.T) {
	boom := &gitrepo.OperationalError{Message: "the partial clone lacks the blob"}

	_, _, err := policyFromSelection(projectconfig.Selection{Fatal: boom})

	if !errors.Is(err, boom) {
		t.Errorf("policyFromSelection = %v; want the error that stopped the read", err)
	}
}

func TestARejectionOfTheCanonicalPolicyLeavesItTheOneToCheck(t *testing.T) {
	path, _, err := policyFromSelection(projectconfig.Selection{Fatal: &projectconfig.ConfigError{Kind: projectconfig.KindInvalid}})

	if err != nil || path != projectconfig.CanonicalPath {
		t.Errorf("policyFromSelection = %q, %v; want the canonical path and no error", path, err)
	}
}
