package coachapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
)

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

// pgMigrationFiles returns every internal/coachapi/migrations/*.sql path, in
// filename order (0001_..., 0002_..., ...), read from disk rather than
// hand-duplicated so this test cannot drift from the real migrations.
func pgMigrationFiles() []string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	dir := filepath.Join(filepath.Dir(thisFile), "migrations")

	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred(), "reading migrations dir %s", dir)

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
// migration, so each spec starts from a clean, known schema against a
// persistent dev Postgres rather than requiring a throwaway database per
// run. It calls Skip (not Fail) when dsn is unreachable, matching this
// repo's real-backend integration test convention (see
// internal/coachapi/queue/redisstream/redisstream_conformance_test.go):
// skip cleanly rather than fail or hang when the real backend isn't
// available.
func setupPostgresStore(ctx context.Context, dsn string) *coachapi.PostgresStore {
	pool, err := pgxpool.New(ctx, dsn)
	Expect(err).NotTo(HaveOccurred(), "pgxpool.New")
	DeferCleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		Skip(fmt.Sprintf("could not connect to COACH_PG_DSN Postgres instance: %v", err))
	}

	_, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	Expect(err).NotTo(HaveOccurred(), "resetting public schema")

	for _, path := range pgMigrationFiles() {
		sqlBytes, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred(), "reading migration %s", path)
		_, err = pool.Exec(ctx, string(sqlBytes))
		Expect(err).NotTo(HaveOccurred(), "applying migration %s", path)
	}

	return coachapi.NewPostgresStore(pool)
}

// coachapi.PostgresStore (Task 2, GitHub issue #103) is exercised against a
// real Postgres 16+ instance, gated on COACH_PG_DSN per this repo's
// real-backend integration test convention: skip cleanly when the env var
// is unset rather than failing or hanging. This is required because
// 0001_init.sql's job_findings UNIQUE NULLS NOT DISTINCT constraint is a
// Postgres 16 feature no in-memory/sqlite double can exercise.
var _ = Describe("coachapi.PostgresStore", func() {
	var (
		ctx   context.Context
		store *coachapi.PostgresStore
	)

	BeforeEach(func() {
		dsn := os.Getenv("COACH_PG_DSN")
		if dsn == "" {
			Skip("COACH_PG_DSN not set; skipping Postgres integration test")
		}
		ctx = context.Background()
		store = setupPostgresStore(ctx, dsn)
	})

	When("a job is created", func() {
		It("round-trips every field through GetJob, including a non-trivial Params blob", func() {
			job := pgQueuedJob("11111111-1111-1111-1111-111111111111")

			Expect(store.CreateJob(ctx, job)).To(Succeed())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ID).To(Equal(job.ID))
			Expect(got.Kind).To(Equal(job.Kind))
			Expect(got.Params).To(MatchJSON(job.Params))
			Expect(got.Status).To(Equal(coachapi.JobStatusQueued))
			Expect(got.CreatedAt).To(BeTemporally("==", job.CreatedAt))
			Expect(got.Attempt).To(Equal(0))
			Expect(got.CreatedByProvider).To(Equal("github"))
			Expect(got.CreatedBySubject).To(Equal("12345"))
			Expect(got.CreatedByLogin).To(Equal("octocat"))
			Expect(got.Error).To(BeNil())
			Expect(got.StartedAt).To(BeNil())
			Expect(got.FinishedAt).To(BeNil())
		})

		It("rejects a second CreateJob for the same id without reporting ErrJobNotFound", func() {
			job := pgQueuedJob("22222222-2222-2222-2222-222222222222")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			err := store.CreateJob(ctx, job)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeFalse(), "a duplicate-id failure is a store failure, not a not-found")
		})
	})

	When("GetJob, RecordCompletion, or RecordFailure are called with an id that was never created", func() {
		It("returns an error satisfying errors.Is(err, coachapi.ErrJobNotFound) for each", func() {
			const missing = "33333333-3333-3333-3333-333333333333"

			_, err := store.GetJob(ctx, missing)
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue(), "GetJob err = %v", err)

			err = store.RecordCompletion(ctx, missing, coachapi.Completion{})
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue(), "RecordCompletion err = %v", err)

			err = store.RecordFailure(ctx, missing, "boom", time.Now())
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue(), "RecordFailure err = %v", err)
		})
	})

	When("GetReport is called for a job that has never completed", func() {
		It("returns an error satisfying errors.Is(err, coachapi.ErrJobNotFound)", func() {
			job := pgQueuedJob("44444444-4444-4444-4444-444444444444")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			_, err := store.GetReport(ctx, job.ID)
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue(), "GetReport err = %v", err)
		})
	})

	When("a job attempt fails", func() {
		It("marks the job failed with the recorded error message and finish time", func() {
			job := pgQueuedJob("55555555-5555-5555-5555-555555555555")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			finishedAt := time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC)
			Expect(store.RecordFailure(ctx, job.ID, "clone failed: timeout", finishedAt)).To(Succeed())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusFailed))
			Expect(got.Error).NotTo(BeNil())
			Expect(*got.Error).To(Equal("clone failed: timeout"))
			Expect(got.FinishedAt).NotTo(BeNil())
			Expect(*got.FinishedAt).To(BeTemporally("==", finishedAt))
		})
	})

	When("a job attempt completes successfully", func() {
		It("marks the job completed and GetReport assembles the same report shape MemoryStore produces", func() {
			job := pgQueuedJob("66666666-6666-6666-6666-666666666666")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

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

			Expect(store.RecordCompletion(ctx, job.ID, completion)).To(Succeed())

			gotJob, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotJob.Status).To(Equal(coachapi.JobStatusCompleted))
			Expect(gotJob.Attempt).To(Equal(1))
			Expect(gotJob.FinishedAt).NotTo(BeNil())
			Expect(*gotJob.FinishedAt).To(BeTemporally("==", finishedAt))
			Expect(gotJob.Error).To(BeNil())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())

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

			Expect(report.ReportVersion).To(Equal(wantReport.ReportVersion))
			Expect(report.JobID).To(Equal(wantReport.JobID))
			Expect(report.Kind).To(Equal(wantReport.Kind))
			Expect(report.Params).To(MatchJSON(wantReport.Params))
			Expect(report.CommitSHA).To(Equal(wantReport.CommitSHA))
			Expect(report.Summary).To(Equal(wantReport.Summary))
			Expect(report.Error).To(BeNil())
			Expect(report.Versions).To(Equal(wantReport.Versions))
			Expect(report.GeneratedAt).To(BeTemporally("==", wantReport.GeneratedAt))
			Expect(report.Diagnostics).To(Equal(wantReport.Diagnostics))

			Expect(report.Findings).To(HaveLen(len(wantReport.Findings)))
			for i, want := range wantReport.Findings {
				got := report.Findings[i]
				Expect(got.Source).To(Equal(want.Source), "Findings[%d].Source", i)
				Expect(got.RubricID).To(Equal(want.RubricID), "Findings[%d].RubricID", i)
				Expect(got.RubricVersion).To(Equal(want.RubricVersion), "Findings[%d].RubricVersion", i)
				Expect(got.ModelIdentity).To(Equal(want.ModelIdentity), "Findings[%d].ModelIdentity", i)
				Expect(got.Payload).To(MatchJSON(want.Payload), "Findings[%d].Payload", i)
			}
		})
	})

	When("multiple findings and diagnostics are recorded in one completion", func() {
		It("preserves input order in GetReport by created_at, independent of id ordering", func() {
			// Row ids are assigned in the REVERSE of insertion order, so a
			// report assembly that (incorrectly) fell back to `ORDER BY id`
			// once created_at values collided would produce the opposite
			// order and fail this test -- unlike the report-shape-parity
			// test above, whose two rows happen to have ascending ids that
			// mask exactly this bug.
			job := pgQueuedJob("88888888-8888-8888-8888-888888888888")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			generatedAt := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
			completion := coachapi.Completion{
				Attempt:   1,
				CommitSHA: "abc123def4567890abc123def4567890abc123de",
				Findings: []coachapi.JobFinding{
					{ID: "88888888-0000-0000-0000-000000000005", JobID: job.ID, Attempt: 1, Source: coachapi.FindingSourceDeterministic, Payload: json.RawMessage(`{"rule_id":"rule.a","n":1}`), PayloadHash: "hash-a"},
					{ID: "88888888-0000-0000-0000-000000000004", JobID: job.ID, Attempt: 1, Source: coachapi.FindingSourceDeterministic, Payload: json.RawMessage(`{"rule_id":"rule.a","n":2}`), PayloadHash: "hash-b"},
					{ID: "88888888-0000-0000-0000-000000000003", JobID: job.ID, Attempt: 1, Source: coachapi.FindingSourceDeterministic, Payload: json.RawMessage(`{"rule_id":"rule.a","n":3}`), PayloadHash: "hash-c"},
				},
				Diagnostics: []coachapi.JobDiagnostic{
					{ID: "88888888-0000-0000-0000-000000000002", JobID: job.ID, Attempt: 1, Scope: "file:a.go", Message: "first"},
					{ID: "88888888-0000-0000-0000-000000000001", JobID: job.ID, Attempt: 1, Scope: "file:b.go", Message: "second"},
				},
				Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt:  generatedAt,
				GeneratedAt: generatedAt,
			}
			Expect(store.RecordCompletion(ctx, job.ID, completion)).To(Succeed())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())

			Expect(report.Findings).To(HaveLen(3))
			Expect(report.Findings[0].Payload).To(MatchJSON(`{"rule_id":"rule.a","n":1}`))
			Expect(report.Findings[1].Payload).To(MatchJSON(`{"rule_id":"rule.a","n":2}`))
			Expect(report.Findings[2].Payload).To(MatchJSON(`{"rule_id":"rule.a","n":3}`))

			Expect(report.Diagnostics).To(Equal([]coachapi.Diagnostic{
				{Scope: "file:a.go", Message: "first"},
				{Scope: "file:b.go", Message: "second"},
			}))
		})
	})

	When("job_findings' UNIQUE NULLS NOT DISTINCT constraint is violated", func() {
		It("rejects a duplicate deterministic finding within (job_id, attempt, source, payload_hash) and rolls back the whole attempt", func() {
			job := pgQueuedJob("77777777-7777-7777-7777-777777777777")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

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
			Expect(err).To(HaveOccurred())

			// The whole attempt is one transaction: the constraint violation
			// must roll back the jobs row update too, not leave the job
			// half-completed.
			gotJob, getErr := store.GetJob(ctx, job.ID)
			Expect(getErr).NotTo(HaveOccurred())
			Expect(gotJob.Status).To(Equal(coachapi.JobStatusQueued), "status after rolled-back RecordCompletion should be unchanged")
			Expect(gotJob.Attempt).To(Equal(0), "attempt after rolled-back RecordCompletion should be unchanged")
		})
	})
})
