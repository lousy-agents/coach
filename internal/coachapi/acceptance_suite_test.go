package coachapi_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestCoachapiAcceptance runs the Ginkgo acceptance suite for
// internal/coachapi (Task 2 / GitHub issue #103): the JobStore seam and its
// in-memory implementation, exercised entirely through the public
// coachapi.JobStore/coachapi.MemoryStore API.
func TestCoachapiAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/coachapi acceptance suite")
}
