package projectreadiness

import (
	"encoding/json"
	"testing"
)

// "The field is absent from every other gap" is the documented contract, so a gap without a cause has no
// "cause" key at all: a decode into a string cannot tell absent from empty, and a consumer matching on
// the key can.
func TestAGapWithoutACauseOmitsTheKey(t *testing.T) {
	cases := map[string]struct {
		gap     Gap
		wantKey bool
	}{
		"a gap with no cause":   {Gap{Code: "policy_missing"}, false},
		"a package manager gap": {Gap{Code: "package_manager_ambiguous", PackageManagerKind: "mise_project"}, false},
		"a gap with a cause":    {Gap{Code: "policy_missing", Cause: CauseDirectory}, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheKeyIs(t, c.gap, c.wantKey) })
	}
}

func requireTheKeyIs(t *testing.T, gap Gap, wantKey bool) {
	t.Helper()
	encoded, err := json.Marshal(gap)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, has := decoded["cause"]; has != wantKey {
		t.Errorf("%s: has a cause key = %v, want %v", encoded, has, wantKey)
	}
}
