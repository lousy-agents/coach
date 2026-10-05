package coachapi

import (
	"encoding/json"
	"os"

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

// Task 1 / Story 1: API error envelope is frozen snake_case
// {"error":{"code":"...","message":"..."}}.
func TestErrorEnvelope_MarshalMatchesGoldenFile(t *testing.T) {
	env := ErrorEnvelope{
		Error: APIError{
			Code:    ErrorCodeJobNotFound,
			Message: "job not found",
		},
	}
	got, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("marshaling ErrorEnvelope must not fail: %v", err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile("testdata/error_envelope_golden.json")
	if err != nil {
		t.Fatalf("reading testdata/error_envelope_golden.json must not fail: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("ErrorEnvelope JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", got, want)
	}
}
