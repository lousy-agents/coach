package queue_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestCoachAPIQueueAcceptance runs the Ginkgo acceptance suite for
// internal/coachapi/queue documenting GitHub issue #100 ("Task 3a:
// TaskQueue / EventBus ports and Watermill adapters", part of epic #97):
// the application-owned TaskQueue and EventBus ports ADR-006 requires so
// domain code never imports Watermill or backend-specific types, and the
// Capabilities fail-fast startup check. It is entirely offline and uses
// only an inline in-memory test double, never a real broker adapter (those
// are separate follow-on tasks).
func TestCoachAPIQueueAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/coachapi/queue acceptance suite")
}
