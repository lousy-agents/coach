package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_propertyTest_69(t *testing.T, tt struct {
	name   string
	ranges []LineRange
	loc    semantics.Location
}) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "extreme.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Extreme", Location: tt.loc, Evidence: "x = 1"},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "extreme.go", Status: "modified", Head: head, ChangedRanges: tt.ranges},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if report == nil {
		t.Fatalf("Build returned nil Report")
	}
}
