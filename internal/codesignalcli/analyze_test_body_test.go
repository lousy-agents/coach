package codesignalcli

import (
	"testing"
)

func body_analyzeTest_66(t *testing.T, tt struct {
	name string
	err  error
	want string
}) {
	got := mapSemanticsError("some/path.go", tt.err)
	if got.Kind != tt.want {
		t.Errorf("mapSemanticsError(%v).Kind = %q, want %q", tt.err, got.Kind, tt.want)
	}
	if got.Path != "some/path.go" {
		t.Errorf("mapSemanticsError(%v).Path = %q, want %q", tt.err, got.Path, "some/path.go")
	}
	if got.Message == "" {
		t.Errorf("mapSemanticsError(%v).Message is empty, want non-empty", tt.err)
	}
}
