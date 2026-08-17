package agentworkflows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Since the exhaustive suite moved to CI, a red PR is where a run *normally*
// ends up rather than where it fails. That makes the repair loop a first-class
// path, and three of its edges were left undefined: what happens to commits it
// pushes, which task owns a failure that belongs to no file, and what a stop
// looks like once the pull request already exists.
var _ = Describe("the repair loop after the PR is open", func() {
	var command string

	BeforeEach(func() { command = readRepoFile(commandPath) })

	// Every other push in the run goes through the publish gate. Repair pushes
	// are the ones most likely to carry a hurried fix, and they land on a branch
	// that already has a pull request describing it.
	When("a repair pushes a fix", func() {
		It("goes through the same publish gate as any other push", func() {
			// Anchored on "repair": step 4 already discusses the gate and pushing
			// in other contexts, so a loose proximity match here passed against
			// prose that says nothing about repair pushes at all.
			Expect(commandSection(command, "4")).To(MatchRegexp(`(?si)repair push.{0,200}(gate|clean|ci-gate)`),
				"a repair push that skips the gate publishes a tree nothing checked, onto a branch a PR already describes")
		})
	})

	// platform-smoke runs Docker and live services against the whole stack. Its
	// failures are almost never traceable to one task's declared files, so the
	// generic "zero or several matches" rule would be applied by judgement every
	// time. Naming it removes the judgement call.
	When("platform-smoke is the job that failed", func() {
		It("routes there without attempting attribution", func() {
			Expect(commandSection(command, "4")).To(MatchRegexp(`(?si)platform-smoke.{0,220}(integration-repair|no single|unattributable)`),
				"platform-smoke exercises the whole stack; guessing an owner sends an implementer outside its scope")
		})
	})

	// A CCR container is reclaimed on inactivity, and the repair loop is the
	// longest-lived phase of a run. Without a rule the orchestrator has to invent
	// one, and the tempting invention is to treat a half-finished repair as done.
	When("the session dies mid-repair", func() {
		It("is a typed stop, not an implied success", func() {
			Expect(commandSection(command, "4")).To(MatchRegexp(`(?si)(session|container).{0,160}environment-failure`),
				"an interrupted repair must not read as a completed one")
		})
	})

	// Rev-8 said the terminal state was "PR opened" while the repair loop
	// continued past it. Resolved in favour of continuing -- which leaves the
	// question this answers: what a stop means once the PR exists.
	When("the run stops after the PR is already open", func() {
		It("leaves the PR open and red, and says so on the PR itself", func() {
			section := commandSection(command, "5")
			Expect(section).To(MatchRegexp(`(?si)(open and red|leave it open|remains? open)`),
				"closing or abandoning it silently loses the work and the evidence")
			Expect(section).To(MatchRegexp(`(?si)comment`),
				"the typed reason has to land where the next reader looks, which is the PR")
			Expect(section).To(MatchRegexp(`(?si)(never|do not|don't)\s+(force|merge|force-merge)`),
				"a red PR must not be merged to make the run look finished")
		})
	})
})
