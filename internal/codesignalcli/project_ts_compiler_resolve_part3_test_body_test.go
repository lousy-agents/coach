package codesignalcli

import (
	"testing"
)

func body_projectTsCompilerResolvePart3Test_23(t *testing.T, tc struct {
	name  string
	value string
	want  []string
}) {
	got := parseMiseToolValue(tc.value)
	if len(got) != len(tc.want) {
		t.Fatalf("parseMiseToolValue(%q) = %v, want %v", tc.value, got, tc.want)
	}
	for i := range got {
		if got[i] != tc.want[i] {
			t.Errorf("parseMiseToolValue(%q)[%d] = %q, want %q", tc.value, i, got[i], tc.want[i])
		}
	}
}
