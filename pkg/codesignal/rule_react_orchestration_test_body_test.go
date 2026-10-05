package codesignal

import "testing"

func body_ruleReactOrchestrationTest_33(t *testing.T, tc struct {
	binding string
	want    string
}) {
	got := classifyStateDomain(tc.binding)
	if got != tc.want {
		t.Errorf("classifyStateDomain(%q) = %q, want %q", tc.binding, got, tc.want)
	}
}
