package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

// fakeTaskQueue is an in-memory TaskQueue test double (same contract shape as
// internal/coachapi/queue's acceptance fake). Tests may use it; production
// workers must not poll Postgres for work alongside TaskQueue.
type inFlightEntry struct {
	task queue.Task
}

type fakeTaskQueue struct {
	mu         sync.Mutex
	pending    []queue.Task
	inFlight   map[string]inFlightEntry
	poison     map[string]bool
	attempts   map[string]int
	complete   []queue.Claim
	enqueueN   int
	enqueueErr error
	// permanentNackFailLeft makes the next N permanent Nack calls fail after
	// releasing the claim back to pending (simulating poison publish failure
	// while leaving the source message redeliverable).
	permanentNackFailLeft int
	permanentNackCalls    int
}

var _ queue.TaskQueue = (*fakeTaskQueue)(nil)

// heartbeatMuteStore wraps MemoryStore so tests can stop refreshing
// heartbeat_at while a handler is paused. FakeClock.Advance fires pending
// After waiters; the heartbeat goroutine would otherwise beat at the advanced
// Now() and un-stale the lease before ClaimJob reclaim (CI flake).
type heartbeatMuteStore struct {
	*coachapi.MemoryStore
	mute atomic.Bool
}

var _ coachapi.WorkerJobStore = (*heartbeatMuteStore)(nil)

var _ = Describe("worker job claiming and lifecycle", func() {
	var (
		ctx   context.Context
		store *coachapi.MemoryStore
		tq    *fakeTaskQueue
		clock *acceptanceharness.FakeClock
		start time.Time
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = coachapi.NewMemoryStore()
		tq = newFakeTaskQueue()
		start = time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
		clock = acceptanceharness.NewFakeClock(start)
	})

	When("a job is running under a worker with an injected clock", func() {
		It("updates heartbeat_at every heartbeat interval", func() {
			job := newQueuedJob("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID, Payload: []byte(job.ID)})).To(Succeed())

			handlerEntered := make(chan struct{})
			handlerRelease := make(chan struct{})
			h := func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				return body_workerAcceptanceTest_89(ctx, job, w, handlerEntered, handlerRelease)
			}

			w, err := worker.New(store, tq, clock, h, worker.Config{
				WorkerID:          "worker-a",
				HeartbeatInterval: 15 * time.Second,
				StaleAfter:        60 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			done := make(chan error, 1)
			go func() {
				_, err := w.ProcessNext(ctx)
				done <- err
			}()

			Eventually(handlerEntered).Should(BeClosed())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusRunning))
			Expect(got.HeartbeatAt).NotTo(BeNil())
			firstHB := *got.HeartbeatAt

			advanceUntil(clock, time.Second, 20*time.Second, func() bool {
				return body_workerAcceptanceTest_120(ctx, store, job, firstHB)
			})

			got, err = store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.HeartbeatAt).NotTo(BeNil())
			Expect(got.HeartbeatAt.After(firstHB)).To(BeTrue(), "heartbeat_at must advance after one heartbeat interval")

			close(handlerRelease)
			Eventually(done).Should(Receive(BeNil()))
		})
	})

	When("a running job's heartbeat is older than the stale threshold", func() {
		It("is reclaimable: attempt increments and prior findings are deleted on the new claim", func() {
			job := newQueuedJob("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			lease1, err := store.ClaimJob(ctx, job.ID, "worker-a", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease1.Attempt).To(Equal(1))

			staleFinding := coachapi.JobFinding{
				ID:          "finding-stale",
				JobID:       job.ID,
				Attempt:     1,
				Source:      coachapi.FindingSourceDeterministic,
				Payload:     json.RawMessage(`{"rule_id":"old"}`),
				PayloadHash: "hash-old",
			}
			Expect(store.InsertFindings(ctx, job.ID, "worker-a", 1, []coachapi.JobFinding{staleFinding})).To(Succeed())

			reclaimAt := start.Add(61 * time.Second)
			lease2, err := store.ClaimJob(ctx, job.ID, "worker-b", reclaimAt, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease2.Attempt).To(Equal(2))
			Expect(lease2.WorkerID).To(Equal("worker-b"))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusRunning))
			Expect(got.Attempt).To(Equal(2))
			Expect(got.ClaimedBy).NotTo(BeNil())
			Expect(*got.ClaimedBy).To(Equal("worker-b"))

			newFinding := coachapi.JobFinding{
				ID:          "finding-new",
				JobID:       job.ID,
				Attempt:     2,
				Source:      coachapi.FindingSourceDeterministic,
				Payload:     json.RawMessage(`{"rule_id":"new"}`),
				PayloadHash: "hash-new",
			}
			Expect(store.InsertFindings(ctx, job.ID, "worker-b", 2, []coachapi.JobFinding{newFinding})).To(Succeed())
			Expect(store.CompleteJob(ctx, job.ID, "worker-b", 2, coachapi.Completion{
				Attempt:     2,
				CommitSHA:   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt:  reclaimAt,
				GeneratedAt: reclaimAt,
			})).To(Succeed())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Findings).To(HaveLen(1))
			Expect(string(report.Findings[0].Payload)).To(ContainSubstring(`"rule_id":"new"`))
		})
	})

	When("a worker's fenced write no longer matches (claimed_by, attempt)", func() {
		It("returns ErrClaimLost for heartbeat, findings, diagnostics, and terminal writes, and abandons without acking", func() {
			muted := &heartbeatMuteStore{MemoryStore: store}
			job := newQueuedJob("cccccccc-cccc-cccc-cccc-cccccccccccc")
			job.CreatedAt = start
			Expect(muted.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID, Payload: []byte(job.ID)})).To(Succeed())

			aEntered := make(chan struct{})
			aResume := make(chan struct{})
			var aWriter worker.JobWriter

			handlerA := func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				aWriter = w
				close(aEntered)
				<-aResume

				return successHandler(ctx, job, w)
			}

			wA, err := worker.New(muted, tq, clock, handlerA, worker.Config{
				WorkerID:          "worker-a",
				HeartbeatInterval: 15 * time.Second,
				StaleAfter:        45 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			aDone := make(chan error, 1)
			go func() {
				_, err := wA.ProcessNext(ctx)
				aDone <- err
			}()
			Eventually(aEntered).Should(BeClosed())
			Expect(tq.inFlightCount()).To(Equal(1))
			Expect(tq.completedCount()).To(Equal(0))

			muted.mute.Store(true)
			clock.Advance(46 * time.Second)
			nowB := clock.Now()
			leaseB, err := muted.ClaimJob(ctx, job.ID, "worker-b", nowB, 45*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(leaseB.Attempt).To(Equal(2))
			muted.mute.Store(false)

			err = muted.Heartbeat(ctx, job.ID, "worker-a", 1, clock.Now())
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = muted.InsertFindings(ctx, job.ID, "worker-a", 1, []coachapi.JobFinding{{
				ID: "late", JobID: job.ID, Attempt: 1,
				Source:  coachapi.FindingSourceDeterministic,
				Payload: json.RawMessage(`{}`), PayloadHash: "x",
			}})
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = muted.InsertDiagnostics(ctx, job.ID, "worker-a", 1, []coachapi.JobDiagnostic{{
				ID: "late-diag", JobID: job.ID, Attempt: 1,
				Scope: "file:a.go", Message: "zombie",
			}})
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = muted.CompleteJob(ctx, job.ID, "worker-a", 1, coachapi.Completion{
				Attempt: 1, CommitSHA: "x", FinishedAt: nowB, GeneratedAt: nowB,
				Versions: coachapi.ReportVersions{Analyzer: "a"},
			})
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = muted.FailJob(ctx, job.ID, "worker-a", 1, "late", nowB)
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			close(aResume)
			Eventually(aDone).Should(Receive(MatchError(coachapi.ErrClaimLost)))
			Expect(tq.completedCount()).To(Equal(0), "worker A must not ack after fence loss")
			Expect(tq.inFlightCount()).To(Equal(1), "queue message remains in-flight for redelivery")
			_ = aWriter
		})
	})

	When("the queue delivers a job id that is already terminal or live-running", func() {
		It("acks completed and live-running duplicates, poisons failed duplicates, and claims queued jobs", func() {
			body_workerAcceptanceTest_acksCompletedAndLiveRunningDuplicatesPoisonsFail_273(ctx, store, tq, clock, start)
		})
	})

	When("permanent Nack fails after the job is already recorded failed", func() {
		It("redelivery retries poison dispatch exactly once rather than Completing the source message", func() {
			job := newQueuedJob("poison01-fail-once-0000-000000000001")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			tq.permanentNackFailLeft = 1

			h := func(context.Context, coachapi.Job, worker.JobWriter) (*coachapi.Completion, error) {
				return nil, errors.New("clone failed: permanent")
			}
			wkr, err := worker.New(store, tq, clock, h, worker.Config{WorkerID: "w-poison-retry"})
			Expect(err).NotTo(HaveOccurred())

			ok, err := wkr.ProcessNext(ctx)
			Expect(ok).To(BeTrue())
			Expect(err).To(HaveOccurred(), "first permanent Nack failure must surface")
			Expect(tq.isPoisoned(job.ID)).To(BeFalse(), "poison must not be recorded when Nack(true) fails")
			Expect(tq.pendingCount()).To(Equal(1), "failed poison publish must leave the source message redeliverable")
			Expect(tq.completedCount()).To(Equal(0))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusFailed), "job stays failed after FailJob; redelivery must not re-run the handler")

			ok, err = wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(tq.isPoisoned(job.ID)).To(BeTrue(), "redelivery of a failed job must complete poison dispatch")
			Expect(tq.permanentNackCallCount()).To(Equal(2), "first fail + one successful poison retry")
			Expect(tq.completedCount()).To(Equal(0), "failed jobs must not Complete past poison")
			Expect(tq.pendingCount()).To(Equal(0))
			Expect(tq.inFlightCount()).To(Equal(0))
		})
	})

	When("the queue redelivers a job whose running heartbeat is stale", func() {
		It("reclaims via ClaimJob (attempt+1), runs the handler, and completes", func() {
			job := newQueuedJob("stale000-2345-6789-abcd-ef0123456789")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			_, err := store.ClaimJob(ctx, job.ID, "worker-dead", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.InsertFindings(ctx, job.ID, "worker-dead", 1, []coachapi.JobFinding{{
				ID: "stale-partial", JobID: job.ID, Attempt: 1,
				Source:  coachapi.FindingSourceDeterministic,
				Payload: json.RawMessage(`{"rule_id":"old"}`), PayloadHash: "old",
			}})).To(Succeed())

			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			clock.Advance(61 * time.Second)

			wkr, err := worker.New(store, tq, clock, successHandler, worker.Config{
				WorkerID:   "worker-alive",
				StaleAfter: 60 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(tq.completedCount()).To(Equal(1))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
			Expect(got.Attempt).To(Equal(2))
			Expect(got.ClaimedBy).NotTo(BeNil())
			Expect(*got.ClaimedBy).To(Equal("worker-alive"))

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Findings).To(HaveLen(1))
			Expect(string(report.Findings[0].Payload)).To(ContainSubstring(`"rule_id":"state.hidden_input_mutation"`))
		})
	})

	When("the queue delivers a job id that does not exist in the store", func() {
		It("acks the message so an orphan does not redeliver forever", func() {
			orphanID := "00000000-0000-0000-0000-000000000001"
			Expect(tq.Enqueue(ctx, queue.Task{ID: orphanID})).To(Succeed())

			var handlerCalls atomic.Int32
			h := func(context.Context, coachapi.Job, worker.JobWriter) (*coachapi.Completion, error) {
				handlerCalls.Add(1)
				return nil, errors.New("must not run")
			}
			wkr, err := worker.New(store, tq, clock, h, worker.Config{WorkerID: "orphan-w"})
			Expect(err).NotTo(HaveOccurred())

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(handlerCalls.Load()).To(Equal(int32(0)))
			Expect(tq.completedCount()).To(Equal(1))
			Expect(tq.inFlightCount()).To(Equal(0))
			Expect(tq.pendingCount()).To(Equal(0))
		})
	})

	When("the job handler fails permanently", func() {
		It("records failed with the error and routes the queue message to poison", func() {
			job := newQueuedJob("99999999-9999-9999-9999-999999999999")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			h := func(context.Context, coachapi.Job, worker.JobWriter) (*coachapi.Completion, error) {
				return nil, errors.New("clone failed: permanent")
			}
			wkr, err := worker.New(store, tq, clock, h, worker.Config{WorkerID: "w-fail"})
			Expect(err).NotTo(HaveOccurred())

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(tq.completedCount()).To(Equal(0), "permanent failure must Nack(true), not Complete")
			Expect(tq.isPoisoned(job.ID)).To(BeTrue())
			Expect(tq.inFlightCount()).To(Equal(0))
			Expect(tq.pendingCount()).To(Equal(0))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusFailed))
			Expect(got.Error).NotTo(BeNil())
			Expect(*got.Error).To(Equal("clone failed: permanent"))
		})
	})

	When("the job handler fails with a retryable error below MaxAttempts", func() {
		It("releases the claim, Nacks for redelivery, and completes on a later attempt", func() {
			body_workerAcceptanceTest_releasesTheClaimNacksForRedeliveryAndCompletesOn_463(ctx, store, tq, clock, start)
		})
	})

	When("retryable failures exhaust MaxAttempts", func() {
		It("records failed and routes the queue message to poison", func() {
			job := newQueuedJob("bbbbbbbb-1111-2222-3333-444444444444")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			var calls atomic.Int32
			h := func(context.Context, coachapi.Job, worker.JobWriter) (*coachapi.Completion, error) {
				calls.Add(1)
				return nil, worker.Retryable(errors.New("upstream still warming"))
			}
			wkr, err := worker.New(store, tq, clock, h, worker.Config{
				WorkerID:    "w-exhaust",
				MaxAttempts: 2,
			})
			Expect(err).NotTo(HaveOccurred())

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(calls.Load()).To(Equal(int32(1)))
			Expect(tq.isPoisoned(job.ID)).To(BeFalse())
			Expect(tq.pendingCount()).To(Equal(1))

			ok, err = wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(calls.Load()).To(Equal(int32(2)))
			Expect(tq.isPoisoned(job.ID)).To(BeTrue())
			Expect(tq.completedCount()).To(Equal(0))
			Expect(tq.pendingCount()).To(Equal(0))
			Expect(tq.inFlightCount()).To(Equal(0))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusFailed))
			Expect(got.Error).NotTo(BeNil())
			Expect(*got.Error).To(ContainSubstring("upstream still warming"))
			Expect(got.Attempt).To(Equal(2))
		})
	})

	When("enqueue failed at submit and left a queued row with no in-flight claim", func() {
		It("the requeue reconciler re-enqueues the API versioned task payload so the job can complete", func() {
			job := newQueuedJob("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

			job.CreatedAt = start.Add(-2 * time.Minute)
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			Expect(tq.pendingCount()).To(Equal(0))

			wantPayload, err := coachapi.MarshalTaskPayload(job.ID)
			Expect(err).NotTo(HaveOccurred())

			h := successHandler
			wkr, err := worker.New(store, tq, clock, h, worker.Config{
				WorkerID:           "w-rec",
				ReconcileInterval:  10 * time.Second,
				QueuedAgeThreshold: 30 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(wkr.ReconcileOnce(ctx)).To(Succeed())
			Expect(tq.enqueueCount()).To(BeNumerically(">=", 1))
			Expect(tq.pendingCount()).To(Equal(1))

			pending := tq.pendingTasks()
			Expect(pending).To(HaveLen(1))
			Expect(pending[0].ID).To(Equal(job.ID))
			Expect(pending[0].Payload).To(Equal(wantPayload),
				"reconciler payload must match API MarshalTaskPayload; got %s", pending[0].Payload)
			var decoded map[string]any
			Expect(json.Unmarshal(pending[0].Payload, &decoded)).To(Succeed())
			Expect(decoded).To(Equal(map[string]any{
				"schema_version": float64(coachapi.TaskPayloadSchemaVersion1),
				"job_id":         job.ID,
			}))

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
		})
	})

	When("a running job is stale and the queue message was already acked", func() {
		It("ReconcileOnce releases it to queued, re-enqueues via TaskQueue, and ProcessNext completes the job", func() {
			job := newQueuedJob("deadbeef-0000-0000-0000-000000000001")

			job.CreatedAt = start.Add(-2 * time.Minute)
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			_, err := store.ClaimJob(ctx, job.ID, "worker-dead", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(tq.pendingCount()).To(Equal(0))
			Expect(tq.inFlightCount()).To(Equal(0))

			mid, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(mid.Status).To(Equal(coachapi.JobStatusRunning))

			wkr, err := worker.New(store, tq, clock, successHandler, worker.Config{
				WorkerID:           "w-stale-rec",
				StaleAfter:         60 * time.Second,
				QueuedAgeThreshold: 30 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			clock.Advance(60 * time.Second)
			Expect(wkr.ReconcileOnce(ctx)).To(Succeed())

			released, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(released.Status).To(Equal(coachapi.JobStatusQueued), "ReconcileOnce must ReleaseStaleRunning before re-enqueue")
			Expect(released.ClaimedBy).To(BeNil())
			Expect(tq.enqueueCount()).To(BeNumerically(">=", 1))
			Expect(tq.pendingCount()).To(Equal(1), "reconciler must re-enqueue after releasing stale running")

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
			Expect(got.Attempt).To(Equal(2), "next ClaimJob after release must increment attempt")
			Expect(got.ClaimedBy).NotTo(BeNil())
			Expect(*got.ClaimedBy).To(Equal("w-stale-rec"))
		})
	})

	When("two workers race to claim the same queued job", func() {
		It("never double-claims", func() {
			body_workerAcceptanceTest_neverDoubleClaims_646(ctx, store, start)
		})
	})

	When("a handler crashes after partial findings and diagnostics persist and the job is reclaimed", func() {
		It("the completed report has no duplicate findings or diagnostics from the failed attempt", func() {
			job := newQueuedJob("abcdef01-2345-6789-abcd-ef0123456789")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			lease1, err := store.ClaimJob(ctx, job.ID, "worker-a", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			partial := []coachapi.JobFinding{
				{
					ID: "p1", JobID: job.ID, Attempt: lease1.Attempt,
					Source:  coachapi.FindingSourceDeterministic,
					Payload: json.RawMessage(`{"rule_id":"dup","path":"a.go"}`), PayloadHash: "dup-hash",
				},
				{
					ID: "p2", JobID: job.ID, Attempt: lease1.Attempt,
					Source:  coachapi.FindingSourceDeterministic,
					Payload: json.RawMessage(`{"rule_id":"dup","path":"b.go"}`), PayloadHash: "dup-hash-2",
				},
			}
			Expect(store.InsertFindings(ctx, job.ID, "worker-a", lease1.Attempt, partial)).To(Succeed())
			Expect(store.InsertDiagnostics(ctx, job.ID, "worker-a", lease1.Attempt, []coachapi.JobDiagnostic{{
				ID: "pd1", JobID: job.ID, Attempt: lease1.Attempt,
				Scope: "file:a.go", Message: "partial attempt diagnostic",
			}})).To(Succeed())

			reclaimAt := start.Add(90 * time.Second)
			lease2, err := store.ClaimJob(ctx, job.ID, "worker-b", reclaimAt, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease2.Attempt).To(Equal(2))

			final := []coachapi.JobFinding{{
				ID: "final-1", JobID: job.ID, Attempt: lease2.Attempt,
				Source:  coachapi.FindingSourceDeterministic,
				Payload: json.RawMessage(`{"rule_id":"dup","path":"a.go"}`), PayloadHash: "dup-hash",
			}}
			Expect(store.InsertFindings(ctx, job.ID, "worker-b", lease2.Attempt, final)).To(Succeed())
			Expect(store.CompleteJob(ctx, job.ID, "worker-b", lease2.Attempt, coachapi.Completion{
				Attempt: lease2.Attempt, CommitSHA: "finalsha0000000000000000000000000000001",
				Versions:   coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt: reclaimAt, GeneratedAt: reclaimAt,
			})).To(Succeed())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Findings).To(HaveLen(1), "only the final attempt's findings must appear")
			Expect(string(report.Findings[0].Payload)).To(ContainSubstring(`"path":"a.go"`))
			Expect(report.Diagnostics).To(BeEmpty(), "failed-attempt diagnostics must not appear in the completed report")
		})
	})

	When("worker A pauses and B reclaims (zombie resume)", func() {
		It("rejects A's late heartbeat, findings, and terminal writes; A does not ack", func() {

			muted := &heartbeatMuteStore{MemoryStore: store}
			job := newQueuedJob("feedface-feed-face-feed-facefeedface")
			job.CreatedAt = start
			Expect(muted.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			entered := make(chan struct{})
			release := make(chan struct{})
			handlerA := func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				close(entered)
				<-release

				err := w.InsertFindings(ctx, []coachapi.JobFinding{{
					ID: "zombie", JobID: job.ID, Attempt: w.Lease().Attempt,
					Source:  coachapi.FindingSourceDeterministic,
					Payload: json.RawMessage(`{}`), PayloadHash: "z",
				}})
				return nil, err
			}
			wA, err := worker.New(muted, tq, clock, handlerA, worker.Config{
				WorkerID: "zombie-a", HeartbeatInterval: 15 * time.Second, StaleAfter: 45 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			aDone := make(chan error, 1)
			go func() {
				_, err := wA.ProcessNext(ctx)
				aDone <- err
			}()
			Eventually(entered).Should(BeClosed())

			muted.mute.Store(true)
			clock.Advance(46 * time.Second)
			_, err = muted.ClaimJob(ctx, job.ID, "zombie-b", clock.Now(), 45*time.Second)
			Expect(err).NotTo(HaveOccurred())
			muted.mute.Store(false)

			close(release)
			Eventually(aDone).Should(Receive(MatchError(coachapi.ErrClaimLost)))
			Expect(tq.completedCount()).To(Equal(0))
			Expect(tq.inFlightCount()).To(Equal(1))

			got, err := muted.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Attempt).To(Equal(2))
			Expect(*got.ClaimedBy).To(Equal("zombie-b"))
		})
	})

	When("inspecting the worker package imports", func() {
		It("has no direct Redis or SQS client imports outside queue adapter packages", func() {
			body_workerAcceptanceTest_hasNoDirectRedisOrSQSClientImportsOutsideQueueAd_788()
		})
	})

	When("the reconciler runs", func() {
		It("only re-enqueues via TaskQueue and does not execute job handlers from a Postgres poll", func() {
			job := newQueuedJob("00000000-0000-0000-0000-000000000099")
			job.CreatedAt = start.Add(-time.Hour)
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			var handlerCalls atomic.Int32
			h := func(context.Context, coachapi.Job, worker.JobWriter) (*coachapi.Completion, error) {
				handlerCalls.Add(1)
				return &coachapi.Completion{
					Attempt: 1, CommitSHA: "c", FinishedAt: start, GeneratedAt: start,
					Versions: coachapi.ReportVersions{Analyzer: "a"},
				}, nil
			}
			wkr, err := worker.New(store, tq, clock, h, worker.Config{
				WorkerID: "no-poll", QueuedAgeThreshold: time.Minute,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(wkr.ReconcileOnce(ctx)).To(Succeed())
			Expect(handlerCalls.Load()).To(Equal(int32(0)), "reconciler must not run handlers")
			Expect(tq.pendingCount()).To(Equal(1), "reconciler only enqueues")

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(handlerCalls.Load()).To(Equal(int32(1)))
		})
	})

	When("the StartReconciler parent context is canceled", func() {
		It("allows a subsequent StartReconciler call to run again", func() {
			job := newQueuedJob("aaaaaaaa-bbbb-cccc-dddd-eeeeeeee0010")
			job.CreatedAt = start.Add(-2 * time.Minute)
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.pendingCount()).To(Equal(0))

			wkr, err := worker.New(store, tq, clock, successHandler, worker.Config{
				WorkerID:           "w-rec-restart",
				ReconcileInterval:  10 * time.Second,
				QueuedAgeThreshold: 30 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			parent1, cancel1 := context.WithCancel(ctx)
			wkr.StartReconciler(parent1)
			cancel1()

			time.Sleep(30 * time.Millisecond)

			parent2, cancel2 := context.WithCancel(ctx)
			defer cancel2()
			wkr.StartReconciler(parent2)

			advanceUntil(clock, time.Second, 15*time.Second, func() bool {
				return tq.enqueueCount() >= 1
			})
			Expect(tq.enqueueCount()).To(BeNumerically(">=", 1),
				"second StartReconciler after parent cancel must tick and re-enqueue")
			wkr.StopReconciler()
		})
	})

	When("store.Heartbeat returns context.Canceled while the job context is still live", func() {
		It("does not surface ErrClaimLost and still completes the job", func() {
			mem := coachapi.NewMemoryStore()
			hbStore := &canceledHeartbeatStore{
				MemoryStore: mem,
				entered:     make(chan struct{}),
			}
			job := newQueuedJob("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbb0001")
			job.CreatedAt = start
			Expect(mem.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID, Payload: []byte(job.ID)})).To(Succeed())

			handlerEntered := make(chan struct{})
			handlerRelease := make(chan struct{})
			h := func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				close(handlerEntered)
				<-handlerRelease
				return &coachapi.Completion{
					Attempt: 1, CommitSHA: "c", FinishedAt: start, GeneratedAt: start,
					Versions: coachapi.ReportVersions{Analyzer: "a"},
				}, nil
			}
			wkr, err := worker.New(hbStore, tq, clock, h, worker.Config{
				WorkerID:          "w-hb-canceled-err",
				HeartbeatInterval: 15 * time.Second,
				StaleAfter:        45 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			done := make(chan error, 1)
			go func() {
				_, err := wkr.ProcessNext(ctx)
				done <- err
			}()

			Eventually(handlerEntered).Should(BeClosed())
			clock.Advance(15 * time.Second)
			Eventually(hbStore.entered).Should(BeClosed())

			Consistently(done, "80ms", "5ms").ShouldNot(Receive())
			close(handlerRelease)

			var got error
			Eventually(done).Should(Receive(&got))
			Expect(got).NotTo(HaveOccurred(), "canceled heartbeat must not abort as claim lost")
			Expect(errors.Is(got, coachapi.ErrClaimLost)).To(BeFalse())
			Expect(tq.completedCount()).To(Equal(1))
		})
	})
})

// canceledHeartbeatStore returns context.Canceled from Heartbeat while the
// caller's ctx is still live — models a misclassified shutdown/timeout that
// must not be treated as claim-fence loss.
type canceledHeartbeatStore struct {
	*coachapi.MemoryStore
	entered chan struct{}
	once    sync.Once
}

var _ coachapi.WorkerJobStore = (*canceledHeartbeatStore)(nil)

func newFakeTaskQueue() *fakeTaskQueue {
	return &fakeTaskQueue{
		inFlight: make(map[string]inFlightEntry),
		poison:   make(map[string]bool),
		attempts: make(map[string]int),
	}
}

func (q *fakeTaskQueue) completedCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.complete)
}

func (q *fakeTaskQueue) inFlightCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.inFlight)
}

func (q *fakeTaskQueue) pendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

func (q *fakeTaskQueue) pendingTasks() []queue.Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]queue.Task, len(q.pending))
	copy(out, q.pending)
	return out
}

func (q *fakeTaskQueue) enqueueCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enqueueN
}

func (q *fakeTaskQueue) isPoisoned(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.poison[taskID]
}

func (q *fakeTaskQueue) permanentNackCallCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.permanentNackCalls
}

func newQueuedJob(id string) coachapi.Job {
	return coachapi.Job{
		ID:                id,
		Kind:              coachapi.JobKindRepoBaselineScan,
		Params:            json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
		Status:            coachapi.JobStatusQueued,
		CreatedAt:         time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		Attempt:           0,
		CreatedByProvider: "github",
		CreatedBySubject:  "12345",
		CreatedByLogin:    "octocat",
	}
}

func (s *canceledHeartbeatStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	s.once.Do(func() { close(s.entered) })
	return context.Canceled
}
