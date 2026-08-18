package agentworkflows

import (
	"os"
	"path/filepath"
	"regexp"

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
			planner := readRepoFile(plannerPath)
			// Anchored to the agent options, not the bare word: `schema:` also
			// appears inside the schema literals themselves, so a substring
			// check stays green after the binding is removed.
			Expect(planner).To(MatchRegexp(`schema:\s*PLAN_SCHEMA`),
				"an unvalidated plan would reach the executor as free text it cannot reliably act on")
			Expect(planner).To(MatchRegexp(`schema:\s*AUDIT_SCHEMA`))
		})
	})

	// Spike C established that the SubagentStop and PreToolUse hooks match on
	// agents spawned by the main session. Agents spawned inside a workflow do
	// not reach them, so moving the implement/review loop into the workflow
	// would silently disable verify-review-verdict.sh and
	// verify-context-relay.sh -- the run would look identical while the review
	// loop lost its fidelity checks.
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
			// "pull request" alone appears in the frontmatter and in prose, so
			// it stays green even if step 5 were rewritten to delegate PR
			// creation into a subagent. Anchor on the ownership language
			// instead: the orchestrator owns git and publishing.
			Expect(command).To(MatchRegexp(`(?i)with your own tool call, from this session`),
				"the PR must be opened by the orchestrator, which owns git and the PR evidence")
			Expect(command).To(ContainSubstring("PULL_REQUEST_TEMPLATE.md"),
				"the template is the PR contract for coding agents")
		})
	})

	// The gate no longer runs the exhaustive suite -- GitHub Actions does, as
	// required parallel jobs on compute that is not the session's. What has to
	// survive is that *something* cheap runs before the PR: an orchestrator that
	// validates nothing locally turns every typo into a full CI round trip.
	When("the run reaches validation", func() {
		It("runs a cheap local check before opening the PR", func() {
			Expect(readRepoFile(commandPath)).To(ContainSubstring("mise run ci-fast"),
				"the per-cycle command is the right cost here; the exhaustive proof is CI's job")
		})

		It("does not tell the orchestrator to re-run the exhaustive suite", func() {
			Expect(readRepoFile(commandPath)).NotTo(ContainSubstring("mise run ci-all"),
				"that is the ~910s duplicate of CI this split removed")
		})
	})
})
