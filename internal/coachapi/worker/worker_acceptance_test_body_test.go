package worker_test

import (
	"context"
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

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

func body_workerAcceptanceTest_89(ctx context.Context, job coachapi.Job, w worker.JobWriter, handlerEntered chan struct{}, handlerRelease chan struct{}) (*coachapi.Completion, error) {
	close(handlerEntered)
	select {
	case <-handlerRelease:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return successHandler(ctx, job, w)
}

func body_workerAcceptanceTest_120(ctx context.Context, store *coachapi.MemoryStore, job coachapi.Job, firstHB time.Time) bool {
	j, err := store.GetJob(ctx, job.ID)
	if err != nil || j.HeartbeatAt == nil {
		return false
	}
	return j.HeartbeatAt.After(firstHB)
}

func body_workerAcceptanceTest_acksCompletedAndLiveRunningDuplicatesPoisonsFail_273(ctx context.Context, store *coachapi.MemoryStore, tq *fakeTaskQueue, clock *acceptanceharness.FakeClock, start time.Time) {

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

	failJob := newQueuedJob("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	failJob.CreatedAt = start
	Expect(store.CreateJob(ctx, failJob)).To(Succeed())
	leaseF, err := store.ClaimJob(ctx, failJob.ID, "w", start, 60*time.Second)
	Expect(err).NotTo(HaveOccurred())
	Expect(store.FailJob(ctx, failJob.ID, "w", leaseF.Attempt, "boom", start)).To(Succeed())
	Expect(tq.Enqueue(ctx, queue.Task{ID: failJob.ID})).To(Succeed())

	liveJob := newQueuedJob("ffffffff-ffff-ffff-ffff-ffffffffffff")
	liveJob.CreatedAt = start
	Expect(store.CreateJob(ctx, liveJob)).To(Succeed())
	_, err = store.ClaimJob(ctx, liveJob.ID, "owner", start, 60*time.Second)
	Expect(err).NotTo(HaveOccurred())
	Expect(tq.Enqueue(ctx, queue.Task{ID: liveJob.ID})).To(Succeed())

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

func body_workerAcceptanceTest_releasesTheClaimNacksForRedeliveryAndCompletesOn_463(ctx context.Context, store *coachapi.MemoryStore, tq *fakeTaskQueue, clock *acceptanceharness.FakeClock, start time.Time) {
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

func body_workerAcceptanceTest_neverDoubleClaims_646(ctx context.Context, store *coachapi.MemoryStore, start time.Time) {
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
}

func body_workerAcceptanceTest_hasNoDirectRedisOrSQSClientImportsOutsideQueueAd_788() {
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
		(&sigbodyworkerAcceptanceTesthasNoDirectRedisOrSQSClientImport{banned: banned, e: e, f: f}).call()

	}
}
