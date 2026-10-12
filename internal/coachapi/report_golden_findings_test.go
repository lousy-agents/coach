package coachapi

import (
	"testing"
)

// expectGoldenReportFindings checks both finding provenance shapes in the
// golden Report: deterministic with null rubric/model fields, agent with all
// three set.
func expectGoldenReportFindings(t *testing.T, roundTripped Report) {
	t.Helper()
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
}
