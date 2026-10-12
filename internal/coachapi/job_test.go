package coachapi

import (
	"encoding/json"
	"testing"
)

// Task 1 / Story 1: Job creator fields and attempt-scoped finding rows.
func TestJobAndFindingCreatorAndAttemptFields(t *testing.T) {
	job := Job{
		ID:                "11111111-1111-1111-1111-111111111111",
		Kind:              JobKindRepoBaselineScan,
		Status:            JobStatusQueued,
		Attempt:           0,
		CreatedByProvider: "github",
		CreatedBySubject:  "12345",
		CreatedByLogin:    "octocat",
	}
	raw, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal Job: %v", err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal Job map: %v", err)
	}
	for _, key := range []string{
		"created_by_provider", "created_by_subject", "created_by_login", "attempt", "status", "kind",
	} {
		if _, ok := asMap[key]; !ok {
			t.Errorf("Job JSON missing %q", key)
		}
	}

	finding := JobFinding{
		JobID:       job.ID,
		Attempt:     2,
		Source:      FindingSourceDeterministic,
		PayloadHash: "hash1",
	}
	fraw, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("marshal JobFinding: %v", err)
	}
	var fmap map[string]json.RawMessage
	if err := json.Unmarshal(fraw, &fmap); err != nil {
		t.Fatalf("unmarshal JobFinding map: %v", err)
	}
	for _, key := range []string{"job_id", "attempt", "source", "payload_hash", "rubric_id", "rubric_version", "model_identity"} {
		if _, ok := fmap[key]; !ok {
			t.Errorf("JobFinding JSON missing %q", key)
		}
	}
}
