package render

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The text prints a gap's fields in the order the JSON declares them, from the same
// struct, so the two cannot differ about which gap carries what.
func TestGapLineCarriesTheFieldsTheJSONDeclares(t *testing.T) {
	cases := map[string]struct {
		gap  projectreadiness.Gap
		want string
	}{
		"a code alone":                        {projectreadiness.Gap{Code: "policy_missing"}, "policy_missing"},
		"a package manager kind":              {projectreadiness.Gap{Code: "package_manager_ambiguous", PackageManagerKind: "mise_project"}, "package_manager_ambiguous package_manager_kind=mise_project"},
		"a cause":                             {projectreadiness.Gap{Code: "policy_missing", Cause: projectreadiness.CauseNotCommitted}, "policy_missing cause=not_committed"},
		"both, the kind first as in the JSON": {projectreadiness.Gap{Code: "x", PackageManagerKind: "k", Cause: projectreadiness.CauseDirectory}, "x package_manager_kind=k cause=directory"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireGapLine(t, c.gap, c.want) })
	}
}

func requireGapLine(t *testing.T, gap projectreadiness.Gap, want string) {
	t.Helper()
	if got := gapLine(gap); got != want {
		t.Errorf("gapLine(%+v) = %q, want %q", gap, got, want)
	}
}
