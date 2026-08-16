// Package agentworkflows holds acceptance specs for the committed agent
// orchestration definitions -- the implement-issue planner workflow and the
// slash command that executes its output. The contracts they encode are
// enforced by hooks that match on where an agent is spawned from, so they are
// verified here rather than left to review.
package agentworkflows

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentWorkflowsAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Workflows Acceptance Suite")
}
