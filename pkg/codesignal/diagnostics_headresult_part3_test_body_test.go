package codesignal

import (
	"context"
	"testing"
)

func body_diagnosticsHeadresultPart3Test_10(t *testing.T, status ChangeStatus) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "new.go", Status: status, Head: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var missing []Diagnostic
	for _, d := range report.Diagnostics {
		if d.Kind == "missing_head_result" {
			missing = append(missing, d)
		}
	}
	if len(missing) != 1 {
		t.Fatalf("missing_head_result diagnostics for status %q: got %d, want 1: %+v", status, len(missing), missing)
	}
	if missing[0].Path != "new.go" {
		t.Errorf("Diagnostic.Path: got %q, want %q", missing[0].Path, "new.go")
	}
}
