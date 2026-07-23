package worker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestWorkerAcceptance runs the Ginkgo acceptance suite for coach-worker job
// claiming and lifecycle (Task 3 / GitHub issue #104).
func TestWorkerAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/coachapi/worker acceptance suite")
}
