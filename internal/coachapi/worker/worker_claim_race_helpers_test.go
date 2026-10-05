package worker_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
)

func expectConcurrentWorkersNeverDoubleClaim(ctx context.Context, store *memory.Store, start time.Time) {
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
