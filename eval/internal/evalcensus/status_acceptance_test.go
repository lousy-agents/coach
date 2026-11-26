package evalcensus_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

var _ = Describe("the analysis_status of each rule on a repository", func() {
	var corpus evalcensus.Corpus
	var goRepo evalcensus.Repository

	BeforeEach(func() {
		var err error
		corpus, err = evalcensus.LoadCorpus(writeJSON(validCorpus()))
		Expect(err).NotTo(HaveOccurred())
		goRepo = corpus.Repositories[0]
	})

	statusOf := func(statuses []evalcensus.RuleStatus, ruleID string) evalcensus.RuleStatus {
		for _, status := range statuses {
			if status.RuleID == ruleID {
				return status
			}
		}
		Fail("no status for " + ruleID)
		return evalcensus.RuleStatus{}
	}

	When("Coach reports zero findings for a rule the repository's language exercises", func() {
		It("records not_exercised, not analysis_failed", func() {
			status := statusOf(evalcensus.StatusesFromReport(corpus, goRepo, coachReport()), "complexity.cognitive_complexity")

			Expect(status.Status).To(Equal(evalcensus.NotExercised))
			Expect(status.Findings).To(BeZero())
		})
	})

	When("Coach reports 12 findings with evidence", func() {
		It("records measured with 12 findings, all adjudicable", func() {
			report := coachReport(repeated(12, signal{RuleID: "complexity.cognitive_complexity", Evidence: "cognitive_complexity=17"})...)

			status := statusOf(evalcensus.StatusesFromReport(corpus, goRepo, report), "complexity.cognitive_complexity")
			Expect(status).To(Equal(evalcensus.RuleStatus{RuleID: "complexity.cognitive_complexity", Status: evalcensus.Measured, Findings: 12, Adjudicable: 12}))
		})
	})

	When("every finding of a rule has empty evidence", func() {
		It("records not_adjudicable_only", func() {
			report := coachReport(signal{RuleID: "complexity.cognitive_complexity"}, signal{RuleID: "complexity.cognitive_complexity", Evidence: ""})

			status := statusOf(evalcensus.StatusesFromReport(corpus, goRepo, report), "complexity.cognitive_complexity")
			Expect(status.Status).To(Equal(evalcensus.NotAdjudicableOnly))
			Expect(status.Findings).To(Equal(2))
			Expect(status.Adjudicable).To(BeZero())
		})
	})

	When("some findings of a rule have evidence and some do not", func() {
		It("records measured and counts only the adjudicable ones as such", func() {
			report := coachReport(signal{RuleID: "complexity.cognitive_complexity"}, signal{RuleID: "complexity.cognitive_complexity", Evidence: "cognitive_complexity=16"})

			status := statusOf(evalcensus.StatusesFromReport(corpus, goRepo, report), "complexity.cognitive_complexity")
			Expect(status.Status).To(Equal(evalcensus.Measured))
			Expect(status.Adjudicable).To(Equal(1))
		})
	})

	When("a rule does not apply to the repository's language", func() {
		It("records out_of_scope, even when the analysis failed", func() {
			fromReport := statusOf(evalcensus.StatusesFromReport(corpus, goRepo, coachReport()), "coupling.deep_relative_import")
			fromFailure := statusOf(evalcensus.StatusesFromFailure(corpus, goRepo, "coach exited 3"), "coupling.deep_relative_import")

			Expect(fromReport.Status).To(Equal(evalcensus.OutOfScope))
			Expect(fromFailure.Status).To(Equal(evalcensus.OutOfScope))
		})
	})

	When("Coach failed on the repository", func() {
		It("records analysis_failed for every rule in scope, with the evidence", func() {
			status := statusOf(evalcensus.StatusesFromFailure(corpus, goRepo, "coach exited 3: incomplete coverage"), "complexity.cognitive_complexity")

			Expect(status.Status).To(Equal(evalcensus.AnalysisFailed))
			Expect(status.Evidence).To(ContainSubstring("incomplete coverage"))
		})
	})

	When("Coach's report cannot be read", func() {
		It("records analysis_failed, naming the parse error", func() {
			status := statusOf(evalcensus.StatusesFromReport(corpus, goRepo, []byte("{not json")), "complexity.cognitive_complexity")

			Expect(status.Status).To(Equal(evalcensus.AnalysisFailed))
			Expect(status.Evidence).To(ContainSubstring("report"))
		})
	})

	When("Coach reports findings of a rule the corpus does not census", func() {
		It("records only the corpus's rules", func() {
			statuses := evalcensus.StatusesFromReport(corpus, goRepo, coachReport(signal{RuleID: "structure.constructor_density"}))

			Expect(statuses).To(HaveLen(len(corpus.Rules)))
		})
	})
})
