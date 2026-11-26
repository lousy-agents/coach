package evalhistory_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

const (
	cognitive = "complexity.cognitive_complexity"
	density   = "complexity.branch_density"
	nesting   = "complexity.max_nesting_depth"
)

var _ = Describe("a history run over a pinned corpus", func() {
	var (
		h                         history
		since, fixBD, fixV, until string
		signals                   []reportSignal
		corpus                    evalcensus.Corpus
	)

	BeforeEach(func() {
		h = newHistory()
		since = h.commit("start", map[string]string{"p/s.go": unitsSource, "p/v.go": varSource})
		fixBD = h.commit("fix: B and D", map[string]string{"p/s.go": replaceLine(replaceLine(unitsSource, 8, "\tx := 20"), 18, "\tx := 40")})
		h.commit("docs: add a readme", map[string]string{"README.md": "readme\n"})
		fixV = h.commit("fix: V", map[string]string{"p/v.go": replaceLine(varSource, 3, "var V = 2")})
		until = h.commit("fix: a new test", map[string]string{"p/s_test.go": "package p\n"})
		signals = []reportSignal{
			functionSignal(cognitive, "p/s.go", "B", 6, 9),
			functionSignal(cognitive, "p/s.go", "C", 11, 14),
			fileSignal(density, "p/v.go"),
		}
		corpus = corpusFor(h, since, until, goRule(cognitive), goRule(density), goRule(nesting),
			evalcensus.Rule{RuleID: "architecture.ts_only", Languages: []string{"typescript"}})
	})

	When("every input is usable", func() {
		var summary evalhistory.Summary

		BeforeEach(func() {
			var err error
			summary, err = evalhistory.Run(corpus, fakeCoach(signals), 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.Repositories).To(HaveLen(1))
		})

		It("states that it measures association, not causation, and the draw it used", func() {
			Expect(summary.Statement).To(ContainSubstring("association, not causation"))
			Expect(summary.Seed).To(Equal(int64(7)))
			Expect(summary.ControlsPerFlagged).To(Equal(3))
			Expect(summary.FixPattern).To(Equal(evalhistory.FixPattern.String()))
			Expect(summary.CorpusID).To(Equal("history"))
			Expect(summary.Coach.SHA256).To(HaveLen(64))
			Expect(summary.AnalysisFailed()).To(BeFalse())
		})

		It("reports the fix regex's match rate and the fixes that map to no unit", func() {
			repo := summary.Repositories[0]
			Expect(repo.Since).To(Equal(since))
			Expect(repo.Until).To(Equal(until))
			Expect(repo.Commits).To(Equal(4))
			Expect(repo.Fixes).To(Equal(3))
			Expect(repo.MatchRate).To(BeNumerically("~", 0.75, 1e-9))
			Expect(repo.UnmappedFixes).To(Equal(evalhistory.UnmappedFixes{Function: 2, File: 1}))
		})

		It("compares a function rule's flagged units with same-bin controls", func() {
			rule := ruleResult(summary, cognitive)
			Expect(rule.Level).To(Equal(evalhistory.FunctionLevel))
			Expect([]int{rule.Flagged, rule.FlaggedInvolved, rule.Controls, rule.ControlsInvolved, rule.Shortfall, rule.Unplaced}).To(Equal([]int{2, 1, 2, 1, 4, 0}))
			Expect(rule.RiskRatio).To(HaveValue(BeNumerically("~", 1.0, 1e-9)))
			Expect(rule.Low).NotTo(BeNil())
			Expect(rule.High).NotTo(BeNil())
		})

		It("gives a null ratio with its reason when a rule has no control or no finding", func() {
			fileRule := ruleResult(summary, density)
			Expect(fileRule.Level).To(Equal(evalhistory.FileLevel))
			Expect([]int{fileRule.Flagged, fileRule.FlaggedInvolved, fileRule.Controls, fileRule.Shortfall}).To(Equal([]int{1, 1, 0, 3}))
			Expect(fileRule.RiskRatio).To(BeNil())
			Expect(fileRule.Reason).To(Equal("no control"))

			unflagged := ruleResult(summary, nesting)
			Expect(unflagged.Flagged).To(Equal(0))
			Expect(unflagged.Reason).To(Equal("no flagged unit"))
		})

		It("leaves out a rule that does not apply to the repository's language", func() {
			for _, rule := range summary.Repositories[0].Rules {
				Expect(rule.RuleID).NotTo(Equal("architecture.ts_only"))
			}
		})

		It("writes one record per placed finding, valued by whether a fix touched its unit", func() {
			records := evalhistory.Records(summary, "2026-11-26")
			Expect(records).To(HaveLen(3))
			byPath := map[string]evalcensus.Record{}
			for _, record := range records {
				Expect(record.Measure).To(Equal("flagged_vs_unflagged_association"))
				Expect(record.Source.Kind).To(Equal("szz"))
				Expect(record.Rater.Kind).To(Equal("automated"))
				Expect(record.Date).To(Equal("2026-11-26"))
				Expect(record.Target.CorpusID).To(Equal("history"))
				Expect(record.Target.SHA).To(Equal(since))
				Expect(record.Rationale).To(ContainSubstring("association, not causation"))
				Expect(strings.Split(record.RecordID, "-")[0]).To(Equal(strings.Split(evalcensus.NewRecordID(record), "-")[0]), "the id's prefix hashes the record's content")
				key := record.Target.Path
				if record.Target.Subject != nil {
					key += ":" + *record.Target.Subject
				}
				byPath[key] = record
			}

			involved := byPath["p/s.go:B"]
			Expect(involved.Value).To(Equal(json.RawMessage("1")))
			Expect(involved.Source.Ref).To(Equal(fixBD))
			Expect(involved.Target.Lines).To(Equal(&[2]int{7, 10}))
			Expect(involved.Target.Fingerprint).To(HaveValue(Equal("fp-B")))

			untouched := byPath["p/s.go:C"]
			Expect(untouched.Value).To(Equal(json.RawMessage("0")))
			Expect(untouched.Source.Ref).To(Equal("none"))
			Expect(untouched.Target.Lines).To(Equal(&[2]int{12, 15}))

			file := byPath["p/v.go"]
			Expect(file.Value).To(Equal(json.RawMessage("1")))
			Expect(file.Source.Ref).To(Equal(fixV))
			Expect(file.Target.Lines).To(BeNil())
		})

		It("writes records the corpus ledger accepts", func() {
			root := GinkgoT().TempDir()
			Expect(evalcensus.AppendCorpusRecords(root, "history", evalhistory.Records(summary, "2026-11-26"))).To(Succeed())
		})

		It("draws the same controls again from the same seed", func() {
			again, err := evalhistory.Run(corpus, fakeCoach(signals), 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(again.Repositories).To(Equal(summary.Repositories))
		})
	})

	When("a fix cannot be traced", func() {
		It("reports it, with its reason, among the unmapped fixes and still compares the rule", func() {
			g := newHistory()
			Expect(os.MkdirAll(filepath.Join(g.dir, "sub"), 0o755)).To(Succeed())
			g.git("update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("1", 40)+",sub")
			start := g.commit("start", map[string]string{"p/s.go": unitsSource})
			g.git("update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("2", 40)+",sub")
			g.git("commit", "-q", "-m", "fix: bump the submodule")
			bump := g.git("rev-parse", "HEAD")
			end := g.commit("fix: B", map[string]string{"p/s.go": replaceLine(unitsSource, 8, "\tx := 20")})

			summary, err := evalhistory.Run(corpusFor(g, start, end, goRule(cognitive)), fakeCoach(signals[:1]), 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.AnalysisFailed()).To(BeFalse())
			repo := summary.Repositories[0]
			Expect(repo.Fixes).To(Equal(2))
			Expect(repo.UnmappedFixes.Function).To(Equal(1))
			Expect(repo.UnprocessedFixes).To(HaveLen(1))
			Expect(repo.UnprocessedFixes[0].SHA).To(Equal(bump))
			Expect(repo.UnprocessedFixes[0].Reason).To(ContainSubstring("sub"))
			Expect(ruleResult(summary, cognitive).FlaggedInvolved).To(Equal(1))
		})
	})

	When("a finding falls in no unit", func() {
		It("counts it as unplaced and writes no record for it", func() {
			signals = append(signals, functionSignal(cognitive, "p/gone.go", "G", 2, 4))
			summary, err := evalhistory.Run(corpus, fakeCoach(signals), 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(ruleResult(summary, cognitive).Unplaced).To(Equal(1))
			Expect(evalhistory.Records(summary, "2026-11-26")).To(HaveLen(3))
		})
	})

	When("Coach fails on a repository", func() {
		It("marks that repository's analysis failed, with Coach's evidence, and writes no record for it", func() {
			summary, err := evalhistory.Run(corpus, fakeCoach(nil), 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.AnalysisFailed()).To(BeTrue())
			repo := summary.Repositories[0]
			Expect(repo.CoachExit).To(Equal(1))
			Expect(repo.Evidence).To(ContainSubstring("coach: scan failed"))
			Expect(repo.Rules).To(BeEmpty())
			Expect(evalhistory.Records(summary, "2026-11-26")).To(BeEmpty())
		})
	})

	When("a production Go file at the census SHA does not parse", func() {
		It("marks the repository's analysis failed, naming the file, and writes no record for it", func() {
			h.commit("docs: a broken file", map[string]string{"p/broken.go": "package p\n\nfunc {\n"})
			until = h.commit("fix: B again", map[string]string{"p/s.go": replaceLine(unitsSource, 8, "\tx := 21")})
			broken := corpusFor(h, h.git("rev-parse", "HEAD~1"), until, goRule(cognitive))
			summary, err := evalhistory.Run(broken, fakeCoach(signals), 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.AnalysisFailed()).To(BeTrue())
			Expect(summary.Repositories[0].Evidence).To(ContainSubstring("p/broken.go"))
			Expect(summary.Repositories[0].Rules).To(BeEmpty())
			Expect(evalhistory.Records(summary, "2026-11-26")).To(BeEmpty())
		})
	})

	When("Coach's output is not a JSON report", func() {
		It("marks the repository's analysis failed", func() {
			summary, err := evalhistory.Run(corpus, fakeCoachPrinting("not json"), 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(summary.AnalysisFailed()).To(BeTrue())
			Expect(summary.Repositories[0].Evidence).To(ContainSubstring("could not be read"))
		})
	})

	DescribeTable("an unusable input stops the run before any scan",
		func(edit func(*evalcensus.Corpus), message string) {
			edit(&corpus)
			_, err := evalhistory.Run(corpus, fakeCoach(signals), 7)
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("a repository without history_until",
			func(c *evalcensus.Corpus) { c.Repositories[0].HistoryUntil = "" }, `repository "r" has no history_until`),
		Entry("a TypeScript repository",
			func(c *evalcensus.Corpus) { c.Repositories[0].Language = "typescript" }, "TypeScript"),
		Entry("history_until not in the repository",
			func(c *evalcensus.Corpus) {
				c.Repositories[0].HistoryUntil = "0123456789abcdef0123456789abcdef01234567"
			}, "is not a commit"),
		Entry("history_until not after the census SHA",
			func(c *evalcensus.Corpus) { c.Repositories[0].SHA, c.Repositories[0].HistoryUntil = until, since }, "does not descend from"),
	)
})

func ruleResult(summary evalhistory.Summary, ruleID string) evalhistory.RuleResult {
	for _, rule := range summary.Repositories[0].Rules {
		if rule.RuleID == ruleID {
			return rule
		}
	}
	Fail("no result for " + ruleID)
	return evalhistory.RuleResult{}
}
