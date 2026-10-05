package acceptanceharness_test

import (
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
			body_clockAcceptanceTest_doesNotFireTheReturnedChannel_28()
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
			body_clockAcceptanceTest_firesEveryRegisteredWaiterExactlyOnceWithNoDataR_91()
		})
	})

	Context("when Advance is called with a negative duration", func() {
		It("panics instead of silently rewinding Now()", func() {
			start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			clock := acceptanceharness.NewFakeClock(start)

			Expect(func() {
				clock.Advance(-time.Second)
			}).To(Panic())

			// Now() must be unaffected by the rejected call.
			Expect(clock.Now()).To(Equal(start))
		})
	})

	Context("when Advance is called with a zero duration", func() {
		It("does not panic and still fires any waiter whose deadline has already been reached", func() {
			start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			clock := acceptanceharness.NewFakeClock(start)

			// A zero-duration After call registers a waiter whose deadline is
			// already Now(); it fires immediately without needing Advance,
			// but Advance(0) must remain a valid, distinct no-op-on-Now()
			// call rather than a rejected one.
			ch := clock.After(0)
			Eventually(ch).Should(Receive(Equal(start)))

			Expect(func() {
				clock.Advance(0)
			}).NotTo(Panic())

			Expect(clock.Now()).To(Equal(start))
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
