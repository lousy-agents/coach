package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

type escalationRule struct {
	ruleID     string
	file       func(path string, metric int) codesignal.FileChange
	threshold  int
	atMultiple int
}

var escalationRules = map[string]escalationRule{
	"cognitive_complexity": {
		ruleID:     "complexity.cognitive_complexity",
		file:       ccFile,
		threshold:  15,
		atMultiple: 30,
	},
	"branch_density": {
		ruleID: "complexity.branch_density",
		file: func(path string, metric int) codesignal.FileChange {
			return metricsFile(path, semantics.StructuralMetrics{Ifs: metric})
		},
		threshold:  12,
		atMultiple: 24,
	},
	"max_nesting_depth": {
		ruleID: "complexity.max_nesting_depth",
		file: func(path string, metric int) codesignal.FileChange {
			return metricsFile(path, semantics.StructuralMetrics{MaxNestingDepth: metric})
		},
		threshold:  4,
		atMultiple: 8,
	},
}

func severityByPath(signals []codesignal.Signal) map[string]codesignal.Severity {
	out := map[string]codesignal.Severity{}
	for _, s := range signals {
		out[s.Path] = s.Severity
	}
	return out
}

var _ = Describe("Severity escalation for metric rules", func() {
	DescribeTable("a finding whose metric reaches twice its threshold is more severe than one that only just crosses it",
		func(name string) {
			rule := escalationRules[name]
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				rule.file("large.go", rule.atMultiple+rule.threshold),
				rule.file("barely.go", rule.threshold),
			}})

			signals := signalsByRule(report, rule.ruleID)
			Expect(signals).To(HaveLen(2))
			Expect(severityByPath(signals)).To(Equal(map[string]codesignal.Severity{
				"barely.go": "medium",
				"large.go":  "high",
			}))
		},
		Entry("cognitive_complexity", "cognitive_complexity"),
		Entry("branch_density", "branch_density"),
		Entry("max_nesting_depth", "max_nesting_depth"),
	)

	DescribeTable("the escalation boundary is exactly twice the threshold",
		func(name string) {
			rule := escalationRules[name]
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				rule.file("at.go", rule.atMultiple),
				rule.file("below.go", rule.atMultiple-1),
			}})

			signals := signalsByRule(report, rule.ruleID)
			Expect(signals).To(HaveLen(2))
			Expect(severityByPath(signals)).To(Equal(map[string]codesignal.Severity{
				"below.go": "medium",
				"at.go":    "high",
			}))
		},
		Entry("cognitive_complexity", "cognitive_complexity"),
		Entry("branch_density", "branch_density"),
		Entry("max_nesting_depth", "max_nesting_depth"),
	)

	When("a repository mixes escalated, barely-over, and low-severity findings", func() {
		It("reports more than two distinct severity values", func() {
			constructors := []semantics.Finding{
				{Kind: "constructor_func", Name: "NewA", Location: semantics.Location{StartRow: 1, EndRow: 1}},
				{Kind: "constructor_func", Name: "NewB", Location: semantics.Location{StartRow: 2, EndRow: 2}},
			}
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				{Path: "ctor.go", Status: "modified", Head: cleanResult("ctor.go", constructors...)},
				ccFile("barely.go", 15),
				ccFile("large.go", 30),
			}})

			severities := map[codesignal.Severity]bool{}
			for _, s := range report.Signals {
				severities[s.Severity] = true
			}
			Expect(severities).To(Equal(map[codesignal.Severity]bool{"high": true, "medium": true, "low": true}))
		})
	})

	When("a high-severity signal has lower confidence than a medium-severity signal and loses on path order", func() {
		It("ranks severity above confidence", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				ccFile("a.go", 29),
				metricsFile("z.go", semantics.StructuralMetrics{Ifs: 24}),
			}})

			Expect(report.Signals).To(HaveLen(2))
			Expect(evidenceAndPath(report.Signals)).To(Equal([]string{
				"z.go branch_sum=24",
				"a.go cognitive_complexity=29",
			}))
			Expect(report.Signals[0].Severity).To(Equal(codesignal.Severity("high")))
			Expect(report.Signals[1].Confidence).To(Equal(codesignal.Confidence("high")))
		})
	})
})
