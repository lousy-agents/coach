package coachapi

import (
	"encoding/json"

	"testing"
)

// Task 1 / Story 1: domain constants and Principal shape used by API layers.
func TestDomainConstantsAndPrincipal(t *testing.T) {
	if JobKindRepoBaselineScan != "repo_baseline_scan" {
		t.Errorf("JobKindRepoBaselineScan: got %q", JobKindRepoBaselineScan)
	}
	for _, st := range []JobStatus{
		JobStatusQueued, JobStatusRunning, JobStatusCompleted, JobStatusFailed,
	} {
		if st == "" {
			t.Error("job status constant must be non-empty")
		}
	}
	if JobStatusQueued != "queued" || JobStatusRunning != "running" ||
		JobStatusCompleted != "completed" || JobStatusFailed != "failed" {
		t.Errorf("unexpected job status values: %q %q %q %q",
			JobStatusQueued, JobStatusRunning, JobStatusCompleted, JobStatusFailed)
	}
	if FindingSourceDeterministic != "deterministic" || FindingSourceAgent != "agent" {
		t.Errorf("finding sources: got %q %q", FindingSourceDeterministic, FindingSourceAgent)
	}

	p := Principal{Provider: "github", Subject: "12345", Login: "octocat"}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal Principal: %v", err)
	}
	var asMap map[string]string
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal Principal: %v", err)
	}
	for _, key := range []string{"provider", "subject", "login"} {
		if _, ok := asMap[key]; !ok {
			t.Errorf("Principal JSON missing %q; got %v", key, asMap)
		}
	}
}

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
