package fakegithub_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

var _ = Describe("fakegithub.NewServer", func() {
	It("panics immediately with a message identifying the nil Fixture, rather than deferring the failure to first request", func() {
		Expect(func() { fakegithub.NewServer(nil) }).To(PanicWith(ContainSubstring("nil Fixture")))
	})
})
