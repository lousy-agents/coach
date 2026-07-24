package rubrics_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestRubricsAcceptance bootstraps the Ginkgo suite for internal/rubrics.
// The *Acceptance suffix is required so mise run test-acceptance-fast picks it up.
func TestRubricsAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/rubrics acceptance suite")
}
