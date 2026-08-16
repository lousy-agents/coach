package agentworkflows

import (
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The executor's loop is prose an orchestrator follows, so its bounds have to be
// stated in that prose. Nothing else stops a reviewer that keeps returning the
// same finding: the loop runs until a human interrupts it or the session dies,
// and an unattended run burns its whole budget getting nowhere.
var _ = Describe("implement/review loop bounds", func() {
	var command string

	BeforeEach(func() { command = readRepoFile(commandPath) })

	// Each rule has to hold in its own step. Asserted against the whole
	// document, the step-2 cap is satisfied by the step-4 repair cap and vice
	// versa, so deleting either one alone still passes.
	step := func(n string) string {
		start := strings.Index(command, "\n"+n+". **")
		Expect(start).To(BeNumerically(">", -1), "step %s not found", n)
		rest := command[start+1:]
		if end := regexp.MustCompile(`\n\d+\. \*\*`).FindStringIndex(rest); end != nil {
			return rest[:end[0]]
		}
		return rest
	}

	When("a task's reviewer keeps returning findings", func() {
		It("caps the cycles at a stated number", func() {
			// A number, not the word "bounded" -- an orchestrator cannot act on
			// an adjective.
			Expect(step("2")).To(MatchRegexp(`(?si)(at most|maximum of|more than)\s+\*{0,2}(three|3|four|4|five|5)\b`),
				"the cap has to be a count the orchestrator can compare against, stated in the loop it bounds")
		})

		// Findings repeating while the diff moved is progress -- the implementer
		// is chipping at it. An unchanged diff with new findings is also
		// progress. Only both together mean the cycle is spinning, so a rule
		// naming just one of them stops too early or never.
		It("defines no-progress as findings repeating AND the diff not moving", func() {
			Expect(step("2")).To(MatchRegexp(`(?si)same.{0,80}finding`))
			Expect(step("2")).To(MatchRegexp(`(?si)(diff|code|tree).{0,60}(unchanged|has not changed|did not change|no.{0,10}change)`))
		})

		It("stops with a reason drawn from a named set", func() {
			for _, reason := range []string{"repeated-finding", "agent-failure", "ambiguous-product-decision"} {
				Expect(step("2")).To(ContainSubstring(reason),
					"an untyped stop tells the next reader nothing about what to do")
			}
		})

		It("reports the stop rather than opening a PR anyway", func() {
			Expect(step("2")).To(MatchRegexp(`(?si)stop.{0,200}(do not\s+open|without\s+opening|\bno PR\b)`),
				"a bounded loop that still ships is not bounded")
		})
	})

	// ci-all is where wasm-build, the sidecar-built projectmodel suite, and
	// cross-file gofmt/tidy first run against the integrated tree -- none of
	// them exercised by any per-task cycle. Making that the one unrecoverable
	// point puts the most likely failure after the entire token spend.
	When("the full validation suite comes back red", func() {
		It("routes the failure back through implement and review", func() {
			Expect(step("4")).To(MatchRegexp(`(?si)ci-all.{0,400}(back through|re-enter|route.{0,20}back|same.{0,20}loop)`),
				"a red suite must be repairable, not terminal")
		})

		It("carries the failing output as the findings block", func() {
			Expect(step("4")).To(MatchRegexp(`(?si)(failing|command).{0,120}output.{0,160}Reviewer Findings`),
				"the implementer needs the actual failure, not a summary of it")
		})

		It("shares the same cycle bound rather than looping freely", func() {
			Expect(step("4")).To(MatchRegexp(`(?si)(same|shares?).{0,40}(cap|bound|limit)`),
				"an unbounded repair loop just moves the runaway to a later phase")
		})
	})
})

// stopReasons is the set the command must define. Kept here so a future edit
// that renames one in the prose fails rather than silently splitting the
// vocabulary between the command and whatever reads its output.
var stopReasonPattern = regexp.MustCompile(`repeated-finding|agent-failure|ambiguous-product-decision|scope-change|environment-failure|merge-conflict`)

var _ = Describe("stop reason vocabulary", func() {
	It("is defined in one place in the command", func() {
		Expect(stopReasonPattern.FindAllString(readRepoFile(commandPath), -1)).
			NotTo(BeEmpty(), "the vocabulary must exist somewhere the orchestrator reads")
	})
})
