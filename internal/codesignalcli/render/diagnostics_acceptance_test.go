package render

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
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

	// AC-VER-3 (issue #334): the head and base entries for one unincorporated
	// candidate file stay distinguishable on their text lines too.
	When("the same candidate file is unincorporated at both head and base", func() {
		It("names that side and revision on each text line", func() {
			coverage := func() *domain.Coverage {
				return &domain.Coverage{Phase: "model", Diagnostics: []domain.Diagnostic{{
					Code:    domain.DiagRootScopeIncomplete,
					Message: "candidate file was never incorporated into the import model",
					Path:    "package.json",
				}}}
			}
			report := buildReport(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				Scope:               codesignal.Scope{Revision: "head-sha", Base: "base-sha"},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &domain.Coverage{Phase: "full", Complete: false},
				BaseProjectCoverage: &domain.Coverage{Phase: "full", Complete: false},
				HeadModelCoverage:   coverage(),
				BaseModelCoverage:   coverage(),
			})

			text := ReportText(report)

			Expect(text).To(MatchRegexp(`(?m)^path: package\.json, kind: project_root_scope_incomplete, message: head revision head-sha .*, side: head, revision: head-sha$`))
			Expect(text).To(MatchRegexp(`(?m)^path: package\.json, kind: base_project_root_scope_incomplete, message: base revision base-sha .*, side: base, revision: base-sha$`))
		})
	})
})
