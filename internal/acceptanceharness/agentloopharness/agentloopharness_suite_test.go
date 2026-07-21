package agentloopharness_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestAgentLoopHarnessAcceptance is the single Ginkgo suite bootstrap for
// internal/acceptanceharness/agentloopharness, covering GitHub issue #78
// ("Task 0.4: Add provider conformance and agent-loop harness seams"), part
// of epic #73 ("Feature Zero: Offline Acceptance Foundation"): the scripted
// model-gateway stand-in (ScriptedGateway) and the recording tool-call
// broker (RecordingToolRegistry) that a later epic's internal/agentloop
// package will import in its own tests. Named with the *Acceptance suffix
// so `go test -race ./... -run Acceptance` (mise run test-acceptance-fast)
// picks it up, mirroring internal/acceptanceharness's own suite bootstrap.
func TestAgentLoopHarnessAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "internal/acceptanceharness/agentloopharness acceptance suite")
}
