package codesignal

import (
	"context"
	"encoding/json"

	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_propertyPart2Test_33(t *testing.T, tt struct {
	name     string
	evidence string
}) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "evidence.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Adversarial", Evidence: tt.evidence},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "evidence.go", Status: "modified", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report) with Evidence %q must not fail: %v", tt.evidence, err)
	}

	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("json.Unmarshal back into map[string]any with Evidence %q must not fail: %v\nJSON: %s", tt.evidence, err, raw)
	}
}
