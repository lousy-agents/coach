package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Every refusal of a policy committed under other spellings than the one Coach reads says so in the same stable word,
// whether the revision holds one other spelling or several: the fix is a rename in each, and a reader matches on the
// word, not on the sentence around it.
func TestEveryRefusalOfAPolicyInAnotherCaseTakesTheSameCause(t *testing.T) {
	one := fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries
	several := fakeRevision{"": {"cfg"}, "cfg": {"p.json", "P.JSON"}}.entries
	for name, refusal := range map[string]error{
		"one other spelling":      wrongCaseOf(one, "cfg/P.json"),
		"several other spellings": wrongCaseOf(several, "cfg/p.JSON"),
	} {
		t.Run(name, func(t *testing.T) { requireTheCaseCause(t, refusal) })
	}
}

func requireTheCaseCause(t *testing.T, verdict error) {
	t.Helper()
	if verdict == nil {
		t.Fatal("no refusal")
	}
	refusal := configErrorOf(t, verdict)
	if refusal.PolicyCause != projectreadiness.PolicyCause("name_case_differs") || refusal.Remedy != projectreadiness.RemedyNone {
		t.Errorf("cause %q, remedy %d in %q; want name_case_differs and no remedy", refusal.PolicyCause, refusal.Remedy, refusal.Message)
	}
}
