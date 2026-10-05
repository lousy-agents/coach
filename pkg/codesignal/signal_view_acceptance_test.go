package codesignal_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func viewReport(severities ...codesignal.Severity) *codesignal.Report {
	report := &codesignal.Report{
		SchemaVersion: "2",
		Summary:       codesignal.Summary{ActiveSignals: len(severities)},
	}
	for i, severity := range severities {
		report.Signals = append(report.Signals, codesignal.Signal{ID: string(rune('a' + i)), Severity: severity})
	}
	return report
}

func withheldWire(report *codesignal.Report) string {
	encoded, err := json.Marshal(report)
	Expect(err).NotTo(HaveOccurred())
	var document map[string]json.RawMessage
	Expect(json.Unmarshal(encoded, &document)).To(Succeed())
	return string(document["signals_withheld"])
}

var _ = Describe("Narrowed report views", func() {
	When("a cap below one is requested", func() {
		DescribeTable("the view is the full report, never a silently emptied one",
			func(n int) {
				report := viewReport("high", "low")

				view := report.WithTop(n)

				Expect(view.Signals).To(HaveLen(2))
				Expect(view.SignalsWithheld).To(BeNil())
				Expect(withheldWire(view)).To(BeEmpty())
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)
	})

	When("a severity floor is not a severity the report can emit", func() {
		DescribeTable("the view is the full report with no withheld record",
			func(floor codesignal.Severity) {
				report := viewReport("high", "low")

				view := report.WithMinSeverity(floor)

				Expect(view.Signals).To(HaveLen(2))
				Expect(view.SignalsWithheld).To(BeNil())
			},
			Entry("empty", codesignal.Severity("")),
			Entry("unknown", codesignal.Severity("urgent")),
		)
	})

	When("the cap is applied before the severity floor", func() {
		It("accounts for every signal that either narrowing withheld", func() {
			report := viewReport("high", "high", "low", "low")

			view := report.WithTop(3).WithMinSeverity("high")

			Expect(view.Signals).To(HaveLen(2))
			Expect(withheldWire(view)).To(Equal(
				`{"min_severity":"high","below_min_severity":1,"top":3,"beyond_top":1}`))
			Expect(report.Signals).To(HaveLen(4))
		})
	})

	When("the cap is applied twice", func() {
		It("reports the tighter cap and every signal withheld across both", func() {
			report := viewReport("high", "high", "medium", "low")

			view := report.WithTop(3).WithTop(1)

			Expect(view.Signals).To(HaveLen(1))
			Expect(withheldWire(view)).To(Equal(`{"top":1,"beyond_top":3}`))
		})

		It("keeps the tighter cap when a looser one is applied afterwards", func() {
			report := viewReport("high", "high", "medium", "low")

			view := report.WithTop(1).WithTop(3)

			Expect(view.Signals).To(HaveLen(1))
			Expect(withheldWire(view)).To(Equal(`{"top":1,"beyond_top":3}`))
		})
	})

	When("the severity floor is applied twice", func() {
		It("reports the stricter floor and every signal withheld across both", func() {
			report := viewReport("high", "medium", "low")

			view := report.WithMinSeverity("medium").WithMinSeverity("high")

			Expect(view.Signals).To(HaveLen(1))
			Expect(withheldWire(view)).To(Equal(`{"min_severity":"high","below_min_severity":2}`))
		})

		It("keeps the stricter floor when a looser one is applied afterwards", func() {
			report := viewReport("high", "medium", "low")

			view := report.WithMinSeverity("high").WithMinSeverity("low")

			Expect(view.Signals).To(HaveLen(1))
			Expect(withheldWire(view)).To(Equal(`{"min_severity":"high","below_min_severity":2}`))
		})
	})
})
