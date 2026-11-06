package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A discovered policy gets guided authoring only when the error says a new policy is the way out. A
// committed link, or any other refusal that shares the kind of an invalid document, has its own
// message and is not replaced by a session that writes a file the scan would refuse again.
func TestDiscoveredPolicyAuthoringIsOfferedOnlyWhereANewPolicyIsTheWayOut(t *testing.T) {
	flags := codesignalFlags{projectLanguage: "typescript", namedPolicy: policyref.Ref{}}
	cases := map[string]struct {
		err  *projectconfig.ConfigError
		want bool
	}{
		"a document that is not valid":      {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyAuthor}, true},
		"a policy committed as a link":      {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseCommittedAsLink}, false},
		"a policy committed as a directory": {&projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseDirectory}, false},
		"a policy that is missing":          {&projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: projectreadiness.RemedyAuthor}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireDiscoveredAuthorable(t, c.err, flags, c.want) })
	}
}

func requireDiscoveredAuthorable(t *testing.T, err *projectconfig.ConfigError, flags codesignalFlags, want bool) {
	t.Helper()
	if got := discoveredPolicyAuthorable(err, flags); got != want {
		t.Errorf("discoveredPolicyAuthorable(kind %d, remedy %d) = %v, want %v", err.Kind, err.Remedy, got, want)
	}
}
