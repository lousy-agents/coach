package agentloop_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestAgentLoopAcceptance is the Ginkgo suite bootstrap for internal/agentloop
// (issue #105 / Task 5: minimal bounded agent tool loop, epic #97). Named with
// the *Acceptance suffix so mise run test-acceptance-fast picks it up.
func TestAgentLoopAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/agentloop acceptance suite")
}
