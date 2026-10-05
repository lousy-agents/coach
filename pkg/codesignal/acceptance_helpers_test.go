package codesignal_test

import (
	"context"

	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/gomega"
)

func build(options codesignal.Options, input codesignal.Input) *codesignal.Report {
	builder, err := codesignal.New(options)
	Expect(err).NotTo(HaveOccurred())
	report, err := builder.Build(context.Background(), input)
	Expect(err).NotTo(HaveOccurred())
	return report
}

func signalsByRule(report *codesignal.Report, ruleID string) []codesignal.Signal {
	var out []codesignal.Signal
	for _, s := range report.Signals {
		if s.RuleID == ruleID {
			out = append(out, s)
		}
	}
	return out
}
