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
// so the policy the checks read leaves the dirty paths the same way in either spelling.
func TestTheDirtyPathsTreatTheTwoUnicodeFormsOfThePolicyNameAsOnePath(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{{Code: " D", Path: listedPrecomposed}}
	cases := map[string]string{
		"typed in the other form": typedDecomposed,
		"typed as listed":         listedPrecomposed,
	}
	for name, policy := range cases {
		t.Run(name, func(t *testing.T) {
			if got := relevantDirtyPaths(entries, nil, policy); len(got) != 0 {
				t.Errorf("relevantDirtyPaths = %q, want the policy's own path left out", got)
			}
		})
	}
}
