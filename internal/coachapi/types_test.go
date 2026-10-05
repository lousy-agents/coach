package coachapi

import (
	"encoding/json"

	"testing"
	"time"
)

// Task 1 / Story 1: top-level error must serialize as JSON null when unset,
// not be omitted, so clients can rely on the key always being present.
// Empty findings/diagnostics must serialize as JSON arrays (not null), matching
// the frozen report contract (spec: findings/diagnostics are arrays).
func TestReport_ErrorSerializesAsNullWhenUnset(t *testing.T) {
	raw, err := json.Marshal(Report{
		ReportVersion: ReportVersion1,
		JobID:         "id",
		Kind:          JobKindRepoBaselineScan,
		Params:        json.RawMessage(`{"repo_owner":"o","repo_name":"n"}`),
		CommitSHA:     "sha",
		Summary:       ReportSummary{},
		Versions:      ReportVersions{Analyzer: "a"},
		GeneratedAt:   time.Unix(0, 0).UTC(),
		Error:         nil,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal map: %v", err)
	}
	errRaw, ok := asMap["error"]
	if !ok {
		t.Fatal(`Report JSON must include top-level "error" key even when unset`)
	}
	if string(errRaw) != "null" {
		t.Errorf(`Report.error must be JSON null when unset; got %s`, errRaw)
	}
	for _, key := range []string{"findings", "diagnostics"} {
		v, ok := asMap[key]
		if !ok {
			t.Errorf(`Report JSON must include %q key`, key)
			continue
		}
		if string(v) != "[]" {
			t.Errorf(`Report.%s must be JSON [] when empty/nil; got %s`, key, v)
		}
	}
	sumRaw, ok := asMap["summary"]
	if !ok {
		t.Fatal(`Report JSON must include "summary"`)
	}
	var sum map[string]json.RawMessage
	if err := json.Unmarshal(sumRaw, &sum); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	fc, ok := sum["finding_counts"]
	if !ok {
		t.Fatal(`summary must include finding_counts`)
	}
	if string(fc) != "{}" {
		t.Errorf(`summary.finding_counts must be JSON {} when empty/nil; got %s`, fc)
	}
}

// goldenReport builds the hand-authored Report fixture used by the Task 1
// golden-file lock. It must exercise nullable top-level error, both finding
// provenance sources (deterministic with null rubric/model fields; agent with
// rubric_id/rubric_version/model_identity), summary.finding_counts keyed by
// source then rule/rubric id, diagnostics, and versions.
func goldenReport() Report {
	agentRubricID := "hidden_mutation_contextualization"
	agentRubricVersion := "1"
	agentModelIdentity := "stub-model@v1"

	return Report{
		ReportVersion: ReportVersion1,
		JobID:         "11111111-1111-1111-1111-111111111111",
		Kind:          JobKindRepoBaselineScan,
		Params: json.RawMessage(
			`{"repo_owner":"acme","repo_name":"widgets","ref":"main"}`,
		),
		CommitSHA: "abc123def4567890abc123def4567890abc123de",
		Summary: ReportSummary{
			FindingCounts: map[string]map[string]int{
				string(FindingSourceDeterministic): {
					"state.hidden_input_mutation": 1,
				},
				string(FindingSourceAgent): {
					"hidden_mutation_contextualization": 1,
				},
			},
		},
		Findings: []Finding{
			{
				Source: FindingSourceDeterministic,
				Payload: json.RawMessage(
					`{"rule_id":"state.hidden_input_mutation","path":"pkg/example/service.go"}`,
				),
			},
			{
				Source:        FindingSourceAgent,
				RubricID:      &agentRubricID,
				RubricVersion: &agentRubricVersion,
				ModelIdentity: &agentModelIdentity,
				Payload: json.RawMessage(
					`{"judgment":"concern","rationale":"mutation hides input state that will surprise reviewers","confidence":"high","suggested_focus":"constructor parameter assignment"}`,
				),
			},
		},
		Diagnostics: []Diagnostic{
			{
				Scope:   "file:pkg/example/legacy.py",
				Message: "unsupported language",
			},
		},
		Error: nil,
		Versions: ReportVersions{
			Analyzer: "codesignal@1",
			Rubrics: map[string]string{
				"change_cohesion":                   "1",
				"hidden_mutation_contextualization": "1",
			},
		},
		GeneratedAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}
}
