package codesignal

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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
			body_propertyPart2Test_33(t, tt)
		})
	}
}

func body_propertyPart2Test_33(t *testing.T, tt struct {
	name     string
	evidence string
}) {
	head := &semantics.Result{
		Path:        "evidence.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Adversarial", Evidence: tt.evidence},
		},
	}

	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{Path: "evidence.go", Status: "modified", Head: head},
		},
	})

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report) with Evidence %q must not fail: %v", tt.evidence, err)
	}

	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("json.Unmarshal back into map[string]any with Evidence %q must not fail: %v\nJSON: %s", tt.evidence, err, raw)
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

func body_propertyTest_69(t *testing.T, tt struct {
	name   string
	ranges []LineRange
	loc    semantics.Location
}) {
	head := &semantics.Result{
		Path:        "extreme.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Extreme", Location: tt.loc, Evidence: "x = 1"},
		},
	}

	report := mustBuild(t, Options{IncludeResolved: true}, Input{
		Files: []FileChange{
			{Path: "extreme.go", Status: "modified", Head: head, ChangedRanges: tt.ranges},
		},
	})
	if report == nil {
		t.Fatalf("Build returned nil Report")
	}
}
