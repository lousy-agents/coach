package codesignalcli

import (
	"testing"
)

func body_projectTsCompilerResolvePart4Test_28(t *testing.T, tc struct {
	name  string
	value string
	want  bool
}) {
	if got := isExactVersion(tc.value); got != tc.want {
		t.Errorf("isExactVersion(%q) = %v, want %v", tc.value, got, tc.want)
	}
}
