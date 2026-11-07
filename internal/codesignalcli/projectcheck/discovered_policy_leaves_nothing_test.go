package projectcheck

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The gap for a discovered policy that the revision does not commit is the policy that is missing, unless the
// working tree holds something a person would commit.
func TestWhatTheWorkingTreeHoldsAtTheDiscoveredPathDecidesWhetherThereIsAPolicyToCommit(t *testing.T) {
	cases := []struct {
		name   string
		status *projectconfig.ConfigError
		want   bool
	}{
		{"a regular file", nil, false},
		{"nothing at the path", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Missing: projectconfig.MissingAbsent, Remedy: projectreadiness.RemedyAuthor}, true},
		{"a copy staged in the index", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Missing: projectconfig.MissingAbsent, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseStagedCopy}, false},
		{"a conflict recorded in the index", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Missing: projectconfig.MissingAbsent, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseMergeConflict}, false},
		{"a path inside a submodule", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Missing: projectconfig.MissingAbsent, PolicyCause: projectreadiness.CauseInSubmodule}, false},
		{"a directory", &projectconfig.ConfigError{Kind: projectconfig.KindInvalid, PolicyCause: projectreadiness.CauseNotARegularFile}, false},
		{"a file where the directory should be", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Missing: projectconfig.MissingBlockedParent, PolicyCause: projectreadiness.CauseParentNotDirectory}, false},
	}
	for _, c := range cases {
		assertLeavesNoPolicyToCommit(t, c.name, c.status, c.want)
	}
}

func assertLeavesNoPolicyToCommit(t *testing.T, name string, status *projectconfig.ConfigError, want bool) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if got := leavesNoPolicyToCommit(status); got != want {
			t.Errorf("leavesNoPolicyToCommit(%+v) = %v; want %v", status, got, want)
		}
	})
}
