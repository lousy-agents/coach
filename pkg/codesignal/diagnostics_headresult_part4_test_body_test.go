package codesignal

import (
	"context"
	"testing"
)

func body_diagnosticsHeadresultPart4Test_10(t *testing.T, status ChangeStatus) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "gone.go", Status: status, Head: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, d := range report.Diagnostics {
		if d.Kind == "missing_head_result" {
			t.Errorf("unexpected missing_head_result diagnostic for status %q: %+v", status, d)
		}
	}
}
