package coachapi_test

import (
	"context"
	"fmt"
	"sync"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func body_storeMemoryAcceptanceTest_doesNotRace_249(ctx context.Context, store *coachapi.MemoryStore) {
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("job-%d", i)
			job := newQueuedJob(id)
			errs <- store.CreateJob(ctx, job)
			_, err := store.GetJob(ctx, id)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		Expect(err).NotTo(HaveOccurred())
	}
}
