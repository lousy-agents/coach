package worker_test

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

func expectDuplicateDeliveriesSettledByJobStatus(ctx context.Context, store *memory.Store, tq *fakeTaskQueue, clock *acceptanceharness.FakeClock, start time.Time) {
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
}

func expectRetryableFailureRedeliversThenCompletes(ctx context.Context, store *memory.Store, tq *fakeTaskQueue, clock *acceptanceharness.FakeClock, start time.Time) {
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
}
