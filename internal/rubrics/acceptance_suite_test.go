package rubrics_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestRubricsAcceptance is the Ginkgo suite bootstrap for internal/rubrics
// (issue #106 / Task 9: seed LLM-as-judge rubrics, epic #97). Named with the
// *Acceptance suffix so mise run test-acceptance-fast picks it up.
func TestRubricsAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/rubrics acceptance suite")
}
