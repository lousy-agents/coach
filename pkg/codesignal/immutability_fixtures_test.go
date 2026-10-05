package codesignal

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// snapshotFileChanges copies each FileChange and its ChangedRanges so a
// later comparison sees in-place edits to the caller's slice elements.
func snapshotFileChanges(files []FileChange) []FileChange {
	snapshot := make([]FileChange, len(files))
	for i, fc := range files {
		cp := fc
		cp.ChangedRanges = append([]LineRange(nil), fc.ChangedRanges...)
		snapshot[i] = cp
	}
	return snapshot
}

func expectFileChangesUnchanged(t *testing.T, files, want []FileChange) {
	t.Helper()
	if len(files) != len(want) {
		t.Fatalf("Input.Files length changed: got %d, want %d", len(files), len(want))
	}
	for i, fc := range files {
		if fc.Path != want[i].Path || fc.Status != want[i].Status {
			t.Errorf("Files[%d] Path/Status mutated: got %+v, want %+v", i, fc, want[i])
		}
		if !reflect.DeepEqual(fc.ChangedRanges, want[i].ChangedRanges) {
			t.Errorf("Files[%d].ChangedRanges mutated: got %+v, want %+v", i, fc.ChangedRanges, want[i].ChangedRanges)
		}
	}
}

func richImmutabilityInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "rev1", Base: "main"},
		Diagnostics: []Diagnostic{
			{Path: "z.go", Kind: "custom", Message: "hello"},
		},
		Files: []FileChange{
			{
				Path:   "a.go",
				Status: "modified",
				Base: &semantics.Result{
					Path:        "a.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Imports: []semantics.ImportFeature{
						{Path: "fmt", Location: semantics.Location{StartRow: 1}},
					},
					Metrics: semantics.StructuralMetrics{Ifs: 3, Functions: 2},
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}, Evidence: "x = 1"},
						{Kind: "mutates_input", Name: "GoneNow", Location: semantics.Location{StartRow: 2}, Evidence: "y = 2"},
					},
				},
				Head: &semantics.Result{
					Path:        "a.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Imports: []semantics.ImportFeature{
						{Path: "fmt", Location: semantics.Location{StartRow: 1}},
						{Path: "os", Location: semantics.Location{StartRow: 2}},
					},
					Metrics: semantics.StructuralMetrics{Ifs: 4, Functions: 3},
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 10}, Evidence: "x = 1"},
						{Kind: "mutates_input", Name: "NewOne", Location: semantics.Location{StartRow: 20}, Evidence: "z = 3"},
					},
				},
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 30}},
			},
			{
				Path:   "broken.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "broken.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("syntax_errors"),
					SyntaxErrors: []semantics.SyntaxIssue{
						{Kind: "error", Location: semantics.Location{StartRow: 1, StartCol: 2, EndRow: 1, EndCol: 5}},
					},
				},
			},
			{
				Path:   "removed.go",
				Status: "removed",
				Base: &semantics.Result{
					Path:        "removed.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Deleted", Location: semantics.Location{StartRow: 5}, Evidence: "w = 4"},
					},
				},
			},
		},
	}
}
