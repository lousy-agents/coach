package agentworkflows

import (
	"regexp"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The executor's loop is prose an orchestrator follows, so its bounds have to be
// stated in that prose. Nothing else stops a reviewer that keeps returning the
// same finding: the loop runs until a human interrupts it or the session dies,
// and an unattended run burns its whole budget getting nowhere.
// capIn extracts a stated cycle cap. Only bounding phrasings count: an earlier
// version accepted "more than 3", which licenses exactly the unbounded loop the
// rule forbids, and passed because the regex treated any number near any
// quantifier as a cap.
func capIn(section string) int {
	GinkgoHelper()
	m := regexp.MustCompile(`(?si)(at most|maximum of|no more than|up to)\s+\*{0,2}(\d+)`).FindStringSubmatch(section)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[2])
	Expect(err).NotTo(HaveOccurred())
	return n
}

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
			Expect(capIn(step("2"))).To(BeNumerically(">", 0),
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

	// The exhaustive suite is where wasm-build, the sidecar-built projectmodel
	// suite, and cross-file gofmt/tidy first run against the integrated tree -- none of
	// them exercised by any per-task cycle. Making that the one unrecoverable
	// point puts the most likely failure after the entire token spend.
	// Step 3 is where step 4's repair path lands, so leaving it unbounded moves
	// the runaway rather than removing it -- and per-step assertions cannot see
	// it from steps 2 or 4.
	When("the integration reviewer keeps returning findings", func() {
		It("caps that loop too", func() {
			Expect(capIn(step("3"))).To(BeNumerically(">", 0),
				"a red ci-all routes into this loop; unbounded here is unbounded overall")
		})

		It("never says to repeat without limit", func() {
			Expect(step("3")).NotTo(MatchRegexp(`(?si)repeat until pass`),
				"an unqualified 'repeat until PASS' is the exact instruction being removed")
		})
	})

	// Two prose copies of one number drift silently; the orchestrator then has
	// two contradictory caps and no way to tell which governs.
	// Steps 2 and 3 state one rule twice, so they must agree. Step 4 is
	// deliberately excluded: its budget is independent, and requiring equality
	// there would forbid tuning repair separately from per-task rework.
	When("one rule is stated in more than one step", func() {
		It("states the same number in both places", func() {
			Expect(capIn(step("3"))).To(Equal(capIn(step("2"))),
				"steps 2 and 3 claim the same rule; a drifting number gives the orchestrator two answers")
		})
	})

	// The gate stopped running ci-all because GHA proves a strict superset in
	// less wall clock, on compute that is not the session's. If step 4 still
	// told the orchestrator to run it, the ~910s would have moved rather than
	// gone -- the same duplicated work, one caller later.
	When("the exhaustive suite runs as required CI checks", func() {
		It("does not also spend the session's compute re-running it", func() {
			// Case-sensitive: an earlier draft matched /CI/i, which "ci-all"
			// satisfies, so the assertion passed against the very text it was
			// written to reject.
			Expect(step("4")).NotTo(ContainSubstring("mise run ci-all"),
				"re-running the full suite locally duplicates GHA at twice the wall clock on the scarcer budget")
			Expect(step("4")).To(MatchRegexp(`GitHub Actions|required check`),
				"the orchestrator has to know where the exhaustive proof comes from, or it will re-add a local run")
		})
	})

	When("the full validation suite comes back red", func() {
		It("routes the failure back through implement and review", func() {
			Expect(step("4")).To(MatchRegexp(`(?si)(ci-all|required check|workflow run|GitHub Actions).{0,400}(back through|re-enter|route.{0,20}back|same.{0,20}loop)`),
				"a red suite must be repairable, not terminal, wherever it is run")
		})

		It("carries the failing output as the findings block", func() {
			Expect(step("4")).To(MatchRegexp(`(?si)(failing|command).{0,120}output.{0,160}Reviewer Findings`),
				"the implementer needs the actual failure, not a summary of it")
		})

		// Sharing the per-task counter would make a task already at its cap
		// unrepairable, so a late validation failure would be terminal after all
		// -- the outcome this path exists to remove. It needs its own budget.
		It("carries its own bound rather than the per-task one", func() {
			Expect(capIn(step("4"))).To(BeNumerically(">", 0),
				"an unbounded repair loop just moves the runaway to a later phase")
			Expect(step("4")).To(MatchRegexp(`(?si)own.{0,30}(cap|bound|budget)|separate from`),
				"sharing the per-task counter makes a capped task unrepairable")
		})

		// ci-all's distinctive failures -- wasm-build, cross-file gofmt, an
		// untidy go.mod -- frequently belong to no single task. Guessing an owner
		// sends a fresh implementer to work outside the scope it was given.
		It("says who receives a failure no single task caused", func() {
			Expect(step("4")).To(MatchRegexp(`(?si)(attribut|scope).{0,400}(several|none|no single)`),
				"an unattributable failure needs a named owner, not a guess")
		})
	})
})
