package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestPerPathDisclosureNamesEachChangedFileOnceInPathOrder(t *testing.T) {
	sets := worktreePathSets{
		untracked: []string{"z.go", "b.go"},
		staged:    []string{"m.go", "b.go"},
		modified:  []string{"a.go"},
	}

	got := disclosedPaths(perPathDisclosure(sets, nil), codesignal.DiagKindWorktreeChangesNotAnalyzed)

	if want := []string{"a.go", "b.go", "m.go", "z.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("per-path disclosures name %v, want %v: ascending by path, a file changed in two ways once", got, want)
	}
}
