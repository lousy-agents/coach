package coachapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
					`{"judgment":"actionable","rule_id":"state.hidden_input_mutation"}`,
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
	if roundTripped.ReportVersion != ReportVersion1 {
		t.Errorf("Report.report_version: got %q, want %q", roundTripped.ReportVersion, ReportVersion1)
	}
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

// Task 1: initial SQL migration defines jobs / job_findings / job_diagnostics
// with creator fields, attempt scoping, and NULLS NOT DISTINCT uniqueness.
func TestInitMigration_DefinesJobTables(t *testing.T) {
	path := filepath.Join("migrations", "0001_init.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s must not fail: %v", path, err)
	}
	sql := string(body)
	for _, frag := range []string{
		"CREATE TABLE jobs",
		"created_by_provider",
		"created_by_subject",
		"created_by_login",
		"attempt",
		"CREATE TABLE job_findings",
		"payload_hash",
		"NULLS NOT DISTINCT",
		"CREATE TABLE job_diagnostics",
	} {
		if !strings.Contains(sql, frag) {
			t.Errorf("migrations/0001_init.sql must contain %q", frag)
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
