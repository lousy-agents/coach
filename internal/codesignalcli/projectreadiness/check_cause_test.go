package projectreadiness

import "testing"

// A check names a remedy or says why it names none, never both: WithCause is the gate for the
// sentence and for the stable word together.
func TestWithCauseKeepsTheCauseOnlyWhereNoRemedyNamesWhatToDo(t *testing.T) {
	cases := map[string]struct {
		remedy PolicyRemedy
		want   Check
	}{
		"no remedy":    {RemedyNone, Check{Remedy: RemedyNone, CauseText: "why", PolicyCause: CauseDirectory}},
		"a new policy": {RemedyAuthor, Check{Remedy: RemedyAuthor}},
		"a restore":    {RemedyRestore, Check{Remedy: RemedyRestore}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheGate(t, c.remedy, c.want) })
	}
}

func requireTheGate(t *testing.T, remedy PolicyRemedy, want Check) {
	t.Helper()
	got := Check{Remedy: remedy}.WithCause("why", CauseDirectory)
	if got.CauseText != want.CauseText || got.PolicyCause != want.PolicyCause || got.Remedy != want.Remedy {
		t.Errorf("Check{Remedy: %d}.WithCause = text %q, word %q; want text %q, word %q", remedy, got.CauseText, got.PolicyCause, want.CauseText, want.PolicyCause)
	}
}
