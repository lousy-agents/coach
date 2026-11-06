package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Every refusal of a name that is spelled in another case than the revision holds says so in the same
// stable word, whether the disk resolved the name, the revision holds one other spelling of a missing
// path, or it holds several: the fix is a rename in each, and a reader matches on the word, not on the
// sentence around it.
func TestEveryRefusalOfANameInAnotherCaseTakesTheSameCause(t *testing.T) {
	spellings := fakeRevision{"": {"cfg"}, "cfg": {"p.json", "P.JSON"}}.entries
	respelled := fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries
	cases := map[string]*ConfigError{
		"a name the disk resolves":                    newCaseMismatch("cfg/P.json", []string{"cfg", "P.json"}, 1, "p.json").config(nil),
		"a missing path the revision holds respelled": respelledInRevision(respelled, "cfg/P.json"),
		"a missing path the revision holds twice":     respelledInRevision(spellings, "cfg/P.json"),
	}
	for name, refusal := range cases {
		t.Run(name, func(t *testing.T) { requireTheCaseCause(t, refusal) })
	}
}

func requireTheCaseCause(t *testing.T, refusal *ConfigError) {
	t.Helper()
	if refusal == nil {
		t.Fatal("no refusal")
	}
	if refusal.PolicyCause != projectreadiness.PolicyCause("name_case_differs") || refusal.Remedy != projectreadiness.RemedyNone {
		t.Errorf("cause %q, remedy %d in %q; want name_case_differs and no remedy", refusal.PolicyCause, refusal.Remedy, refusal.Message)
	}
}
