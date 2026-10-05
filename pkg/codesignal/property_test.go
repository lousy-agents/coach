package codesignal

import (
	"context"
	"encoding/json"
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

func reverseFileChanges(files []FileChange) []FileChange {
	out := make([]FileChange, len(files))
	for i, fc := range files {
		reordered := fc
		if fc.Base != nil {
			baseCopy := *fc.Base
			baseCopy.Findings = reverseFindings(fc.Base.Findings)
			reordered.Base = &baseCopy
		}
		if fc.Head != nil {
			headCopy := *fc.Head
			headCopy.Findings = reverseFindings(fc.Head.Findings)
			reordered.Head = &headCopy
		}
		out[len(files)-1-i] = reordered
	}
	return out
}

func reverseFindings(findings []semantics.Finding) []semantics.Finding {
	if findings == nil {
		return nil
	}
	out := make([]semantics.Finding, len(findings))
	for i, f := range findings {
		out[len(findings)-1-i] = f
	}
	return out
}
