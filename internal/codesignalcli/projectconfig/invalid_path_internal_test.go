package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A path that cannot name a policy in this repository (one that climbs out of it, is absolute, is not
// normalized or spells a separator the wrong way) is not fixed by a new policy: authoring writes at its
// own location, and the path stays wrong. The error says what is wrong with the path and offers nothing.
func TestAPathThatCannotNameAPolicyOffersNoAuthoringAndSaysHowToSpellIt(t *testing.T) {
	repo := committedRepository(t)
	for _, path := range []string{"../../policies/p.json", "/etc/p.json", "cfg//p.json", `cfg\p.json`} {
		t.Run(path, func(t *testing.T) { requireNoAuthoringForPath(t, repo, path) })
	}
}

func requireNoAuthoringForPath(t *testing.T, repo, path string) {
	t.Helper()
	_, err := Load(repo, "HEAD", path)

	refusal, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("Load(%q) = %v (%T); want a *ConfigError", path, err, err)
	}
	if refusal.OffersAuthoring() || refusal.Remedy != projectreadiness.RemedyNone || refusal.PolicyCause != "" {
		t.Errorf("remedy %d, cause %q in %q; want no remedy and no word of the cause vocabulary", refusal.Remedy, refusal.PolicyCause, refusal.Message)
	}
}

func TestThePathErrorSaysThePathIsTakenFromTheRepositoryRoot(t *testing.T) {
	err := ValidatePath("../../policies/p.json")

	if err == nil || !strings.Contains(err.Error(), "from the repository root") {
		t.Errorf("ValidatePath = %v; want it to say that the path is taken from the repository root", err)
	}
}
