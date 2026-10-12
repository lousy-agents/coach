package coachapi

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Task 1 / Story 1+5: marshaling a hand-authored Report must match the
// checked-in golden file byte-for-byte, locking frozen snake_case field names
// including nullable top-level error and finding provenance fields.
func TestReport_MarshalMatchesGoldenFile(t *testing.T) {
	got, err := json.MarshalIndent(goldenReport(), "", "  ")
	if err != nil {
		t.Fatalf("marshaling the golden Report must not fail: %v", err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile("testdata/report_golden.json")
	if err != nil {
		t.Fatalf("reading testdata/report_golden.json must not fail: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Report JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", got, want)
	}

	var roundTripped Report
	if err := json.Unmarshal(want, &roundTripped); err != nil {
		t.Fatalf("golden file must unmarshal back into a Report: %v", err)
	}
	expectGoldenReportHeader(t, roundTripped)
	expectGoldenReportFindings(t, roundTripped)
	if _, ok := roundTripped.Summary.FindingCounts[string(FindingSourceDeterministic)]["state.hidden_input_mutation"]; !ok {
		t.Errorf("summary.finding_counts.deterministic missing rule id; got %#v", roundTripped.Summary.FindingCounts)
	}
}

// expectGoldenReportHeader checks the golden Report's version, kind, and
// null top-level error after a round trip.
func expectGoldenReportHeader(t *testing.T, roundTripped Report) {
	t.Helper()
	if roundTripped.ReportVersion != ReportVersion1 {
		t.Errorf("Report.report_version: got %q, want %q", roundTripped.ReportVersion, ReportVersion1)
	}
	if roundTripped.Kind != JobKindRepoBaselineScan {
		t.Errorf("Report.kind: got %q, want %q", roundTripped.Kind, JobKindRepoBaselineScan)
	}
	if roundTripped.Error != nil {
		t.Errorf("Report.error: got %v, want null", *roundTripped.Error)
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
