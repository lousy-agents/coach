package agentworkflows

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	plannerPath = "../../.claude/workflows/implement-issue-plan.js"
	commandPath = "../../.claude/commands/implement-issue.md"
)

func readRepoFile(path string) string {
	GinkgoHelper()
	body, err := os.ReadFile(filepath.Clean(path))
	Expect(err).NotTo(HaveOccurred(), "expected %s to be committed", path)
	return string(body)
}

var _ = Describe("implement-issue planner workflow", func() {
	When("the workflow is committed", func() {
		It("declares the meta block the Workflow tool requires", func() {
			planner := readRepoFile(plannerPath)
			Expect(planner).To(HavePrefix("export const meta = {"),
				"the Workflow tool requires meta as the first statement")
			for _, field := range []string{"name:", "description:", "phases:"} {
				Expect(planner).To(ContainSubstring(field))
			}
			Expect(planner).To(ContainSubstring("name: 'implement-issue-plan'"),
				"the command invokes the workflow by this name")
		})

		It("constrains its planning output to a schema", func() {
			Expect(readRepoFile(plannerPath)).To(ContainSubstring("schema:"),
				"an unvalidated plan would reach the executor as free text it cannot reliably act on")
		})
	})

	// Spike C established that the SubagentStop and PreToolUse hooks match on
	// agents spawned by the main session. Agents spawned inside a workflow do
	// not reach them, so moving the implement/review loop into the workflow
	// would silently disable verify-review-verdict.sh and gate-pr-creation.sh
	// -- the run would look identical while enforcing nothing.
	When("the planner is asked to do more than plan", func() {
		It("never spawns the implementer or reviewer agents itself", func() {
			planner := readRepoFile(plannerPath)
			for _, agent := range []string{"task-implementer", "task-reviewer", "workflow-integration-reviewer"} {
				Expect(planner).NotTo(MatchRegexp(`agentType:\s*['"]`+regexp.QuoteMeta(agent)),
					"%s must be spawned by the main session, or its SubagentStop hook never fires", agent)
			}
		})

		It("grants its own agents no way to mutate the repository", func() {
			planner := readRepoFile(plannerPath)
			for _, mutator := range []string{"Edit", "Write", "NotebookEdit"} {
				Expect(planner).NotTo(MatchRegexp(`['"]` + regexp.QuoteMeta(mutator) + `['"]`))
			}
			Expect(planner).NotTo(ContainSubstring("isolation: 'worktree'"),
				"a worktree exists to keep parallel mutations from colliding; needing one would mean the planner writes")
		})
	})
})

var _ = Describe("implement-issue command", func() {
	When("the command orchestrates a run", func() {
		It("delegates planning to the workflow rather than re-deriving it", func() {
			Expect(readRepoFile(commandPath)).To(ContainSubstring("implement-issue-plan"))
		})

		It("runs the implement and review loop from the main session", func() {
			command := readRepoFile(commandPath)
			Expect(command).To(ContainSubstring("Agent tool"),
				"the loop must run as main-session Agent calls so SubagentStop fires on each reviewer")
			Expect(command).To(ContainSubstring("task-implementer"))
			Expect(command).To(ContainSubstring("task-reviewer"))
		})

		It("reviews the integrated diff before shipping", func() {
			Expect(readRepoFile(commandPath)).To(ContainSubstring("workflow-integration-reviewer"),
				"per-task review cannot see cross-task interaction or missing integration glue")
		})

		It("opens the pull request from the main session", func() {
			command := readRepoFile(commandPath)
			Expect(strings.ToLower(command)).To(ContainSubstring("pull request"))
			Expect(command).To(ContainSubstring("PULL_REQUEST_TEMPLATE.md"),
				"the template is the PR contract for coding agents")
		})
	})

	// The gate runs ci-all itself at PR time. A command that tells the
	// orchestrator to skip its own validation would leave the gate's run cold,
	// which measured 391s against ~40s warm -- close enough to the hook
	// timeout to turn a legitimate PR into a deny.
	When("the run reaches validation", func() {
		It("validates with the authoritative task before opening the PR", func() {
			Expect(readRepoFile(commandPath)).To(ContainSubstring("ci-all"))
		})
	})
})
