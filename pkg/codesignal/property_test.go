package codesignal

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// reorderingScenarioInput builds an Input with 3+ FileChange entries (mixed
// added/modified/removed), multiple Findings per Result, and a duplicate
// (same key: RuleID/path/subject/evidence) pair within one file, so
// occurrence-ordinal grouping is exercised alongside plain sorting.
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
						// Two occurrences sharing the same key ("Dup", "x = 1") --
						// exercises occurrence-ordinal grouping.
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

// reverseFileChanges returns a copy of files with the outer slice reversed
// and each FileChange's Base/Head Findings slice internally reversed too --
// files/Findings are deep-enough-copied that mutating the copy's Findings
// order can't alias the original.
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

// TestProperty_ReorderingInputDoesNotChangeReportJSON proves grouping,
// occurrence-ordinal assignment, fingerprinting, and sorting don't depend on
// input insertion order: reversing Input.Files and each Result.Findings
// slice must produce byte-identical Report JSON.
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

// TestProperty_RangeOverlapNeverPanics feeds LineRange/Location values at
// extremes through Build via FileChange.ChangedRanges combined with a real
// signal-producing Head, and asserts Build completes without panicking. A
// panic fails the test naturally via the Go test runner, so no explicit
// recover() is needed.
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
			{StartRow: 5, EndRow: 3}, // invalid, must be dropped without panicking
			{StartRow: 1000000, EndRow: 2000000},
		}, semantics.Location{StartRow: 1500000, EndRow: 1500000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		})
	}
}

// TestProperty_ArbitraryEvidenceProducesValidJSON runs Finding.Evidence
// values containing embedded double quotes, backslashes, unicode (including
// multi-byte and an emoji), control characters, and a very long string
// through Build, then through json.Marshal and json.Unmarshal into a generic
// map[string]any, asserting both round-trip without error.
func TestProperty_ArbitraryEvidenceProducesValidJSON(t *testing.T) {
	longEvidence := ""
	for i := 0; i < 10000; i++ {
		longEvidence += "x"
	}

	tests := []struct {
		name     string
		evidence string
	}{
		{"embedded double quotes", `cfg.Name = "hello \"world\""`},
		{"backslashes", `path = "C:\\Users\\name"`},
		{"multi-byte unicode", "变量.名前 =値"},
		{"emoji", "cfg.Emoji = \"🎉🚀💥\""},
		{"tab and newline control characters", "x = 1\t// comment\ny = 2"},
		{"embedded null byte", "x\x00 = 1"},
		{"very long string", longEvidence},
		{"mixed adversarial", "x = \"\\n\\t\x00\" + 变量 + \"🎉\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		})
	}
}
