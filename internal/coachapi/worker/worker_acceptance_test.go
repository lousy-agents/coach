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

func (q *fakeTaskQueue) enqueueCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enqueueN
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

	When("a worker's fenced write no longer matches (claimed_by, attempt)", func() {
		It("returns ErrClaimLost and the worker abandons without acking the queue message", func() {
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

			// A's late heartbeat / findings / terminal must fail the fence.
			err = store.Heartbeat(ctx, job.ID, "worker-a", 1, clock.Now())
			Expect(errors.Is(err, coachapi.ErrClaimLost)).To(BeTrue())

			err = store.InsertFindings(ctx, job.ID, "worker-a", 1, []coachapi.JobFinding{{
				ID: "late", JobID: job.ID, Attempt: 1,
				Source:  coachapi.FindingSourceDeterministic,
				Payload: json.RawMessage(`{}`), PayloadHash: "x",
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

	When("the queue delivers a job id that is already terminal or live-running", func() {
		It("acks completed/failed and live-running duplicates, and claims queued jobs", func() {
			// completed → ack
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

			// failed → ack
			failJob := newQueuedJob("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
			failJob.CreatedAt = start
			Expect(store.CreateJob(ctx, failJob)).To(Succeed())
			leaseF, err := store.ClaimJob(ctx, failJob.ID, "w", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.FailJob(ctx, failJob.ID, "w", leaseF.Attempt, "boom", start)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: failJob.ID})).To(Succeed())

			// live running → ack
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
			Expect(tq.completedCount()).To(Equal(4))
			Expect(tq.inFlightCount()).To(Equal(0))

			got, err := store.GetJob(ctx, queuedJob.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
		})
	})

	When("the job handler fails permanently", func() {
		It("records failed with the error and acks the queue message", func() {
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
			Expect(tq.completedCount()).To(Equal(1))
			Expect(tq.inFlightCount()).To(Equal(0))

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusFailed))
			Expect(got.Error).NotTo(BeNil())
			Expect(*got.Error).To(Equal("clone failed: permanent"))
		})
	})

	When("enqueue failed at submit and left a queued row with no in-flight claim", func() {
		It("the requeue reconciler re-enqueues so the job can complete", func() {
			job := newQueuedJob("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
			// Created in the past relative to clock so it exceeds QueuedAgeThreshold.
			job.CreatedAt = start.Add(-2 * time.Minute)
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			// Simulate submit-time enqueue failure: row is queued, queue is empty.
			Expect(tq.pendingCount()).To(Equal(0))

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

			ok, err := wkr.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
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

	When("a handler crashes after partial findings persist and the job is reclaimed", func() {
		It("the completed report has no duplicate findings from the failed attempt", func() {
			job := newQueuedJob("abcdef01-2345-6789-abcd-ef0123456789")
			job.CreatedAt = start
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			// Attempt 1: partial findings, then "crash" (no CompleteJob).
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

			// Reclaim after stale.
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
})
