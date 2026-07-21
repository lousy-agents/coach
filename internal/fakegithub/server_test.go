package fakegithub_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

// NewServer(nil) is exercised directly here (rather than through a specific
// route) because the bug this spec guards against is in NewServer itself: a
// nil *Fixture must fail fast and obviously at construction time, not
// confusingly on the first incoming request deep inside a handler.
var _ = Describe("fakegithub.NewServer", func() {
	It("panics immediately with a message identifying the nil Fixture, rather than deferring the failure to first request", func() {
		Expect(func() { fakegithub.NewServer(nil) }).To(PanicWith(ContainSubstring("nil Fixture")))
	})
})
