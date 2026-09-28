package codesignal

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestProperty_ReorderingInputDoesNotChangeReportJSON(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	original := reorderingScenarioInput()
	reordered := original
	reordered.Files = reverseFileChanges(original.Files)

	reportA, err := b.Build(context.Background(), original)
	if err != nil {
		t.Fatalf("Build(original): %v", err)
	}
	reportB, err := b.Build(context.Background(), reordered)
	if err != nil {
		t.Fatalf("Build(reordered): %v", err)
	}

	jsonA, err := json.Marshal(reportA)
	if err != nil {
		t.Fatalf("marshaling reportA: %v", err)
	}
	jsonB, err := json.Marshal(reportB)
	if err != nil {
		t.Fatalf("marshaling reportB: %v", err)
	}

	if string(jsonA) != string(jsonB) {
		t.Errorf("Build must be order-independent.\noriginal-order JSON:\n%s\nreordered JSON:\n%s", jsonA, jsonB)
	}
}

func TestProperty_RangeOverlapNeverPanics(t *testing.T) {
	const maxUint = uint(math.MaxUint32)

	tests := []struct {
		name   string
		ranges []LineRange
		loc    semantics.Location
	}{
		{"zero range, zero location", []LineRange{{StartRow: 0, EndRow: 0}}, semantics.Location{StartRow: 0, EndRow: 0}},
		{"huge range, zero location", []LineRange{{StartRow: 0, EndRow: maxUint}}, semantics.Location{StartRow: 0, EndRow: 0}},
		{"huge location, zero range", []LineRange{{StartRow: 0, EndRow: 0}}, semantics.Location{StartRow: maxUint, EndRow: maxUint}},
		{"start == end at max", []LineRange{{StartRow: maxUint, EndRow: maxUint}}, semantics.Location{StartRow: maxUint, EndRow: maxUint}},
		{"invalid range start > end", []LineRange{{StartRow: maxUint, EndRow: 0}}, semantics.Location{StartRow: 0, EndRow: maxUint}},
		{"huge gap between range and location", []LineRange{{StartRow: 0, EndRow: 1}}, semantics.Location{StartRow: maxUint, EndRow: maxUint}},
		{"no ranges at all", nil, semantics.Location{StartRow: maxUint, EndRow: maxUint}},
		{"many ranges", []LineRange{
			{StartRow: 0, EndRow: 0},
			{StartRow: maxUint, EndRow: maxUint},
			{StartRow: 5, EndRow: 3},
			{StartRow: 1000000, EndRow: 2000000},
		}, semantics.Location{StartRow: 1500000, EndRow: 1500000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_propertyTest_69(t, tt)
		})
	}
}

func reorderingScenarioInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "rev1", Base: "main"},
		Files: []FileChange{
			{
				Path:   "a.go",
				Status: "modified",
				Base: &semantics.Result{
					Path:        "a.go",
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Dup", Location: semantics.Location{StartRow: 1}, Evidence: "x = 1"},
					},
				},
				Head: &semantics.Result{
					Path:        "a.go",
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{

						{Kind: "mutates_input", Name: "Dup", Location: semantics.Location{StartRow: 5}, Evidence: "x = 1"},
						{Kind: "mutates_input", Name: "Dup", Location: semantics.Location{StartRow: 8}, Evidence: "x = 1"},
						{Kind: "mutates_input", Name: "Alpha", Location: semantics.Location{StartRow: 12}, Evidence: "y = 2"},
					},
				},
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 20}},
			},
			{
				Path:   "b.go",
				Status: "added",
				Head: &semantics.Result{
					Path:        "b.go",
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Beta", Location: semantics.Location{StartRow: 2}, Evidence: "z = 3"},
						{Kind: "mutates_input", Name: "Gamma", Location: semantics.Location{StartRow: 4}, Evidence: "w = 4"},
					},
				},
			},
			{
				Path:   "c.go",
				Status: "removed",
				Base: &semantics.Result{
					Path:        "c.go",
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Delta", Location: semantics.Location{StartRow: 6}, Evidence: "v = 5"},
					},
				},
			},
		},
	}
}
