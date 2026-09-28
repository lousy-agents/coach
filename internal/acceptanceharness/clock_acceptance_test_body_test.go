package acceptanceharness_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func body_clockAcceptanceTest_doesNotFireTheReturnedChannel_28() {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := acceptanceharness.NewFakeClock(start)

	ch := clock.After(5 * time.Second)

	select {
	case <-ch:
		Fail("After channel fired before any Advance call")
	default:
	}

	// Give real time a brief, generous grace window in case of a
	// buggy implementation racing off wall-clock time instead of the
	// fake clock; this is a Consistently check, not a sleep-based
	// race, and the fake clock itself never advances here.
	Consistently(ch, "50ms", "10ms").ShouldNot(Receive())
}

func body_clockAcceptanceTest_firesEveryRegisteredWaiterExactlyOnceWithNoDataR_91() {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := acceptanceharness.NewFakeClock(start)

	const workers = 8
	const iterationsPerWorker = 50
	const totalWaiters = workers * iterationsPerWorker

	fired := make(chan time.Time, totalWaiters)

	var producers sync.WaitGroup
	producers.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer producers.Done()
			for j := 0; j < iterationsPerWorker; j++ {
				ch := clock.After(time.Millisecond)
				// Concurrent Now() reads from a producer goroutine,
				// racing with the Advance loop below, are exactly
				// the "heartbeat ticker under test" scenario the
				// FakeClock doc comment claims is safe.
				_ = clock.Now()
				fired <- <-ch
			}
		}()
	}

	stopAdvancing := make(chan struct{})
	var advancer sync.WaitGroup
	advancer.Add(1)
	go func() {
		defer advancer.Done()
		// Paced with a short ticker rather than a tight busy-loop: this
		// still calls Advance many times, concurrently with the
		// producer goroutines above, without burning CPU spinning on
		// an unpaced default case.
		ticker := time.NewTicker(200 * time.Microsecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopAdvancing:
				return
			case <-ticker.C:
				clock.Advance(time.Millisecond)
			}
		}
	}()

	producers.Wait()
	close(stopAdvancing)
	advancer.Wait()
	close(fired)

	var got []time.Time
	for t := range fired {
		got = append(got, t)
	}
	Expect(got).To(HaveLen(totalWaiters), "every registered After waiter must fire exactly once")
}
