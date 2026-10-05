package codesignal

import (
	"testing"
)

func body_goldenPart2Test_NoFrozenNameMissing_35(t *testing.T, got map[string]struct{}) {
	for name := range frozenReportJSONFieldNames {
		if _, ok := got[name]; !ok {
			t.Errorf("expected JSON field %q not found among Report's reachable struct tags (renamed or removed?)", name)
		}
	}
}

func body_goldenPart2Test_NoUnexpectedNameAppeared_42(t *testing.T, got map[string]struct{}) {
	for name := range got {
		if _, ok := frozenReportJSONFieldNames[name]; !ok {
			t.Errorf("unexpected JSON field %q found among Report's reachable struct tags (new field, or a rename, not reflected in frozenReportJSONFieldNames)", name)
		}
	}
}
