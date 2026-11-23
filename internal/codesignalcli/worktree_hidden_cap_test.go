package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestRootsAreOrderedBySeverityThenName(t *testing.T) {
	tests := []struct {
		name     string
		verdicts map[string]string
		want     []string
	}{
		{"yes before unknown before no", map[string]string{"a": codesignal.HidesInScopeNo, "b": codesignal.HidesInScopeUnknown, "c": codesignal.HidesInScopeYes}, []string{"c", "b", "a"}},
		{"alphabetical within a verdict", map[string]string{"z": codesignal.HidesInScopeYes, "m": codesignal.HidesInScopeYes, "a": codesignal.HidesInScopeYes}, []string{"a", "m", "z"}},
		{"a verdict that is not yes or no counts as unknown", map[string]string{"a": codesignal.HidesInScopeNo, "b": "future", "c": codesignal.HidesInScopeUnknown, "d": codesignal.HidesInScopeYes}, []string{"d", "b", "c", "a"}},
	}
	for _, tt := range tests {
		if got := orderRootsBySeverity(tt.verdicts); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: orderRootsBySeverity(%v) = %q, want %q", tt.name, tt.verdicts, got, tt.want)
		}
	}
}
