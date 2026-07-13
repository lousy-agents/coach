package semantics_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestSemanticsAcceptance runs the Ginkgo acceptance suite documenting the
// user stories and acceptance criteria of GitHub issue #1 ("Feature:
// Semantic Analysis Module") for pkg/semantics, exercised entirely through
// the package's public API (semantics.NewAnalyzer / Analyzer.AnalyzeBytes).
// It complements, rather than replaces, the exhaustive white-box *_test.go
// coverage already in this package (package semantics): see the
// acceptance-coverage matrix in the PR description for which AC each style
// of test carries.
func TestSemanticsAcceptance(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "pkg/semantics acceptance suite")
}
