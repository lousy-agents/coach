package codesignalcli

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func keepEverything(files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) { return files, nil }

func keepNothing([]gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) { return nil, nil }

func failToFilter([]gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) {
	return nil, errors.New("scope filter unavailable")
}

func TestHidesInScopeFailsClosedWhenTheScopeCannotBeConsulted(t *testing.T) {
	tests := []struct {
		name           string
		scope          ScopeFilter
		selectedPolicy string
		hidden         []string
		want           string
	}{
		{"a source file with no scope filter is unknown", nil, "", []string{"lib/a.go"}, codesignal.HidesInScopeUnknown},
		{"a source file whose scope filter fails is unknown", failToFilter, "", []string{"lib/a.go"}, codesignal.HidesInScopeUnknown},
		{"a source file the scope filter drops is no", keepNothing, "", []string{"lib/a.go"}, codesignal.HidesInScopeNo},
		{"a source file the scope filter keeps is yes", keepEverything, "", []string{"lib/a.go"}, codesignal.HidesInScopeYes},
		{"a tsconfig.json outside node_modules is yes", keepNothing, "", []string{"x/tsconfig.json"}, codesignal.HidesInScopeYes},
		{"a tsconfig.json inside node_modules is no", keepEverything, "", []string{"node_modules/x/tsconfig.json"}, codesignal.HidesInScopeNo},
		{"a source file outside node_modules is yes", keepEverything, "", []string{"x/index.ts"}, codesignal.HidesInScopeYes},
		{"a source file inside node_modules is no", keepEverything, "", []string{"node_modules/x/index.ts"}, codesignal.HidesInScopeNo},
		{"the selected policy is yes whatever the scope filter says", keepNothing, "project.json", []string{"project.json"}, codesignal.HidesInScopeYes},
		{"the selected policy hidden in another Unicode normalization form is yes", keepNothing, "caf\u00e9/policy.json", []string{"cafe\u0301/policy.json"}, codesignal.HidesInScopeYes},
		{"a file that is not the selected policy is no", keepNothing, "project.json", []string{"other.json"}, codesignal.HidesInScopeNo},
		{"nothing hidden is no", nil, "", nil, codesignal.HidesInScopeNo},
	}
	for _, tt := range tests {
		view := &WorktreeView{scope: tt.scope}

		if got := view.hidesInScope(map[string][]string{"root": tt.hidden}, tt.selectedPolicy)["root"]; got != tt.want {
			t.Errorf("%s: hidesInScope(%q) = %q, want %q", tt.name, tt.hidden, got, tt.want)
		}
	}
}

func TestHidesInScopeJudgesEachRootByItsOwnPathsWhenTheScopeCannotBeConsulted(t *testing.T) {
	hidden := map[string][]string{"cfg": {"x/tsconfig.json"}, "src": {"lib/a.go"}, "doc": {"docs/a.md"}}
	want := map[string]string{
		"cfg": codesignal.HidesInScopeYes,
		"src": codesignal.HidesInScopeUnknown,
		"doc": codesignal.HidesInScopeNo,
	}
	for name, scope := range map[string]ScopeFilter{"no scope filter": nil, "failing scope filter": failToFilter} {
		view := &WorktreeView{scope: scope}

		if got := view.hidesInScope(hidden, ""); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: hidesInScope = %v, want %v", name, got, want)
		}
	}
}
