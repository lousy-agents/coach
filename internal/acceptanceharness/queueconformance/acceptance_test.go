package queueconformance

import (
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// referenceVisibilityTimeout is comfortably smaller than reclaimAdvance, so
// Run's clock.Advance(reclaimAdvance) reliably forces the in-memory
// reference adapter's visibility timeout to elapse.
const referenceVisibilityTimeout = time.Minute

// TestQueueConformanceAcceptance self-validates Run (GitHub issue #78,
// epic #73, Task 0.4) against the unexported in-memory reference Queue: it
// proves the conformance suite red-then-green catches a contract violation
// and passes a correct implementation. Matches this repo's *Acceptance
// naming convention, so it is picked up by `go test -race ./... -run
// Acceptance` / `mise run test-acceptance-fast`.
func TestQueueConformanceAcceptance(t *testing.T) {
	Run(t, func(tb testing.TB, clock acceptanceharness.Clock) Queue {
		return newInMemoryQueue(clock, referenceVisibilityTimeout)
	})
}
