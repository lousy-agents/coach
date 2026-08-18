// Package agentworkflows holds acceptance specs for the committed agent
// orchestration definitions -- the implement-issue planner workflow and the
// slash command that executes its output. The contracts they encode are
// enforced by hooks that match on where an agent is spawned from, so they are
// verified here rather than left to review.
package agentworkflows

import (
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentWorkflowsAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Workflows Acceptance Suite")
}

// commandSection slices one numbered step out of the command, so a rule
// asserted for one step cannot be satisfied by prose belonging to another.
func commandSection(command, n string) string {
	GinkgoHelper()
	start := strings.Index(command, "\n"+n+". **")
	Expect(start).To(BeNumerically(">", -1), "step %s not found", n)
	rest := command[start+1:]
	if end := regexp.MustCompile(`\n\d+\. \*\*`).FindStringIndex(rest); end != nil {
		return rest[:end[0]]
	}
	return rest
}
