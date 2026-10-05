package semantics_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// mustAnalyzer builds an Analyzer with default options, failing the spec
// immediately if construction fails (it never should for AnalyzerOptions{}).
func mustAnalyzer() *semantics.Analyzer {
	a, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	Expect(err).NotTo(HaveOccurred())
	return a
}
