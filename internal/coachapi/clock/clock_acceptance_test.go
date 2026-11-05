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

		Expect(got).To(BeTemporally(">=", before))
		Expect(got).To(BeTemporally("<=", after.Add(time.Second)))
	})

	It("fires After once the real duration has elapsed", func() {
		var c clock.Clock = clock.RealClock{}

		Eventually(c.After(10 * time.Millisecond)).Should(Receive())
	})
})
