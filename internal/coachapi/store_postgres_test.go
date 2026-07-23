package coachapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// jsonEqual compares two JSON documents by decoded value, not raw bytes:
// jsonb columns canonicalize key order and whitespace on round-trip, so byte
// comparison against the originally-inserted literal would fail for
// reasons unrelated to correctness.
func jsonEqual(t *testing.T, got, want json.RawMessage) bool {
	t.Helper()
	var gotVal, wantVal any
	if err := json.Unmarshal(got, &gotVal); err != nil {
		t.Fatalf("unmarshal got JSON %s: %v", got, err)
	}
	if err := json.Unmarshal(want, &wantVal); err != nil {
		t.Fatalf("unmarshal want JSON %s: %v", want, err)
	}
	return reflect.DeepEqual(gotVal, wantVal)
}

// migrationFiles returns every internal/coachapi/migrations/*.sql path, in
// filename order (0001_..., 0002_..., ...), read from disk rather than
// hand-duplicated so this test cannot drift from the real migrations.
func migrationFiles(t *testing.T) []string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "migrations")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading migrations dir %s: %v", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files
}

// setupPostgresStore resets dsn's public schema to empty and reapplies every
// migration, so the test is repeatable against a persistent dev Postgres
// rather than requiring a throwaway database per run.
func setupPostgresStore(t *testing.T, dsn string) *coachapi.PostgresStore {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("could not connect to COACH_PG_DSN Postgres instance: %v", err)
	}

	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("resetting public schema: %v", err)
	}

	for _, path := range migrationFiles(t) {
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading migration %s: %v", path, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("applying migration %s: %v", path, err)
		}
	}

	return coachapi.NewPostgresStore(pool)
}

func pgQueuedJob(id string) coachapi.Job {
	return coachapi.Job{
		ID:                id,
		Kind:              coachapi.JobKindRepoBaselineScan,
		Params:            json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets","ref":"main","extra":{"nested":[1,2,3],"flag":true}}`),
		Status:            coachapi.JobStatusQueued,
		CreatedAt:         time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		Attempt:           0,
		CreatedByProvider: "github",
		CreatedBySubject:  "12345",
		CreatedByLogin:    "octocat",
	}
}

// TestPostgresStoreAcceptance exercises coachapi.PostgresStore (Task 2,
// GitHub issue #103) against a real Postgres 16+ instance, gated on
// COACH_PG_DSN per this repo's real-backend integration test convention
// (see internal/coachapi/queue/redisstream/redisstream_conformance_test.go):
// skip cleanly when the env var is unset rather than failing or hanging.
// This is required because 0001_init.sql's job_findings UNIQUE NULLS NOT
// DISTINCT constraint is a Postgres 16 feature no in-memory/sqlite double
// can exercise.
func TestPostgresStoreAcceptance(t *testing.T) {
	dsn := os.Getenv("COACH_PG_DSN")
	if dsn == "" {
		t.Skip("COACH_PG_DSN not set; skipping Postgres integration test")
	}

	store := setupPostgresStore(t, dsn)
	ctx := context.Background()

	t.Run("CreateJob round-trips every field through GetJob, including a non-trivial Params blob", func(t *testing.T) {
		job := pgQueuedJob("11111111-1111-1111-1111-111111111111")

		if err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		got, err := store.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob: %v", err)
		}
		if got.ID != job.ID {
			t.Errorf("ID = %q, want %q", got.ID, job.ID)
		}
		if got.Kind != job.Kind {
			t.Errorf("Kind = %q, want %q", got.Kind, job.Kind)
		}
		if !jsonEqual(t, got.Params, job.Params) {
			t.Errorf("Params = %s, want %s", got.Params, job.Params)
		}
		if got.Status != coachapi.JobStatusQueued {
			t.Errorf("Status = %q, want queued", got.Status)
		}
		if !got.CreatedAt.Equal(job.CreatedAt) {
			t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, job.CreatedAt)
		}
		if got.Attempt != 0 {
			t.Errorf("Attempt = %d, want 0", got.Attempt)
		}
		if got.CreatedByProvider != "github" || got.CreatedBySubject != "12345" || got.CreatedByLogin != "octocat" {
			t.Errorf("created_by fields = %+v", got)
		}
		if got.Error != nil {
			t.Errorf("Error = %v, want nil", *got.Error)
		}
		if got.StartedAt != nil {
			t.Errorf("StartedAt = %v, want nil", *got.StartedAt)
		}
		if got.FinishedAt != nil {
			t.Errorf("FinishedAt = %v, want nil", *got.FinishedAt)
		}
	})

	t.Run("rejects a second CreateJob for the same id without reporting ErrJobNotFound", func(t *testing.T) {
		job := pgQueuedJob("22222222-2222-2222-2222-222222222222")
		if err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("first CreateJob: %v", err)
		}

		err := store.CreateJob(ctx, job)
		if err == nil {
			t.Fatal("second CreateJob: want error, got nil")
		}
		if errors.Is(err, coachapi.ErrJobNotFound) {
			t.Errorf("second CreateJob error wraps ErrJobNotFound, want a plain store failure: %v", err)
		}
	})

	t.Run("GetJob/RecordCompletion/RecordFailure on an unknown id all return ErrJobNotFound", func(t *testing.T) {
		const missing = "33333333-3333-3333-3333-333333333333"

		if _, err := store.GetJob(ctx, missing); !errors.Is(err, coachapi.ErrJobNotFound) {
			t.Errorf("GetJob err = %v, want ErrJobNotFound", err)
		}
		if err := store.RecordCompletion(ctx, missing, coachapi.Completion{}); !errors.Is(err, coachapi.ErrJobNotFound) {
			t.Errorf("RecordCompletion err = %v, want ErrJobNotFound", err)
		}
		if err := store.RecordFailure(ctx, missing, "boom", time.Now()); !errors.Is(err, coachapi.ErrJobNotFound) {
			t.Errorf("RecordFailure err = %v, want ErrJobNotFound", err)
		}
	})

	t.Run("GetReport on a job that was never completed returns ErrJobNotFound", func(t *testing.T) {
		job := pgQueuedJob("44444444-4444-4444-4444-444444444444")
		if err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		if _, err := store.GetReport(ctx, job.ID); !errors.Is(err, coachapi.ErrJobNotFound) {
			t.Errorf("GetReport err = %v, want ErrJobNotFound", err)
		}
	})

	t.Run("RecordFailure marks the job failed with the recorded error message and finish time", func(t *testing.T) {
		job := pgQueuedJob("55555555-5555-5555-5555-555555555555")
		if err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		finishedAt := time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC)
		if err := store.RecordFailure(ctx, job.ID, "clone failed: timeout", finishedAt); err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}

		got, err := store.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob: %v", err)
		}
		if got.Status != coachapi.JobStatusFailed {
			t.Errorf("Status = %q, want failed", got.Status)
		}
		if got.Error == nil || *got.Error != "clone failed: timeout" {
			t.Errorf("Error = %v, want \"clone failed: timeout\"", got.Error)
		}
		if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
			t.Errorf("FinishedAt = %v, want %v", got.FinishedAt, finishedAt)
		}
	})

	t.Run("RecordCompletion marks the job completed and GetReport assembles the same report shape MemoryStore produces", func(t *testing.T) {
		job := pgQueuedJob("66666666-6666-6666-6666-666666666666")
		if err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		agentRubricID := "hidden_mutation_contextualization"
		agentRubricVersion := "1"
		agentModelIdentity := "stub-model@v1"
		generatedAt := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
		finishedAt := time.Date(2026, 1, 15, 11, 59, 0, 0, time.UTC)

		completion := coachapi.Completion{
			Attempt:   1,
			CommitSHA: "abc123def4567890abc123def4567890abc123de",
			Findings: []coachapi.JobFinding{
				{
					ID:          "66666666-0000-0000-0000-000000000001",
					JobID:       job.ID,
					Attempt:     1,
					Source:      coachapi.FindingSourceDeterministic,
					Payload:     json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"pkg/example/service.go"}`),
					PayloadHash: "hash-det-1",
				},
				{
					ID:            "66666666-0000-0000-0000-000000000002",
					JobID:         job.ID,
					Attempt:       1,
					Source:        coachapi.FindingSourceAgent,
					RubricID:      &agentRubricID,
					RubricVersion: &agentRubricVersion,
					ModelIdentity: &agentModelIdentity,
					Payload:       json.RawMessage(`{"judgment":"actionable","rule_id":"state.hidden_input_mutation"}`),
					PayloadHash:   "hash-agent-1",
				},
			},
			Diagnostics: []coachapi.JobDiagnostic{
				{
					ID:      "66666666-0000-0000-0000-000000000003",
					JobID:   job.ID,
					Attempt: 1,
					Scope:   "file:pkg/example/legacy.py",
					Message: "unsupported language",
				},
			},
			Versions: coachapi.ReportVersions{
				Analyzer: "codesignal@1",
				Rubrics: map[string]string{
					"change_cohesion":                   "1",
					"hidden_mutation_contextualization": "1",
				},
			},
			FinishedAt:  finishedAt,
			GeneratedAt: generatedAt,
		}

		if err := store.RecordCompletion(ctx, job.ID, completion); err != nil {
			t.Fatalf("RecordCompletion: %v", err)
		}

		gotJob, err := store.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob: %v", err)
		}
		if gotJob.Status != coachapi.JobStatusCompleted {
			t.Errorf("Status = %q, want completed", gotJob.Status)
		}
		if gotJob.Attempt != 1 {
			t.Errorf("Attempt = %d, want 1", gotJob.Attempt)
		}
		if gotJob.FinishedAt == nil || !gotJob.FinishedAt.Equal(finishedAt) {
			t.Errorf("FinishedAt = %v, want %v", gotJob.FinishedAt, finishedAt)
		}
		if gotJob.Error != nil {
			t.Errorf("Error = %v, want nil", *gotJob.Error)
		}

		report, err := store.GetReport(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetReport: %v", err)
		}

		wantReport := coachapi.Report{
			ReportVersion: coachapi.ReportVersion1,
			JobID:         job.ID,
			Kind:          coachapi.JobKindRepoBaselineScan,
			Params:        job.Params,
			CommitSHA:     completion.CommitSHA,
			Summary: coachapi.ReportSummary{
				FindingCounts: map[string]map[string]int{
					"deterministic": {"state.hidden_input_mutation": 1},
					"agent":         {"hidden_mutation_contextualization": 1},
				},
			},
			Findings: []coachapi.Finding{
				{
					Source:  coachapi.FindingSourceDeterministic,
					Payload: json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"pkg/example/service.go"}`),
				},
				{
					Source:        coachapi.FindingSourceAgent,
					RubricID:      &agentRubricID,
					RubricVersion: &agentRubricVersion,
					ModelIdentity: &agentModelIdentity,
					Payload:       json.RawMessage(`{"judgment":"actionable","rule_id":"state.hidden_input_mutation"}`),
				},
			},
			Diagnostics: []coachapi.Diagnostic{
				{Scope: "file:pkg/example/legacy.py", Message: "unsupported language"},
			},
			Error:       nil,
			Versions:    completion.Versions,
			GeneratedAt: generatedAt,
		}

		if report.ReportVersion != wantReport.ReportVersion {
			t.Errorf("ReportVersion = %q, want %q", report.ReportVersion, wantReport.ReportVersion)
		}
		if report.JobID != wantReport.JobID {
			t.Errorf("JobID = %q, want %q", report.JobID, wantReport.JobID)
		}
		if report.Kind != wantReport.Kind {
			t.Errorf("Kind = %q, want %q", report.Kind, wantReport.Kind)
		}
		if !jsonEqual(t, report.Params, wantReport.Params) {
			t.Errorf("Params = %s, want %s", report.Params, wantReport.Params)
		}
		if report.CommitSHA != wantReport.CommitSHA {
			t.Errorf("CommitSHA = %q, want %q", report.CommitSHA, wantReport.CommitSHA)
		}
		gotSummary, err := json.Marshal(report.Summary)
		if err != nil {
			t.Fatalf("marshal got summary: %v", err)
		}
		wantSummary, err := json.Marshal(wantReport.Summary)
		if err != nil {
			t.Fatalf("marshal want summary: %v", err)
		}
		if string(gotSummary) != string(wantSummary) {
			t.Errorf("Summary = %s, want %s", gotSummary, wantSummary)
		}
		if len(report.Findings) != len(wantReport.Findings) {
			t.Fatalf("len(Findings) = %d, want %d", len(report.Findings), len(wantReport.Findings))
		}
		for i, want := range wantReport.Findings {
			got := report.Findings[i]
			if got.Source != want.Source {
				t.Errorf("Findings[%d].Source = %q, want %q", i, got.Source, want.Source)
			}
			if !equalStringPtr(got.RubricID, want.RubricID) {
				t.Errorf("Findings[%d].RubricID = %v, want %v", i, got.RubricID, want.RubricID)
			}
			if !equalStringPtr(got.RubricVersion, want.RubricVersion) {
				t.Errorf("Findings[%d].RubricVersion = %v, want %v", i, got.RubricVersion, want.RubricVersion)
			}
			if !equalStringPtr(got.ModelIdentity, want.ModelIdentity) {
				t.Errorf("Findings[%d].ModelIdentity = %v, want %v", i, got.ModelIdentity, want.ModelIdentity)
			}
			if !jsonEqual(t, got.Payload, want.Payload) {
				t.Errorf("Findings[%d].Payload = %s, want %s", i, got.Payload, want.Payload)
			}
		}
		if len(report.Diagnostics) != len(wantReport.Diagnostics) {
			t.Fatalf("len(Diagnostics) = %d, want %d", len(report.Diagnostics), len(wantReport.Diagnostics))
		}
		for i, want := range wantReport.Diagnostics {
			got := report.Diagnostics[i]
			if got != want {
				t.Errorf("Diagnostics[%d] = %+v, want %+v", i, got, want)
			}
		}
		if report.Error != nil {
			t.Errorf("Error = %v, want nil", *report.Error)
		}
		gotVersions, err := json.Marshal(report.Versions)
		if err != nil {
			t.Fatalf("marshal got versions: %v", err)
		}
		wantVersions, err := json.Marshal(wantReport.Versions)
		if err != nil {
			t.Fatalf("marshal want versions: %v", err)
		}
		if string(gotVersions) != string(wantVersions) {
			t.Errorf("Versions = %s, want %s", gotVersions, wantVersions)
		}
		if !report.GeneratedAt.Equal(wantReport.GeneratedAt) {
			t.Errorf("GeneratedAt = %v, want %v", report.GeneratedAt, wantReport.GeneratedAt)
		}
	})

	t.Run("job_findings UNIQUE NULLS NOT DISTINCT rejects a duplicate deterministic finding within (job_id, attempt, source, payload_hash)", func(t *testing.T) {
		job := pgQueuedJob("77777777-7777-7777-7777-777777777777")
		if err := store.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		dupPayload := json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"pkg/example/service.go"}`)
		completion := coachapi.Completion{
			Attempt:   1,
			CommitSHA: "abc123def4567890abc123def4567890abc123de",
			Findings: []coachapi.JobFinding{
				{
					ID:          "77777777-0000-0000-0000-000000000001",
					JobID:       job.ID,
					Attempt:     1,
					Source:      coachapi.FindingSourceDeterministic,
					Payload:     dupPayload,
					PayloadHash: "hash-dup",
				},
				{
					// Distinct row id, but same (job_id, attempt, source,
					// rubric_id=NULL, payload_hash) as the row above: with a
					// default UNIQUE constraint NULL rubric_id would make
					// these "distinct", silently permitting the duplicate.
					// 0001_init.sql's UNIQUE NULLS NOT DISTINCT must reject
					// this insert instead.
					ID:          "77777777-0000-0000-0000-000000000002",
					JobID:       job.ID,
					Attempt:     1,
					Source:      coachapi.FindingSourceDeterministic,
					Payload:     dupPayload,
					PayloadHash: "hash-dup",
				},
			},
			Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
			FinishedAt:  time.Date(2026, 1, 15, 11, 59, 0, 0, time.UTC),
			GeneratedAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		}

		err := store.RecordCompletion(ctx, job.ID, completion)
		if err == nil {
			t.Fatal("RecordCompletion with a duplicate deterministic finding: want error, got nil")
		}

		// The whole attempt is one transaction: the constraint violation
		// must roll back the jobs row update too, not leave the job
		// half-completed.
		gotJob, getErr := store.GetJob(ctx, job.ID)
		if getErr != nil {
			t.Fatalf("GetJob after failed RecordCompletion: %v", getErr)
		}
		if gotJob.Status != coachapi.JobStatusQueued {
			t.Errorf("Status after rolled-back RecordCompletion = %q, want queued (unchanged)", gotJob.Status)
		}
		if gotJob.Attempt != 0 {
			t.Errorf("Attempt after rolled-back RecordCompletion = %d, want 0 (unchanged)", gotJob.Attempt)
		}
	})
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
