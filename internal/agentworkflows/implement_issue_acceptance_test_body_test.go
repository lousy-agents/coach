package agentworkflows

import (
	"regexp"

	. "github.com/onsi/gomega"
)

func body_implementIssueAcceptanceTest_declaresTheMetaBlockTheWorkflowToolRequires_26() {
	planner := readRepoFile(plannerPath)
	Expect(planner).To(HavePrefix("export const meta = {"),
		"the Workflow tool requires meta as the first statement")
	for _, field := range []string{"name:", "description:", "phases:"} {
		Expect(planner).To(ContainSubstring(field))
	}
	Expect(planner).To(ContainSubstring("name: 'implement-issue-plan'"),
		"the command invokes the workflow by this name")
}

func body_implementIssueAcceptanceTest_neverSpawnsTheImplementerOrReviewerAgentsItself_55() {
	planner := readRepoFile(plannerPath)
	for _, agent := range []string{"task-implementer", "task-reviewer", "workflow-integration-reviewer"} {
		Expect(planner).NotTo(MatchRegexp(`agentType:\s*['"]`+regexp.QuoteMeta(agent)),
			"%s must be spawned by the main session, or its SubagentStop hook never fires", agent)
	}
}

func body_implementIssueAcceptanceTest_grantsItsOwnAgentsNoWayToMutateTheRepository_63() {
	planner := readRepoFile(plannerPath)
	for _, mutator := range []string{"Edit", "Write", "NotebookEdit"} {
		Expect(planner).NotTo(MatchRegexp(`['"]` + regexp.QuoteMeta(mutator) + `['"]`))
	}
	Expect(planner).NotTo(ContainSubstring("isolation: 'worktree'"),
		"a worktree exists to keep parallel mutations from colliding; needing one would mean the planner writes")
}
