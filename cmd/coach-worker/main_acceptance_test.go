package main

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

// memoryQueue is a minimal in-process TaskQueue for composition-root tests.
type memoryQueue struct {
	pending []queue.Task
	flight  map[string]queue.Task
	n       int
}

func newMemoryQueue() *memoryQueue {
	return &memoryQueue{flight: make(map[string]queue.Task)}
}

func (q *memoryQueue) Enqueue(_ context.Context, task queue.Task) error {
	q.pending = append(q.pending, task)
	return nil
}

func (q *memoryQueue) Claim(_ context.Context) (queue.Claim, bool, error) {
	if len(q.pending) == 0 {
		return queue.Claim{}, false, nil
	}
	t := q.pending[0]
	q.pending = q.pending[1:]
	q.n++
	token := t.ID + "-t"
	q.flight[token] = t
	return queue.Claim{TaskID: t.ID, Attempt: q.n - 1, Token: token}, true, nil
}

func (q *memoryQueue) Complete(_ context.Context, claim queue.Claim) error {
	delete(q.flight, claim.Token)
	return nil
}

func (q *memoryQueue) Nack(_ context.Context, claim queue.Claim, _ bool) error {
	t := q.flight[claim.Token]
	delete(q.flight, claim.Token)
	q.pending = append(q.pending, t)
	return nil
}

var _ queue.TaskQueue = (*memoryQueue)(nil)

var _ = Describe("cmd/coach-worker composition", func() {
	When("the stub handler is wired through worker.New like main does", func() {
		It("completes a queued job end-to-end via TaskQueue only", func() {
			ctx := context.Background()
			store := coachapi.NewMemoryStore()
			tq := newMemoryQueue()
			clock := acceptanceharness.NewFakeClock(time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC))

			job := coachapi.Job{
				ID:                "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				Kind:              coachapi.JobKindRepoBaselineScan,
				Params:            []byte(`{"repo_owner":"a","repo_name":"b"}`),
				Status:            coachapi.JobStatusQueued,
				CreatedAt:         clock.Now(),
				CreatedByProvider: "github",
				CreatedBySubject:  "1",
				CreatedByLogin:    "octocat",
			}
			Expect(store.CreateJob(ctx, job)).To(Succeed())
			Expect(tq.Enqueue(ctx, queue.Task{ID: job.ID})).To(Succeed())

			w, err := worker.New(store, tq, clock, stubJobHandler, worker.Config{WorkerID: "compose-w"})
			Expect(err).NotTo(HaveOccurred())

			ok, err := w.ProcessNext(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusCompleted))
		})
	})

	When("inspecting cmd/coach-worker production source imports", func() {
		It("does not import Redis/SQS clients outside the queue adapter package path", func() {
			// Composition root may import redisstream (adapter); it must not
			// reach for go-redis / aws-sdk directly.
			_, thisFile, _, ok := runtime.Caller(0)
			Expect(ok).To(BeTrue())
			dir := filepath.Dir(thisFile)

			bannedDirect := []string{
				"github.com/redis/go-redis",
				"github.com/aws/aws-sdk-go",
				"github.com/aws/aws-sdk-go-v2",
			}
			fset := token.NewFileSet()
			entries, err := os.ReadDir(dir)
			Expect(err).NotTo(HaveOccurred())
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
					continue
				}
				path := filepath.Join(dir, e.Name())
				f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
				Expect(err).NotTo(HaveOccurred(), path)
				for _, imp := range f.Imports {
					p := strings.Trim(imp.Path.Value, `"`)
					for _, b := range bannedDirect {
						Expect(p).NotTo(HavePrefix(b), "%s must not import %s directly", e.Name(), b)
					}
				}
			}
		})
	})
})
