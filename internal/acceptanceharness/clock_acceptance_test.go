package acceptanceharness_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

var _ = Describe("controlled clock seam", func() {
	Context("when a test constructs a FakeClock at a fixed start time", func() {
		It("reports that exact time from Now(), never drifting with wall-clock time", func() {
			start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			clock := acceptanceharness.NewFakeClock(start)

			Expect(clock.Now()).To(Equal(start))

			// Even if real time passes, Now() must not drift until Advance is
			// called explicitly.
			Consistently(clock.Now, "50ms", "10ms").Should(Equal(start))
		})
	})

	Context("when a consumer calls After(d) before any Advance", func() {
		It("does not fire the returned channel", func() {
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
		})
	})

	Context("when Advance moves Now() to or past an After deadline", func() {
		It("fires the channel with the deadline time", func() {
			start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			clock := acceptanceharness.NewFakeClock(start)

			ch := clock.After(5 * time.Second)
			clock.Advance(5 * time.Second)

			Eventually(ch).Should(Receive(Equal(start.Add(5 * time.Second))))
			Expect(clock.Now()).To(Equal(start.Add(5 * time.Second)))
		})

		It("fires even when Advance overshoots the deadline", func() {
			start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			clock := acceptanceharness.NewFakeClock(start)

			ch := clock.After(5 * time.Second)
			clock.Advance(10 * time.Second)

			Eventually(ch).Should(Receive(Equal(start.Add(5 * time.Second))))
		})
	})

	Context("when two After calls with different durations are pending", func() {
		It("fires each one in deadline order as Advance is called incrementally", func() {
			start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			clock := acceptanceharness.NewFakeClock(start)

			shortCh := clock.After(3 * time.Second)
			longCh := clock.After(7 * time.Second)

			clock.Advance(3 * time.Second)

			Eventually(shortCh).Should(Receive(Equal(start.Add(3 * time.Second))))
			Consistently(longCh, "50ms", "10ms").ShouldNot(Receive())

			clock.Advance(4 * time.Second)

			Eventually(longCh).Should(Receive(Equal(start.Add(7 * time.Second))))
		})
	})

	Context("when one goroutine calls After and reads Now() while another concurrently calls Advance", func() {
		It("fires every registered waiter exactly once with no data race (FakeClock.mu guards concurrent access)", func() {
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
		})
	})

	Context("when production code uses RealClock", func() {
		It("Now() reflects the real wall clock, not a stub zero value", func() {
			clock := acceptanceharness.RealClock{}

			before := time.Now()
			got := clock.Now()
			after := time.Now()

			Expect(got).To(BeTemporally(">=", before))
			Expect(got).To(BeTemporally("<=", after.Add(time.Second)))
		})
	})
})
