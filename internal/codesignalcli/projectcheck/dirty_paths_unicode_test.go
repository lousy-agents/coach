package projectcheck

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

const (
	listedPrecomposed = "cfg/pé.json"  // git on macOS lists a name precomposed
	typedDecomposed   = "cfg/pé.json" // and a user can type it decomposed
)

// The path git status lists and the path the user typed are one path when they differ only in Unicode form,
// so the policy the checks decided from the disk leaves the dirty paths the same way in either spelling,
// and a policy named in the other form is still the policy's own path.
func TestTheDirtyPathsTreatTheTwoUnicodeFormsOfThePolicyNameAsOnePath(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{{Code: " D", Path: listedPrecomposed}}
	cases := map[string]struct {
		policy, readFromDisk string
		wantPaths            []string
	}{
		"read from disk, typed in the other form":     {typedDecomposed, typedDecomposed, nil},
		"read from disk, typed as listed":             {listedPrecomposed, listedPrecomposed, nil},
		"not read from disk, typed in the other form": {typedDecomposed, "", []string{listedPrecomposed}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireDirtyPaths(t, entries, c.policy, c.readFromDisk, c.wantPaths) })
	}
}

func requireDirtyPaths(t *testing.T, entries []gitrepo.WorktreeEntry, policy, readFromDisk string, want []string) {
	t.Helper()
	got := relevantDirtyPaths(entries, nil, policy, readFromDisk)
	if len(got) != len(want) || (len(want) == 1 && got[0] != want[0]) {
		t.Errorf("relevantDirtyPaths = %q, want %q", got, want)
	}
}
