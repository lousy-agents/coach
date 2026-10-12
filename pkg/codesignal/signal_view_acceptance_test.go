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

// narrowRecovering turns a panic into a returned value so a missing guard
// fails the spec rather than aborting the suite.
func narrowRecovering(report *codesignal.Report, opts codesignal.NarrowOptions) (view *codesignal.Report, err error, panicked any) {
	defer func() { panicked = recover() }()
	view, err = report.Narrow(opts)
	return view, err, nil
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

	When("the caller's report lists its signals in reverse rank order", func() {
		It("keeps the highest-ranked signal under a cap of one", func() {
			built := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				ccFile("a.go", 16),
				ccFile("z.go", 29),
			}})
			Expect(evidenceAndPath(built.Signals)).To(Equal([]string{
				"z.go cognitive_complexity=29",
				"a.go cognitive_complexity=16",
			}))
			report := *built
			report.Signals = []codesignal.Signal{built.Signals[1], built.Signals[0]}

			view := narrow(&report, codesignal.NarrowOptions{Top: 1})

			Expect(evidenceAndPath(view.Signals)).To(Equal([]string{"z.go cognitive_complexity=29"}))
			Expect(evidenceAndPath(report.Signals)).To(Equal([]string{
				"a.go cognitive_complexity=16",
				"z.go cognitive_complexity=29",
			}))
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

		It("refuses a request that narrows nothing when the report is already narrowed", func() {
			source := viewReport("high", "medium", "low")
			narrowed := narrow(&source, codesignal.NarrowOptions{MinSeverity: "medium"})

			view, err := narrowed.Narrow(codesignal.NarrowOptions{})

			Expect(err).To(MatchError(codesignal.ErrAlreadyNarrowed))
			Expect(view).To(BeNil())
		})

		DescribeTable("refuses a nil report with an error rather than panicking",
			func(opts codesignal.NarrowOptions) {
				var report *codesignal.Report

				view, err, panicked := narrowRecovering(report, opts)

				Expect(panicked).To(BeNil())
				Expect(err).To(MatchError(codesignal.ErrNilReport))
				Expect(view).To(BeNil())
			},
			Entry("with a cap", codesignal.NarrowOptions{Top: 1}),
			Entry("with a floor", codesignal.NarrowOptions{MinSeverity: "high"}),
			Entry("with no narrowing", codesignal.NarrowOptions{}),
		)

		It("reports an invalid option on a narrowed report as already narrowed, the first thing wrong with the call", func() {
			source := viewReport("high", "low")
			narrowed := narrow(&source, codesignal.NarrowOptions{Top: 1})

			_, err := narrowed.Narrow(codesignal.NarrowOptions{MinSeverity: "urgent"})

			Expect(err).To(MatchError(codesignal.ErrAlreadyNarrowed))
		})
	})
})

var _ = Describe("A withheld record that Narrow cannot produce", func() {
	DescribeTable("fails to encode rather than reading as an unnarrowed or smaller analysis",
		func(withheld codesignal.SignalsWithheld) {
			report := viewReport("high", "low")
			report.SignalsWithheld = &withheld

			encoded, err := json.Marshal(report)

			Expect(err).To(MatchError(codesignal.ErrInvalidSignalsWithheld), "encoded as %s", encoded)
		},
		Entry("a floor count without its floor", codesignal.SignalsWithheld{BelowMinSeverity: 3}),
		Entry("a cap count without its cap", codesignal.SignalsWithheld{BeyondTop: 2}),
		Entry("a negative cap", codesignal.SignalsWithheld{Top: -2, BeyondTop: 1}),
		Entry("a negative floor count", codesignal.SignalsWithheld{MinSeverity: "high", BelowMinSeverity: -1}),
		Entry("a negative cap count", codesignal.SignalsWithheld{Top: 2, BeyondTop: -1}),
		Entry("an empty record", codesignal.SignalsWithheld{}),
		Entry("a floor outside the severity vocabulary", codesignal.SignalsWithheld{MinSeverity: "critical"}),
		Entry("a known severity spelled in another case", codesignal.SignalsWithheld{MinSeverity: "HIGH", BelowMinSeverity: 1}),
		Entry("an unknown floor beside an otherwise valid cap", codesignal.SignalsWithheld{MinSeverity: "urgent", BelowMinSeverity: 1, Top: 2, BeyondTop: 1}),
	)
})
