package coachapi

import (
	"encoding/json"
	"os"

	"testing"
)

// Task 1 / Story 1+5: marshaling a hand-authored Report must match the
// checked-in golden file byte-for-byte, locking frozen snake_case field names
// including nullable top-level error and finding provenance fields.
func TestReport_MarshalMatchesGoldenFile(t *testing.T) {
	got, err := json.MarshalIndent(goldenReport(), "", "  ")
	(&sigTestReportMarshalMatchesGoldenFileS158059557{err: err, t: t}).call()

	got = append(got, '\n')

	want, err := os.ReadFile("testdata/report_golden.json")
	(&sigTestReportMarshalMatchesGoldenFileS455697778{err: err, t: t}).call()
	(&sigTestReportMarshalMatchesGoldenFileS561530703{got: got, t: t, want: want}).call()

	var roundTripped Report
	(&sigTestReportMarshalMatchesGoldenFileS761534661{roundTripped: &roundTripped, t: t, want: want}).call()
	(&sigTestReportMarshalMatchesGoldenFileS855176925{roundTripped: roundTripped, t: t}).call()

	if roundTripped.Kind != JobKindRepoBaselineScan {
		t.Errorf("Report.kind: got %q, want %q", roundTripped.Kind, JobKindRepoBaselineScan)
	}
	if roundTripped.Error != nil {
		t.Errorf("Report.error: got %v, want null", *roundTripped.Error)
	}
	if len(roundTripped.Findings) != 2 {
		t.Fatalf("Report.findings length: got %d, want 2", len(roundTripped.Findings))
	}
	det := roundTripped.Findings[0]
	if det.Source != FindingSourceDeterministic {
		t.Errorf("findings[0].source: got %q, want %q", det.Source, FindingSourceDeterministic)
	}
	if det.RubricID != nil || det.RubricVersion != nil || det.ModelIdentity != nil {
		t.Errorf("deterministic finding must have null rubric_id/rubric_version/model_identity; got %#v %#v %#v",
			det.RubricID, det.RubricVersion, det.ModelIdentity)
	}
	agent := roundTripped.Findings[1]
	if agent.Source != FindingSourceAgent {
		t.Errorf("findings[1].source: got %q, want %q", agent.Source, FindingSourceAgent)
	}
	if agent.RubricID == nil || *agent.RubricID != "hidden_mutation_contextualization" {
		t.Errorf("findings[1].rubric_id: got %v, want hidden_mutation_contextualization", agent.RubricID)
	}
	if agent.RubricVersion == nil || *agent.RubricVersion != "1" {
		t.Errorf("findings[1].rubric_version: got %v, want 1", agent.RubricVersion)
	}
	if agent.ModelIdentity == nil || *agent.ModelIdentity != "stub-model@v1" {
		t.Errorf("findings[1].model_identity: got %v, want stub-model@v1", agent.ModelIdentity)
	}
	if _, ok := roundTripped.Summary.FindingCounts[string(FindingSourceDeterministic)]["state.hidden_input_mutation"]; !ok {
		t.Errorf("summary.finding_counts.deterministic missing rule id; got %#v", roundTripped.Summary.FindingCounts)
	}
}
