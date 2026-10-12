package baseline_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestBaselineAcceptance runs the Ginkgo acceptance suite for the
// repo_baseline_scan use case, driven through baseline.NewScanHandler.
func TestBaselineAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/coachapi/baseline acceptance suite")
}
