package memory_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestMemoryStoreAcceptance runs the Ginkgo acceptance suite for the
// in-memory coachapi.WorkerJobStore (Task 2 / GitHub issue #103),
// exercised entirely through the public memory.Store API.
func TestMemoryStoreAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/coachapi/store/memory acceptance suite")
}
