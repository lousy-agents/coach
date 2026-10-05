package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/postgres"
)

// postgres.Store (Task 2, GitHub issue #103) is exercised against a
// real Postgres 16+ instance, gated on COACH_PG_DSN per this repo's
// real-backend integration test convention: skip cleanly when the env var
// is unset rather than failing or hanging. This is required because
// 0001_init.sql's job_findings UNIQUE NULLS NOT DISTINCT constraint is a
// Postgres 16 feature no in-memory/sqlite double can exercise.
var _ = Describe("postgres.Store", func() {

	var (
		ctx   context.Context
		store *postgres.Store
		pool  *pgxpool.Pool
	)

	BeforeEach(func() {
		dsn := os.Getenv("COACH_PG_DSN")
		if dsn == "" {
			Skip("COACH_PG_DSN not set; skipping Postgres integration test")
		}
		ctx = context.Background()
		store, pool = setupPostgresStore(ctx, dsn)
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
			expectCompletionAssemblesMemoryStoreReportShape(ctx, store)
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

	// Task 3 / #104: claim, fence, and reclaim against real Postgres.
	// Issue #104: reclaim increments attempt and deletes prior findings and
	// diagnostics. Row counts are asserted via SQL so GetReport's final-attempt
	// filter cannot false-green a missing DELETE.
	When("ClaimJob reclaims a stale running job", func() {
		It("increments attempt, deletes prior findings and diagnostics, and fences the previous worker out", func() {
			job := pgQueuedJob("88888888-8888-8888-8888-888888888888")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			start := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
			lease1, err := store.ClaimJob(ctx, job.ID, "worker-a", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease1.Attempt).To(Equal(1))

			Expect(store.InsertFindings(ctx, job.ID, "worker-a", 1, []coachapi.JobFinding{{
				ID:          "88888888-0000-0000-0000-000000000001",
				JobID:       job.ID,
				Attempt:     1,
				Source:      coachapi.FindingSourceDeterministic,
				Payload:     json.RawMessage(`{"rule_id":"old"}`),
				PayloadHash: "hash-old",
				CreatedAt:   start,
			}})).To(Succeed())
			Expect(store.InsertDiagnostics(ctx, job.ID, "worker-a", 1, []coachapi.JobDiagnostic{{
				ID:        "88888888-0000-0000-0000-0000000000d1",
				JobID:     job.ID,
				Attempt:   1,
				Scope:     "file:a.go",
				Message:   "partial crash",
				CreatedAt: start,
			}})).To(Succeed())

			reclaimAt := start.Add(61 * time.Second)
			lease2, err := store.ClaimJob(ctx, job.ID, "worker-b", reclaimAt, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease2.Attempt).To(Equal(2))

			var findingCount, diagCount int
			Expect(pool.QueryRow(ctx, `SELECT COUNT(*) FROM job_findings WHERE job_id = $1`, job.ID).Scan(&findingCount)).To(Succeed())
			Expect(pool.QueryRow(ctx, `SELECT COUNT(*) FROM job_diagnostics WHERE job_id = $1`, job.ID).Scan(&diagCount)).To(Succeed())
			Expect(findingCount).To(Equal(0), "ClaimJob must DELETE prior findings rows")
			Expect(diagCount).To(Equal(0), "ClaimJob must DELETE prior diagnostics rows")

			err = store.Heartbeat(ctx, job.ID, "worker-a", 1, reclaimAt)
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			Expect(store.InsertFindings(ctx, job.ID, "worker-b", 2, []coachapi.JobFinding{{
				ID:          "88888888-0000-0000-0000-000000000002",
				JobID:       job.ID,
				Attempt:     2,
				Source:      coachapi.FindingSourceDeterministic,
				Payload:     json.RawMessage(`{"rule_id":"new"}`),
				PayloadHash: "hash-new",
				CreatedAt:   reclaimAt,
			}})).To(Succeed())
			Expect(store.CompleteJob(ctx, job.ID, "worker-b", 2, coachapi.Completion{
				Attempt:     2,
				CommitSHA:   "abc123def4567890abc123def4567890abc123de",
				Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt:  reclaimAt,
				GeneratedAt: reclaimAt,
			})).To(Succeed())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Findings).To(HaveLen(1))
			Expect(report.Findings[0].Payload).To(MatchJSON(`{"rule_id":"new"}`))
			Expect(report.Diagnostics).To(BeEmpty())
		})
	})

	// Reviewer finding #1: fenced inserts must not succeed if ClaimJob reclaim
	// commits between the fence check and the INSERT (TOCTOU).
	When("InsertFindings holds an open fenced insert transaction and another connection reclaims the job", func() {
		It("returns ErrClaimLost and does not persist the zombie worker's findings", func() {
			expectZombieInsertLosesToReclaim(ctx, store)
		})
	})

	// Reviewer finding #2: inserts must stamp lease jobID/attempt, not client fields.
	When("InsertFindings is given findings stamped with a wrong Attempt", func() {
		It("persists the fenced lease attempt so GetReport includes them", func() {
			job := pgQueuedJob("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			start := time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC)
			lease, err := store.ClaimJob(ctx, job.ID, "worker-a", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease.Attempt).To(Equal(1))

			Expect(store.InsertFindings(ctx, job.ID, "worker-a", lease.Attempt, []coachapi.JobFinding{{
				ID:          "aaaaaaaa-0000-0000-0000-000000000001",
				JobID:       job.ID,
				Attempt:     0, // client-supplied wrong attempt must not win
				Source:      coachapi.FindingSourceDeterministic,
				Payload:     json.RawMessage(`{"rule_id":"stamped"}`),
				PayloadHash: "hash-stamped",
				CreatedAt:   start,
			}})).To(Succeed())

			Expect(store.CompleteJob(ctx, job.ID, "worker-a", lease.Attempt, coachapi.Completion{
				Attempt:     lease.Attempt,
				CommitSHA:   "abc123def4567890abc123def4567890abc123de",
				Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt:  start,
				GeneratedAt: start,
			})).To(Succeed())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Findings).To(HaveLen(1), "findings must be stored under the lease attempt, not client Attempt")
			Expect(report.Findings[0].Payload).To(MatchJSON(`{"rule_id":"stamped"}`))
		})
	})

	// Reviewer finding #3: age == staleAfter must be reclaimable (match MemoryStore).
	When("a running job's heartbeat age equals staleAfter exactly", func() {
		It("allows ClaimJob to reclaim and ReleaseStaleRunning to release", func() {
			job := pgQueuedJob("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			const staleAfter = 60 * time.Second
			start := time.Date(2026, 7, 23, 14, 0, 0, 0, time.UTC)
			_, err := store.ClaimJob(ctx, job.ID, "worker-a", start, staleAfter)
			Expect(err).NotTo(HaveOccurred())

			// Boundary: now.Sub(heartbeat) == staleAfter must be stale.
			boundary := start.Add(staleAfter)
			lease2, err := store.ClaimJob(ctx, job.ID, "worker-b", boundary, staleAfter)
			Expect(err).NotTo(HaveOccurred(), "ClaimJob at age==staleAfter must reclaim (got %v)", err)
			Expect(lease2.Attempt).To(Equal(2))
			Expect(lease2.WorkerID).To(Equal("worker-b"))

			// Re-claim as C then test ReleaseStaleRunning at the same boundary.
			job2 := pgQueuedJob("cccccccc-cccc-cccc-cccc-cccccccccccc")
			Expect(store.CreateJob(ctx, job2)).To(Succeed())
			_, err = store.ClaimJob(ctx, job2.ID, "worker-a", start, staleAfter)
			Expect(err).NotTo(HaveOccurred())

			released, err := store.ReleaseStaleRunning(ctx, boundary, staleAfter)
			Expect(err).NotTo(HaveOccurred())
			Expect(released).To(BeNumerically(">=", 1))

			got, err := store.GetJob(ctx, job2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusQueued), "ReleaseStaleRunning at age==staleAfter must release")
		})
	})

	// Issue #104: two concurrent workers never double-claim (Postgres exclusivity).
	// MemoryStore already race-tests this; production safety is the SQL claim predicate.
	When("many goroutines race ClaimJob on the same queued job against Postgres", func() {
		It("exactly one worker wins and the rest observe ErrNotClaimable", func() {
			expectExactlyOneConcurrentClaimWins(ctx, store)
		})
	})
})
