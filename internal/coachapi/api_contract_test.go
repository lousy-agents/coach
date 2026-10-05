package coachapi

import (
	"encoding/json"
	"os"
	"testing"
)

// Pilot error codes named in Story 1 must exist as stable constants.
func TestErrorCodes_PilotSet(t *testing.T) {
	codes := []string{
		ErrorCodeUnauthenticated,
		ErrorCodeUnauthorized,
		ErrorCodeInvalidRequest,
		ErrorCodeJobNotFound,
		ErrorCodeJobNotCompleted,
		ErrorCodeUnsupportedJobKind,
		ErrorCodeRepoNotAuthorized,
		ErrorCodeNotFound,
		ErrorCodeInternalError,
	}
	want := []string{
		"unauthenticated",
		"unauthorized",
		"invalid_request",
		"job_not_found",
		"job_not_completed",
		"unsupported_job_kind",
		"repo_not_authorized",
		"not_found",
		"internal_error",
	}
	for i, got := range codes {
		if got != want[i] {
			t.Errorf("error code[%d]: got %q, want %q", i, got, want[i])
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

// GET /v1/jobs/{id} status body: error key is always present (null when unset),
// matching Report.error — not omitempty — so clients share one nullability rule.
func TestJobStatusResponse_ErrorSerializesAsNullWhenUnset(t *testing.T) {
	raw, err := json.Marshal(JobStatusResponse{
		ID:      "id",
		Kind:    JobKindRepoBaselineScan,
		Status:  JobStatusQueued,
		Attempt: 0,
		Error:   nil,
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
		t.Fatal(`JobStatusResponse JSON must include "error" key even when unset`)
	}
	if string(errRaw) != "null" {
		t.Errorf(`JobStatusResponse.error must be JSON null when unset; got %s`, errRaw)
	}
}
