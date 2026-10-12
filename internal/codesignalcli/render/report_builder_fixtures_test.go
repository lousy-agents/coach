package render

import (
	"context"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func buildReport(options codesignal.Options, input codesignal.Input) *codesignal.Report {
	builder, err := codesignal.New(options)
	Expect(err).NotTo(HaveOccurred())
	report, err := builder.Build(context.Background(), input)
	Expect(err).NotTo(HaveOccurred())
	return report
}
