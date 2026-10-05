package codesignalcli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestRenderJSONDoesNotAddFields(t *testing.T) {
	report := &codesignal.Report{
		SchemaVersion: "1",
		Summary:       codesignal.Summary{FilesAnalyzed: 1},
	}

	encoded, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("RenderJSON: %s", err)
	}

	var directMarshal map[string]any
	direct, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal: %s", err)
	}
	if err := json.Unmarshal(direct, &directMarshal); err != nil {
		t.Fatalf("json.Unmarshal direct: %s", err)
	}

	var rendered map[string]any
	if err := json.Unmarshal(encoded, &rendered); err != nil {
		t.Fatalf("json.Unmarshal rendered: %s", err)
	}

	if len(rendered) != len(directMarshal) {
		t.Errorf("RenderJSON produced a different field set than json.Marshal: rendered=%v direct=%v", rendered, directMarshal)
	}
	for k := range directMarshal {
		if _, ok := rendered[k]; !ok {
			t.Errorf("RenderJSON is missing field %q present in plain json.Marshal", k)
		}
	}
}

func TestRenderTextSignalLabels(t *testing.T) {
	report := &codesignal.Report{
		Summary: codesignal.Summary{FilesAnalyzed: 3, ActiveSignals: 1},
		Signals: []codesignal.Signal{
			{
				Path:           "a.go",
				SourceScope:    "production",
				Location:       semantics.Location{StartRow: 4},
				Lifecycle:      codesignal.Lifecycle("introduced"),
				Changed:        true,
				Evidence:       "func Update mutates input",
				WhyItMatters:   "callers may not expect their argument to be mutated",
				Recommendation: "return a new value instead of mutating input",
			},
		},
	}

	got := RenderText(report)

	for _, want := range []string{
		"path: a.go",
		"line: 5",
		"lifecycle: introduced",
		"source_scope: production",
		"changed: true",
		"evidence: func Update mutates input",
		"why it matters: callers may not expect their argument to be mutated",
		"recommendation: return a new value instead of mutating input",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing %q; got:\n%s", want, got)
		}
	}
}

func TestRenderTextLineIsOneBasedFromStartRow(t *testing.T) {
	tests := []struct {
		name     string
		startRow uint
		wantLine string
	}{
		{name: "reports line 1 for a zero start row", startRow: 0, wantLine: "line: 1"},
		{name: "reports line 11 for a start row of ten", startRow: 10, wantLine: "line: 11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_renderPart2Test_94(t, tt)
		})
	}
}
