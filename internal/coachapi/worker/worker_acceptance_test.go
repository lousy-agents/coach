package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func newFakeTaskQueue() *fakeTaskQueue {
	return &fakeTaskQueue{
		inFlight: make(map[string]inFlightEntry),
		poison:   make(map[string]bool),
		attempts: make(map[string]int),
	}
}

func (q *fakeTaskQueue) Enqueue(_ context.Context, task queue.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueErr != nil {
		return q.enqueueErr
	}
	q.enqueueN++
	q.pending = append(q.pending, task)
	return nil
}

func (q *fakeTaskQueue) Claim(_ context.Context) (queue.Claim, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return queue.Claim{}, false, nil
	}
	task := q.pending[0]
	q.pending = q.pending[1:]
	q.attempts[task.ID]++
	attempt := q.attempts[task.ID] - 1
	token := fmt.Sprintf("%s-attempt-%d", task.ID, q.attempts[task.ID])
	claim := queue.Claim{TaskID: task.ID, Attempt: attempt, Token: token}
	q.inFlight[token] = inFlightEntry{task: task}
	return claim, true, nil
}

func (q *fakeTaskQueue) Complete(_ context.Context, claim queue.Claim) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.inFlight[claim.Token]
	if !ok || entry.task.ID != claim.TaskID {
		return errors.New("fakeTaskQueue: stale claim")
	}
	delete(q.inFlight, claim.Token)
	q.complete = append(q.complete, claim)
	return nil
}

func (q *fakeTaskQueue) Nack(_ context.Context, claim queue.Claim, permanent bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.inFlight[claim.Token]
	if !ok || entry.task.ID != claim.TaskID {
		return errors.New("fakeTaskQueue: stale claim")
	}
	delete(q.inFlight, claim.Token)
	if permanent {
		q.permanentNackCalls++
		if q.permanentNackFailLeft > 0 {
			q.permanentNackFailLeft--
			// Source message stays redeliverable; poison was not published.
			q.pending = append(q.pending, entry.task)
			return errors.New("fakeTaskQueue: poison destination unavailable")
		}
		q.poison[claim.TaskID] = true
		return nil
	}
	q.pending = append(q.pending, entry.task)
	return nil
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

var _ queue.TaskQueue = (*fakeTaskQueue)(nil)

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

func successHandler(_ context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
	lease := w.Lease()
	finding := coachapi.JobFinding{
		ID:          fmt.Sprintf("f-%s-%d", job.ID, lease.Attempt),
		JobID:       job.ID,
		Attempt:     lease.Attempt,
		Source:      coachapi.FindingSourceDeterministic,
		Payload:     json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"a.go"}`),
		PayloadHash: fmt.Sprintf("hash-%d", lease.Attempt),
		CreatedAt:   time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := w.InsertFindings(context.Background(), []coachapi.JobFinding{finding}); err != nil {
		return nil, err
	}
	return &coachapi.Completion{
		Attempt:     lease.Attempt,
		CommitSHA:   "abc123def4567890abc123def4567890abc123de",
		Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
		FinishedAt:  time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}, nil
}

// advanceUntil advances clock in small steps until cond is true or max
// total advance is exhausted. Used so FakeClock-driven heartbeats fire
// without wall-clock sleeps.
func advanceUntil(clock *acceptanceharness.FakeClock, step time.Duration, max time.Duration, cond func() bool) {
	GinkgoHelper()
	deadline := time.After(2 * time.Second) // wall bound only for stuck tests
	advanced := time.Duration(0)
	for advanced <= max {
		if cond() {
			return
		}
		select {
		case <-deadline:
			return
		default:
		}
		clock.Advance(step)
		advanced += step
		// Yield so heartbeat/reconciler goroutines scheduled on After can run.
		time.Sleep(time.Millisecond)
	}
}

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
				close(handlerEntered)
				select {
				case <-handlerRelease:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return successHandler(ctx, job, w)
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
				j, err := store.GetJob(ctx, job.ID)
				if err != nil || j.HeartbeatAt == nil {
					return false
				}
				return j.HeartbeatAt.After(firstHB)
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

			// Advance past stale threshold without heartbeats.
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

			// Prior attempt findings must be gone; completing with only the
			// new attempt's findings yields a single-finding report.
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

	// Issue #104 fenced writes: every heartbeat, findings/diagnostics insert,
	// and terminal transition is conditional on (claimed_by, attempt); fence
	// failure abandons without queue ack.
	When("a worker's fenced write no longer matches (claimed_by, attempt)", func() {
		It("returns ErrClaimLost for heartbeat, findings, diagnostics, and terminal writes, and abandons without acking", func() {
			job := newQueuedJob("cccccccc-cccc-cccc-cccc-cccccccccccc")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID, Payload: []byte(job.ID)})).To(Succeed())

			// Worker A will pause mid-handler; B reclaims after stale; A's
			// late writes must all be rejected and A must not Complete.
			aEntered := make(chan struct{})
			aResume := make(chan struct{})
			var aWriter worker.JobWriter

			handlerA := func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				aWriter = w
				close(aEntered)
				<-aResume
				// Late terminal path: try to complete after B reclaimed.
				return successHandler(ctx, job, w)
			}

			wA, err := worker.New(store, tq, clock, handlerA, worker.Config{
				WorkerID:          "worker-a",
				HeartbeatInterval: 15 * time.Second,
				StaleAfter:        45 * time.Second, // 3×15
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

			// Advance past stale; B claims via store (simulating queue redelivery).
			clock.Advance(46 * time.Second)
			nowB := clock.Now()
			leaseB, err := store.ClaimJob(ctx, job.ID, "worker-b", nowB, 45*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(leaseB.Attempt).To(Equal(2))

			// A's late heartbeat / findings / diagnostics / terminal must fail the fence.
			err = store.Heartbeat(ctx, job.ID, "worker-a", 1, clock.Now())
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = store.InsertFindings(ctx, job.ID, "worker-a", 1, []coachapi.JobFinding{{
				ID: "late", JobID: job.ID, Attempt: 1,
				Source:  coachapi.FindingSourceDeterministic,
				Payload: json.RawMessage(`{}`), PayloadHash: "x",
			}})
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = store.InsertDiagnostics(ctx, job.ID, "worker-a", 1, []coachapi.JobDiagnostic{{
				ID: "late-diag", JobID: job.ID, Attempt: 1,
				Scope: "file:a.go", Message: "zombie",
			}})
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = store.CompleteJob(ctx, job.ID, "worker-a", 1, coachapi.Completion{
				Attempt: 1, CommitSHA: "x", FinishedAt: nowB, GeneratedAt: nowB,
				Versions: coachapi.ReportVersions{Analyzer: "a"},
			})
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = store.FailJob(ctx, job.ID, "worker-a", 1, "late", nowB)
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			// Resume A: handler's InsertFindings/CompleteJob path also fences out.
			close(aResume)
			Eventually(aDone).Should(Receive(MatchError(coachapi.ErrClaimLost)))
			Expect(tq.completedCount()).To(Equal(0), "worker A must not ack after fence loss")
			Expect(tq.inFlightCount()).To(Equal(1), "queue message remains in-flight for redelivery")
			_ = aWriter
		})
	})

	// Issue #104 duplicate-delivery disposition:
	// completed → Complete; failed → Nack(true) so poison is not skipped;
	// running with live heartbeat → Complete; queued → claim.
	When("the queue delivers a job id that is already terminal or live-running", func() {
		It("acks completed and live-running duplicates, poisons failed duplicates, and claims queued jobs", func() {
			// completed → Complete
			doneJob := newQueuedJob("dddddddd-dddd-dddd-dddd-dddddddddddd")
			doneJob.CreatedAt = start
			Expect(store.CreateJob(ctx, doneJob)).To(Succeed())
			lease, err := store.ClaimJob(ctx, doneJob.ID, "w", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.CompleteJob(ctx, doneJob.ID, "w", lease.Attempt, coachapi.Completion{
				Attempt: lease.Attempt, CommitSHA: "c", FinishedAt: start, GeneratedAt: start,
				Versions: coachapi.ReportVersions{Analyzer: "a"},
			})).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: doneJob.ID})).To(Succeed())

			// failed → Nack(true) (ADR-006 poison contract on redelivery)
			failJob := newQueuedJob("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
			failJob.CreatedAt = start
			Expect(store.CreateJob(ctx, failJob)).To(Succeed())
			leaseF, err := store.ClaimJob(ctx, failJob.ID, "w", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.FailJob(ctx, failJob.ID, "w", leaseF.Attempt, "boom", start)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: failJob.ID})).To(Succeed())

			// live running → Complete
			liveJob := newQueuedJob("ffffffff-ffff-ffff-ffff-ffffffffffff")
			liveJob.CreatedAt = start
			Expect(store.CreateJob(ctx, liveJob)).To(Succeed())
			_, err = store.ClaimJob(ctx, liveJob.ID, "owner", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(tq.Enqueue(ctx, queue.Task{ID: liveJob.ID})).To(Succeed())

			// queued → claim + run
			queuedJob := newQueuedJob("12345678-1234-1234-1234-1234567890ab")
			queuedJob.CreatedAt = start
			Expect(store.CreateJob(ctx, queuedJob)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: queuedJob.ID})).To(Succeed())

			var handlerCalls atomic.Int32
			h := func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				handlerCalls.Add(1)
				return successHandler(ctx, job, w)
			}
			wkr, err := worker.New(store, tq, clock, h, worker.Config{WorkerID: "disp"})
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < 4; i++ {
				ok, err := wkr.ProcessNext(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue())
			}
			Expect(handlerCalls.Load()).To(Equal(int32(1)), "only the queued job should run the handler")
			Expect(tq.completedCount()).To(Equal(3), "completed + live-running + successful queued job")
			Expect(tq.isPoisoned(failJob.ID)).To(BeTrue(), "failed duplicate must Nack(true) so poison is not skipped")
			Expect(tq.inFlightCount()).To(Equal(0))

			got, err := store.GetJob(ctx, queuedJob.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
		})
	})

	// ADR-006 permanent-failure contract: FailJob before poison must not
	// permanently skip poison if Nack(true) fails and the message redelivers.
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

	// Issue #104: stale running is eligible for queue redelivery / re-dispatch;
	// ProcessNext must reclaim (not ack as live-running).
	When("the queue redelivers a job whose running heartbeat is stale", func() {
		It("reclaims via ClaimJob (attempt+1), runs the handler, and completes", func() {
			job := newQueuedJob("stale000-2345-6789-abcd-ef0123456789")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			// Prior owner claimed then went silent (no heartbeats).
			_, err := store.ClaimJob(ctx, job.ID, "worker-dead", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.InsertFindings(ctx, job.ID, "worker-dead", 1, []coachapi.JobFinding{{
				ID: "stale-partial", JobID: job.ID, Attempt: 1,
				Source:  coachapi.FindingSourceDeterministic,
				Payload: json.RawMessage(`{"rule_id":"old"}`), PayloadHash: "old",
			}})).To(Succeed())

			// Queue redelivery after visibility/pending reclaim.
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			// Advance clock past stale threshold before ProcessNext observes the job.
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

	// Orphan/poison hygiene: unknown job ids must not loop forever on the queue.
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
			job := newQueuedJob("aaaaaaaa-1111-2222-3333-444444444444")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			var calls atomic.Int32
			h := func(c context.Context, j coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				if calls.Add(1) == 1 {
					return nil, worker.Retryable(errors.New("transient clone timeout"))
				}
				return successHandler(c, j, w)
			}
			wkr, err := worker.New(store, tq, clock, h, worker.Config{
				WorkerID:    "w-retry",
				MaxAttempts: 3,
			})
			Expect(err).NotTo(HaveOccurred())

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(calls.Load()).To(Equal(int32(1)))
			Expect(tq.isPoisoned(job.ID)).To(BeFalse())
			Expect(tq.completedCount()).To(Equal(0))
			Expect(tq.pendingCount()).To(Equal(1), "retryable Nack must re-enqueue the task")
			Expect(tq.inFlightCount()).To(Equal(0))

			mid, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(mid.Status).To(Equal(coachapi.JobStatusQueued), "retryable failure must not mark the job failed")
			Expect(mid.Attempt).To(Equal(1), "attempt stays until the next ClaimJob")

			ok, err = wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(calls.Load()).To(Equal(int32(2)))
			Expect(tq.completedCount()).To(Equal(1))
			Expect(tq.isPoisoned(job.ID)).To(BeFalse())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
			Expect(got.Attempt).To(Equal(2))
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
			// Created in the past relative to clock so it exceeds QueuedAgeThreshold.
			job.CreatedAt = start.Add(-2 * time.Minute)
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			// Simulate submit-time enqueue failure: row is queued, queue is empty.
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

			// ADR-006 versioned payloads: reconciler must emit the same wire
			// shape as POST /v1/jobs (schema_version + job_id), not raw job id bytes.
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

	// Issue #104 / epic submit durability: the same reconciler recovers a job
	// stuck running after its queue message was already acked (e.g. duplicate
	// live-running delivery acked the broker while the owner later died).
	// ReleaseStaleRunning → queued + Enqueue → ProcessNext completes.
	When("a running job is stale and the queue message was already acked", func() {
		It("ReconcileOnce releases it to queued, re-enqueues via TaskQueue, and ProcessNext completes the job", func() {
			job := newQueuedJob("deadbeef-0000-0000-0000-000000000001")
			// Old enough that after release, ListQueuedOlderThan still selects it.
			job.CreatedAt = start.Add(-2 * time.Minute)
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			// Owner claimed; queue is empty (message already acked / never pending).
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

			// Heartbeat age == staleAfter must release (same boundary as store ATs).
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
			job := newQueuedJob("11111111-2222-3333-4444-555555555555")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			const n = 20
			var wg sync.WaitGroup
			results := make(chan error, n)
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, err := store.ClaimJob(ctx, job.ID, fmt.Sprintf("w-%d", i), start, 60*time.Second)
					results <- err
				}(i)
			}
			wg.Wait()
			close(results)

			var wins, losses int
			for err := range results {
				if err == nil {
					wins++
					continue
				}
				Expect(errors.Is(err, coachapi.ErrNotClaimable)).To(BeTrue())
				losses++
			}
			Expect(wins).To(Equal(1), "exactly one worker must win the claim")
			Expect(losses).To(Equal(n - 1))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusRunning))
			Expect(got.Attempt).To(Equal(1))
		})
	})

	// Issue #104: crash-after-partial-findings-persist then reclaim yields a
	// completed report with no duplicate findings (single final attempt only).
	// Also covers diagnostics cleanup on reclaim.
	When("a handler crashes after partial findings and diagnostics persist and the job is reclaimed", func() {
		It("the completed report has no duplicate findings or diagnostics from the failed attempt", func() {
			job := newQueuedJob("abcdef01-2345-6789-abcd-ef0123456789")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			// Attempt 1: partial findings + diagnostics, then "crash" (no CompleteJob).
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

			// Reclaim after stale. Direct row-delete is asserted in
			// coachapi MemoryStore/PostgresStore acceptance (export_test /
			// SQL counts); this worker-boundary spec locks the customer-
			// visible report contract.
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
			// Covered in detail by the fenced-write scenario above; this
			// variant drives the full worker A ProcessNext path with clock.
			job := newQueuedJob("feedface-feed-face-feed-facefeedface")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			entered := make(chan struct{})
			release := make(chan struct{})
			handlerA := func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
				close(entered)
				<-release
				// After B reclaims, this InsertFindings must fail the fence.
				err := w.InsertFindings(ctx, []coachapi.JobFinding{{
					ID: "zombie", JobID: job.ID, Attempt: w.Lease().Attempt,
					Source:  coachapi.FindingSourceDeterministic,
					Payload: json.RawMessage(`{}`), PayloadHash: "z",
				}})
				return nil, err
			}
			wA, err := worker.New(store, tq, clock, handlerA, worker.Config{
				WorkerID: "zombie-a", HeartbeatInterval: 15 * time.Second, StaleAfter: 45 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			aDone := make(chan error, 1)
			go func() {
				_, err := wA.ProcessNext(ctx)
				aDone <- err
			}()
			Eventually(entered).Should(BeClosed())

			clock.Advance(46 * time.Second)
			_, err = store.ClaimJob(ctx, job.ID, "zombie-b", clock.Now(), 45*time.Second)
			Expect(err).NotTo(HaveOccurred())

			close(release)
			Eventually(aDone).Should(Receive(MatchError(coachapi.ErrClaimLost)))
			Expect(tq.completedCount()).To(Equal(0))
			Expect(tq.inFlightCount()).To(Equal(1))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Attempt).To(Equal(2))
			Expect(*got.ClaimedBy).To(Equal("zombie-b"))
		})
	})

	When("inspecting the worker package imports", func() {
		It("has no direct Redis or SQS client imports outside queue adapter packages", func() {
			_, thisFile, _, ok := runtime.Caller(0)
			Expect(ok).To(BeTrue())
			dir := filepath.Dir(thisFile)

			entries, err := os.ReadDir(dir)
			Expect(err).NotTo(HaveOccurred())

			banned := []string{
				"github.com/redis/go-redis",
				"github.com/ThreeDotsLabs/watermill-redisstream",
				"github.com/ThreeDotsLabs/watermill-aws",
				"github.com/aws/aws-sdk-go",
				"github.com/aws/aws-sdk-go-v2",
			}
			fset := token.NewFileSet()
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
					continue
				}
				path := filepath.Join(dir, e.Name())
				f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
				Expect(err).NotTo(HaveOccurred(), path)
				for _, imp := range f.Imports {
					path := strings.Trim(imp.Path.Value, `"`)
					for _, b := range banned {
						Expect(path).NotTo(HavePrefix(b), "%s must not import %s (use queue.TaskQueue only)", e.Name(), b)
					}
					// Adapter packages are also banned inside worker proper.
					Expect(path).NotTo(ContainSubstring("/queue/redisstream"), e.Name())
					Expect(path).NotTo(ContainSubstring("/queue/sqs"), e.Name())
				}
			}
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

			// Work still requires TaskQueue.Claim via ProcessNext.
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
			// Parent cancel must tear down the live reconciler so a later
			// StartReconciler is not a permanent no-op (StopReconciler not required).
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

			// Heartbeat returned context.Canceled; that must not be treated as fence loss.
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

func (s *canceledHeartbeatStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	s.once.Do(func() { close(s.entered) })
	return context.Canceled
}

var _ coachapi.WorkerJobStore = (*canceledHeartbeatStore)(nil)
