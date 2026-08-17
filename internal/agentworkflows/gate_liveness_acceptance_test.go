package agentworkflows

import (
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

// A run whose hooks never registered is indistinguishable from a gated one
// from the inside: the agents still answer, the reviews still return verdicts,
// the PR still opens. Every guarantee in this command is decorative on that
// path, and nothing in the run's own output says so.
//
// This was observed, not theorised. A proving run cloned the repository into a
// session whose project directory was elsewhere; Claude Code binds .claude/ at
// session start, so all four gates, both task subagents, and the planner
// workflow sat on disk unregistered while the orchestrator followed this file
// believing they were live.
var _ = Describe("gate liveness", func() {
	var command string

	BeforeEach(func() { command = readRepoFile(commandPath) })

	When("the orchestrator starts a run", func() {
		// Reading the hook files, or finding .claude/settings.json on disk,
		// proves only that the repository is checked out -- which was true in
		// the failing run. Registration is the thing in question, and the only
		// evidence of it is a hook actually firing.
		// Rev-8's step 0 provoked one hook and inferred the rest. That inference
		// holds only for the all-or-nothing binding failure in §8 -- a whole
		// .claude/ that never registered. It does not hold for the failure this
		// repository has actually shipped twice: a registration whose matcher
		// names an agent that no longer exists, which leaves exactly one gate
		// inert while every other one answers normally. D-16 exists because of
		// it. So each gate has to be provoked on its own.
		It("provokes every gate, not one of them", func() {
			section := commandSection(command, "0")
			for _, gate := range []string{
				"verify-review-verdict.sh",
				"validate-no-git-writes.sh",
				"verify-context-relay.sh",
				"enforce-cycle-ceiling.sh",
				"gate-pr-creation.sh",
			} {
				Expect(section).To(ContainSubstring(gate),
					"%s is never provoked, so a run cannot tell it apart from an absent one", gate)
			}
		})

		// A probe's failure mode is that the action actually happens. Spike C
		// learned this the hard way and scoped its commit probe to a scratch
		// clone with no remote; the constraint has to survive into step 0, where
		// there is no scratch clone. Probing a publish gate on a *clean* tree
		// means a dead gate performs a real push or opens a real pull request.
		It("constructs each probe so a dead gate does something harmless", func() {
			section := commandSection(command, "0")
			Expect(section).To(MatchRegexp(`(?si)dirty|uncommitted|stray file`),
				"the publish probes must run against a deliberately dirty tree, so the denial lands on the worktree check before anything is published")
			// Assert the artifact, not a word order: the probe must name a remote
			// that does not exist, so a dead gate's push has nowhere to go.
			Expect(section).To(ContainSubstring("coach-gate-probe-remote"),
				"if the push gate is dead the push executes; it has to be aimed at a remote that does not exist")
			Expect(section).To(MatchRegexp(`(?si)bogus|nonexistent|does not exist`),
				"and the reason has to be stated, or someone will 'fix' it to a real remote")
			Expect(section).NotTo(MatchRegexp(`(?si)git commit --allow-empty`),
				"a probe that lands a junk commit on a live branch when the gate is dead is the wrong shape")
		})

		It("proves registration by provoking a denial rather than by inspection", func() {
			section := commandSection(command, "0")
			Expect(section).To(MatchRegexp(`(?si)deni(ed|al)|denies`),
				"a probe whose pass condition is anything other than being denied cannot tell a live gate from an absent one")
			Expect(section).To(ContainSubstring("verify-context-relay.sh"),
				"the probe has to name the hook it expects to fire, or a reader cannot check it")
			Expect(section).To(ContainSubstring("## Reviewer Findings"),
				"that hook denies on the absence of this literal heading; the probe is built from it")

			// Mutation testing killed the earlier version of this spec: a step 0
			// that replaced the probe with "confirm .claude/settings.json and the
			// hook scripts exist on disk" passed every assertion above, because
			// it still mentioned denial, the hook, and the heading. Vocabulary is
			// not structure. The probe has to be an action the gate can
			// intercept, and only a tool call is that.
			Expect(section).To(MatchRegexp(`(?si)call the Agent tool with.{0,60}subagent_type`),
				"a check that reads the filesystem cannot distinguish registered from merely present -- that is the whole finding")
			Expect(section).To(ContainSubstring("task-implementer"),
				"the hook only denies for this subagent type; a probe naming another one is never evaluated")
		})

		// The same mutation sweep found that inverting the pass condition --
		// "Being allowed is the pass" -- survived every assertion, because they
		// all tested for the presence of words rather than which outcome each
		// word was bound to. An inverted step 0 is worse than none: it reads a
		// live gate as broken and a dead gate as healthy.
		It("binds the pass to the denial and not to its opposite", func() {
			section := commandSection(command, "0")
			Expect(section).To(MatchRegexp(`(?si)being\s+\*{0,2}denied\*{0,2}\s+is the pass`),
				"the direction has to be stated outright; an orchestrator cannot infer which outcome is good")
			Expect(section).NotTo(MatchRegexp(`(?si)being\s+\*{0,2}(allowed|permitted|accepted)\*{0,2}\s+is the pass`),
				"this is the inversion the sweep produced, and it passed the original specs unchanged")
		})

		It("treats an un-denied probe as a stop, not a warning", func() {
			section := commandSection(command, "0")
			// \*{0,2} for the same reason capIn carries it: these files are
			// markdown, and the emphasis a human reader wants sits inside the
			// phrase the assertion is matching.
			Expect(section).To(MatchRegexp(`(?si)(not|isn't|fails to be)\*{0,2}\s+deni`),
				"the failure direction must be stated: succeeding is the bad outcome here, which inverts the usual reading")
			Expect(section).To(ContainSubstring("environment-failure"),
				"an ungated run needs the same typed stop as any other unusable environment")
			Expect(section).To(MatchRegexp(`(?si)(do not|don't|never)\s+(open|proceed|continue)|stop the run`),
				"continuing past a dead gate is the exact outcome the probe exists to prevent")
		})

		// Agents register from .claude/ exactly as hooks do, so the failure this
		// probe exists to catch takes task-implementer down with it: the call
		// errors on an unknown agent type instead of spawning. A rule written
		// only against "denied" and "ran" leaves the likeliest outcome
		// unclassified, and an unclassified outcome is one an orchestrator
		// talks itself past.
		It("classifies an erroring probe as well as a denied or a running one", func() {
			section := commandSection(command, "0")
			Expect(section).To(MatchRegexp(`(?si)(unknown|unrecognis|unrecogniz|not found|no such).{0,40}(agent|subagent|type)`),
				"the likeliest symptom of the failure being probed for is the probe itself failing to resolve")
		})

		// Without the cause, the reader gets a stop reason and no way to act on
		// it. The fix is not in the repository -- it is in how the session was
		// created -- so the command has to say so.
		It("names the cause a reader can act on", func() {
			section := commandSection(command, "0")
			Expect(section).To(MatchRegexp(`(?si)project director`),
				"the binding is to the session's project directory, and that is what was wrong")
			Expect(section).To(MatchRegexp(`(?si)(session start|startup|at start)`),
				"the binding happens once, at session start -- which is why a later clone does not fix it")
		})

		It("runs before any task is implemented", func() {
			// Index returns -1 for a missing step, which compares less than
			// every real offset -- so an ordering assertion alone passes most
			// loudly when the step does not exist at all.
			probe := strings.Index(command, "\n0. **")
			execute := strings.Index(command, "\n2. **")
			Expect(probe).To(BeNumerically(">", -1), "step 0 not found")
			Expect(execute).To(BeNumerically(">", -1), "step 2 not found")
			Expect(probe).To(BeNumerically("<", execute),
				"a liveness check after the work is spent reports a wasted run rather than preventing one")
		})
	})

	// The planner workflow is registered from the same directory as the hooks,
	// so it is absent under exactly the conditions above. The fallback was
	// written for a harness with no Workflow tool at all and does not read as
	// covering a tool that is present but does not know this workflow's name.
	When("the Workflow tool is present but the workflow is not registered", func() {
		It("routes that case somewhere rather than leaving it undefined", func() {
			section := commandSection(command, "1")
			Expect(section).To(MatchRegexp(`(?si)(not registered|unregistered|unknown workflow|does not know|reports it unknown|scriptPath)`),
				"a present-but-unregistered workflow matches neither the delegate branch nor the no-Workflow-tool branch")
		})
	})
})
