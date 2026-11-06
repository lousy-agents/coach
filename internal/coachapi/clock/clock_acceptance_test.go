package clock_test

import (
	"time"

	"github.com/lousy-agents/coach/internal/coachapi/clock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the production clock", func() {
	It("reads the real wall clock, not a stub zero value", func() {
		var c clock.Clock = clock.RealClock{}

		before := time.Now()
		got := c.Now()
		after := time.Now()

		Expect(got).To(BeTemporally(">=", before), "a clock that runs behind the wall clock")
		Expect(got).To(BeTemporally("<=", after), "a clock that runs ahead of the wall clock: it was read between the two wall-clock reads, so it cannot be later than the second")
	})

	It("fires After no sooner than the duration it was given, and then once", func() {
		var c clock.Clock = clock.RealClock{}
		const duration = 200 * time.Millisecond

		start := time.Now()
		fired := c.After(duration)
		deadline := <-fired
		elapsed := time.Since(start)

		Expect(elapsed).To(BeNumerically(">=", duration), "After fired after %s, before the %s it was asked to wait", elapsed, duration)
		Expect(deadline).To(BeTemporally(">=", start.Add(duration)), "the value it fires with is the deadline, which cannot be earlier than the call plus the duration")
		Consistently(fired, 50*time.Millisecond).ShouldNot(Receive(), "After fires exactly once")
	})

	It("fires After promptly once the duration has elapsed, rather than long after", func() {
		var c clock.Clock = clock.RealClock{}
		const duration = 50 * time.Millisecond

		Eventually(c.After(duration), 10*duration).Should(Receive())
	})
})
