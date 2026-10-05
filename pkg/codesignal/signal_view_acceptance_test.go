package codesignal_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func viewReport(severities ...codesignal.Severity) codesignal.Report {
	report := codesignal.Report{
		SchemaVersion: "2",
		Summary:       codesignal.Summary{ActiveSignals: len(severities)},
	}
	for i, severity := range severities {
		report.Signals = append(report.Signals, codesignal.Signal{ID: string(rune('a' + i)), Severity: severity})
	}
	return report
}

// mixedLifecycleReport is built, not hand-assembled, so its order is the one
// Build produces: a medium finding introduced by the change sorts ahead of a
// high finding that already existed, because lifecycle outranks severity.
func mixedLifecycleReport() codesignal.Report {
	introducedMedium := codesignal.FileChange{
		Path:          "new.go",
		Status:        "modified",
		Base:          cleanResult("new.go"),
		Head:          resultWithCognitiveComplexity("new.go", ccRecord("fresh", 16, semantics.Location{StartRow: 1, EndRow: 9})),
		ChangedRanges: []codesignal.LineRange{{StartRow: 0, EndRow: 10}},
	}
	existingHigh := codesignal.FileChange{
		Path:   "old.go",
		Status: "modified",
		Base:   resultWithCognitiveComplexity("old.go", ccRecord("legacy", 40, semantics.Location{StartRow: 1, EndRow: 9})),
		Head:   resultWithCognitiveComplexity("old.go", ccRecord("legacy", 40, semantics.Location{StartRow: 1, EndRow: 9})),
	}
	return *build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{introducedMedium, existingHigh}})
}

func withheldWire(report codesignal.Report) string {
	encoded, err := json.Marshal(report)
	Expect(err).NotTo(HaveOccurred())
	var document map[string]json.RawMessage
	Expect(json.Unmarshal(encoded, &document)).To(Succeed())
	return string(document["signals_withheld"])
}

func narrow(report *codesignal.Report, opts codesignal.NarrowOptions) codesignal.Report {
	view, err := report.Narrow(opts)
	Expect(err).NotTo(HaveOccurred())
	return *view
}

var _ = Describe("Narrowed report views", func() {
	When("a high finding that already existed ranks behind a medium finding the change introduced", func() {
		It("has the lifecycle-first order the narrowing must survive", func() {
			report := mixedLifecycleReport()

			Expect(report.Signals).To(HaveLen(2))
			Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(report.Signals[0].Severity).To(Equal(codesignal.Severity("medium")))
			Expect(report.Signals[1].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(report.Signals[1].Severity).To(Equal(codesignal.Severity("high")))
		})

		It("keeps the high finding when a floor of high and a cap of one are requested together", func() {
			report := mixedLifecycleReport()

			view := narrow(&report, codesignal.NarrowOptions{MinSeverity: "high", Top: 1})

			Expect(view.Signals).To(HaveLen(1))
			Expect(view.Signals[0].Severity).To(Equal(codesignal.Severity("high")))
			Expect(withheldWire(view)).To(Equal(
				`{"min_severity":"high","below_min_severity":1,"top":1,"beyond_top":0}`))
			Expect(report.Signals).To(HaveLen(2))
		})

		It("keeps the leading introduced finding when only the cap is requested", func() {
			report := mixedLifecycleReport()

			view := narrow(&report, codesignal.NarrowOptions{Top: 1})

			Expect(view.Signals).To(HaveLen(1))
			Expect(view.Signals[0].Severity).To(Equal(codesignal.Severity("medium")))
			Expect(withheldWire(view)).To(Equal(`{"top":1,"beyond_top":1}`))
		})
	})

	When("both narrowings withhold signals", func() {
		It("accounts for every signal once: the floor first, then the cap over what remains", func() {
			report := viewReport("high", "high", "low", "low")

			view := narrow(&report, codesignal.NarrowOptions{MinSeverity: "high", Top: 1})

			Expect(view.Signals).To(HaveLen(1))
			Expect(withheldWire(view)).To(Equal(
				`{"min_severity":"high","below_min_severity":2,"top":1,"beyond_top":1}`))
			Expect(report.Signals).To(HaveLen(4))
		})
	})

	When("no narrowing is requested", func() {
		It("returns the report itself with no withheld record", func() {
			report := viewReport("high", "low")

			view, err := report.Narrow(codesignal.NarrowOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(view).To(BeIdenticalTo(&report))
			Expect(view.SignalsWithheld).To(BeNil())
			Expect(withheldWire(*view)).To(BeEmpty())
		})
	})

	When("the request cannot be honoured", func() {
		DescribeTable("it returns an error and no view rather than a different view than asked for",
			func(opts codesignal.NarrowOptions, wantErr error) {
				report := viewReport("high", "low")

				view, err := report.Narrow(opts)

				Expect(err).To(MatchError(wantErr))
				Expect(view).To(BeNil())
				Expect(report.Signals).To(HaveLen(2))
				Expect(report.SignalsWithheld).To(BeNil())
			},
			Entry("a floor outside the severities a report emits", codesignal.NarrowOptions{MinSeverity: "urgent"}, codesignal.ErrUnknownSeverityFloor),
			Entry("a floor in the wrong case", codesignal.NarrowOptions{MinSeverity: "HIGH", Top: 1}, codesignal.ErrUnknownSeverityFloor),
			Entry("a negative cap", codesignal.NarrowOptions{Top: -1}, codesignal.ErrNegativeTop),
			Entry("a negative cap beside a valid floor", codesignal.NarrowOptions{MinSeverity: "high", Top: -3}, codesignal.ErrNegativeTop),
		)

		It("refuses to narrow a report that is already narrowed", func() {
			source := viewReport("high", "medium", "low")
			narrowed := narrow(&source, codesignal.NarrowOptions{MinSeverity: "medium"})

			view, err := narrowed.Narrow(codesignal.NarrowOptions{Top: 1})

			Expect(err).To(MatchError(codesignal.ErrAlreadyNarrowed))
			Expect(view).To(BeNil())
			Expect(withheldWire(narrowed)).To(Equal(`{"min_severity":"medium","below_min_severity":1}`))
		})
	})
})
