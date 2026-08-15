package claudehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// verdictRegistrationMatchers returns the SubagentStop matchers that route to
// verify-review-verdict.sh -- i.e. the agent types whose final reply is
// required to begin with PASS or FINDINGS.
func verdictRegistrationMatchers() []string {
	GinkgoHelper()

	raw, err := os.ReadFile(filepath.Join("..", "..", ".claude", "settings.json"))
	Expect(err).NotTo(HaveOccurred())

	var settings struct {
		Hooks struct {
			SubagentStop []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Args []string `json:"args"`
				} `json:"hooks"`
			} `json:"SubagentStop"`
		} `json:"hooks"`
	}
	Expect(json.Unmarshal(raw, &settings)).To(Succeed())

	var matchers []string
	for _, reg := range settings.Hooks.SubagentStop {
		for _, h := range reg.Hooks {
			if strings.Contains(strings.Join(h.Args, " "), "verify-review-verdict.sh") {
				matchers = append(matchers, reg.Matcher)
			}
		}
	}
	return matchers
}

// reviewerAgents returns the names of committed agent definitions that
// mandate the PASS/FINDINGS verdict contract.
func reviewerAgents() []string {
	GinkgoHelper()

	paths, err := filepath.Glob(filepath.Join("..", "..", ".claude", "agents", "*.md"))
	Expect(err).NotTo(HaveOccurred())

	var names []string
	for _, p := range paths {
		body, err := os.ReadFile(p)
		Expect(err).NotTo(HaveOccurred())
		text := string(body)
		if strings.Contains(text, "`PASS`") && strings.Contains(text, "`FINDINGS`") {
			names = append(names, strings.TrimSuffix(filepath.Base(p), ".md"))
		}
	}
	return names
}

var _ = Describe("reviewer verdict enforcement", func() {
	// verify-review-verdict.sh is wired by a literal agent-type matcher, so a
	// new reviewer agent is not covered by the existing task-reviewer
	// registration. Adding one without a matching registration silently
	// removes the only mechanical guarantee that a verdict is well-formed --
	// the failure is invisible, because an ungated reviewer looks identical
	// to a gated one until it emits a malformed reply.
	When("the repository defines an agent that must return PASS or FINDINGS", func() {
		It("finds at least one such agent", func() {
			Expect(reviewerAgents()).NotTo(BeEmpty(),
				"a scan returning nothing would make the coverage assertion below vacuous")
		})

		It("registers every one of them with the verdict hook", func() {
			matchers := verdictRegistrationMatchers()
			Expect(matchers).NotTo(BeEmpty())
			for _, agent := range reviewerAgents() {
				Expect(matchers).To(ContainElement(agent),
					"agent %q mandates the PASS/FINDINGS contract but no SubagentStop registration enforces it", agent)
			}
		})
	})

	// Phase 5 of the implement-issue flow reviews the integrated diff, which
	// task-reviewer's contract does not cover: it is scoped to one task's
	// diff, its acceptance criteria, and file:line findings. Cross-task
	// interactions and integration glue have no such location.
	When("the integrated diff needs review", func() {
		It("has a dedicated integration reviewer", func() {
			_, err := os.Stat(filepath.Join("..", "..", ".claude", "agents", "workflow-integration-reviewer.md"))
			Expect(err).NotTo(HaveOccurred(),
				"per-task review cannot detect cross-task interaction or missing integration glue")
		})
	})
})
