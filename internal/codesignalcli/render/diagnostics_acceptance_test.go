package render

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("ReportText diagnostic lines", func() {
	When("a Diagnostic carries Side and Revision (AC-VER-3)", func() {
		It("appends a side and revision clause to the diagnostic's own line", func() {
			report := &codesignal.Report{
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.go", Kind: "base_syntax_errors", Message: "unexpected token", Side: "base", Revision: "abc1234"},
				},
			}

			text := ReportText(report)

			Expect(text).To(ContainSubstring("path: a.go, kind: base_syntax_errors, message: unexpected token, side: base, revision: abc1234\n"))
		})
	})

	When("a Diagnostic has no Side or Revision set", func() {
		It("renders exactly the original diagnostic line, with no side or revision clause", func() {
			report := &codesignal.Report{
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.go", Kind: "syntax_errors", Message: "unexpected token"},
				},
			}

			text := ReportText(report)

			Expect(text).To(ContainSubstring("path: a.go, kind: syntax_errors, message: unexpected token\n"))
			Expect(text).NotTo(ContainSubstring("side:"))
			Expect(text).NotTo(ContainSubstring("revision:"))
		})
	})
})
