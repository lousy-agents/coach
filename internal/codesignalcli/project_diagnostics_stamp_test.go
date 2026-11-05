package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestProjectDiagnosticsAreStampedWithTheirSide(t *testing.T) {
	input := []codesignal.Diagnostic{{Kind: "project_layer_bypass_coverage_incomplete", Path: "project.json", Message: "m"}}
	cases := []struct {
		name  string
		stamp func([]codesignal.Diagnostic, string) []codesignal.Diagnostic
		want  []codesignal.Diagnostic
	}{
		{"head keeps its kind", headProjectDiagnostics, []codesignal.Diagnostic{{Kind: "project_layer_bypass_coverage_incomplete", Path: "project.json", Message: "m", Side: "head", Revision: "rev1"}}},
		{"base marks its kind", baseProjectDiagnostics, []codesignal.Diagnostic{{Kind: "base_project_layer_bypass_coverage_incomplete", Path: "project.json", Message: "m", Side: "base", Revision: "rev1"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertStamped(t, c.stamp, input, c.want)
		})
	}
}

func assertStamped(t *testing.T, stamp func([]codesignal.Diagnostic, string) []codesignal.Diagnostic, input, want []codesignal.Diagnostic) {
	t.Helper()
	if got := stamp(input, "rev1"); !reflect.DeepEqual(got, want) {
		t.Errorf("stamped = %#v, want %#v", got, want)
	}
	if input[0].Side != "" || input[0].Revision != "" {
		t.Errorf("the input diagnostic was changed: %#v", input[0])
	}
	if got := stamp(nil, "rev1"); got != nil {
		t.Errorf("no diagnostics stamped to %#v, want nil", got)
	}
}
