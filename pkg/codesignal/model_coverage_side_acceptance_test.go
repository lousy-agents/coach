package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func incompleteModelCoverage(paths ...string) *domain.Coverage {
	coverage := &domain.Coverage{Phase: "model", Complete: false}
	for _, path := range paths {
		coverage.Diagnostics = append(coverage.Diagnostics, domain.Diagnostic{
			Code:    domain.DiagRootScopeIncomplete,
			Message: "candidate file was never incorporated into the import model",
			Path:    path,
		})
	}
	coverage.Diagnostics = append(coverage.Diagnostics, domain.Diagnostic{
		Code:    "ts_reachability_local_call_not_followed_gap",
		Message: "routine reachability gap",
		Path:    "pkg/handlers/gap.ts",
	})
	return coverage
}

func diagnosticsAtPath(report *codesignal.Report, path string) []codesignal.Diagnostic {
	var matches []codesignal.Diagnostic
	for _, d := range report.Diagnostics {
		if d.Path == path {
			matches = append(matches, d)
		}
	}
	return matches
}

// AC-VER-3 (issue #334, SA-280-008): a file the project model never
// incorporated is named once per incomplete side, and the two entries for
// the same path stay distinguishable in JSON and in text.
var _ = Describe("Side-attributed model-coverage diagnostics (AC-VER-3)", func() {
	When("the same candidate file is unincorporated at both head and base", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			report = build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				Scope:               codesignal.Scope{Revision: "head-sha", Base: "base-sha"},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &domain.Coverage{Phase: "full", Complete: false},
				BaseProjectCoverage: &domain.Coverage{Phase: "full", Complete: false},
				HeadModelCoverage:   incompleteModelCoverage("package.json"),
				BaseModelCoverage:   incompleteModelCoverage("package.json"),
			})
		})

		It("emits one entry per side with that side's kind, side, and revision", func() {
			Expect(diagnosticsAtPath(report, "package.json")).To(ConsistOf(
				And(HaveField("Kind", domain.DiagRootScopeIncomplete), HaveField("Side", "head"), HaveField("Revision", "head-sha"),
					HaveField("Message", HavePrefix("head revision head-sha project model coverage: "))),
				And(HaveField("Kind", "base_"+domain.DiagRootScopeIncomplete), HaveField("Side", "base"), HaveField("Revision", "base-sha"),
					HaveField("Message", HavePrefix("base revision base-sha project model coverage: "))),
			))
		})

		It("names the same side and revision in each text line", func() {
			text := codesignalcli.RenderText(report)

			Expect(text).To(MatchRegexp(`(?m)^path: package\.json, kind: project_root_scope_incomplete, message: head revision head-sha .*, side: head, revision: head-sha$`))
			Expect(text).To(MatchRegexp(`(?m)^path: package\.json, kind: base_project_root_scope_incomplete, message: base revision base-sha .*, side: base, revision: base-sha$`))
		})

		It("does not promote a routine reachability gap", func() {
			Expect(diagnosticsAtPath(report, "pkg/handlers/gap.ts")).To(BeEmpty())
		})
	})

	When("no base model was built", func() {
		It("does not promote base coverage a caller supplied anyway", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Scope:             codesignal.Scope{Revision: "head-sha"},
				ProjectCoverage:   &domain.Coverage{Phase: "full", Complete: false},
				HeadModelCoverage: incompleteModelCoverage("src/only-head.ts"),
				BaseModelCoverage: incompleteModelCoverage("src/only-base.ts"),
			})

			Expect(diagnosticsAtPath(report, "src/only-head.ts")).To(ConsistOf(HaveField("Side", "head")))
			Expect(diagnosticsAtPath(report, "src/only-base.ts")).To(BeEmpty())
		})
	})
})

// files_unanalyzed and files_with_diagnostics count changed files the
// file-level pipeline reported on. A project-model path -- a per-file
// unincorporated candidate, or an older analyzer's per-root fallback that
// names the root directory -- is not such a file.
var _ = Describe("File-level summary counters exclude project-model coverage paths", func() {
	When("head and base model coverage name a root directory and an unincorporated file, and one changed file was skipped", func() {
		It("counts only the skipped changed file while still reporting the model-coverage diagnostics", func() {
			report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
				Scope:               codesignal.Scope{Revision: "head-sha", Base: "base-sha"},
				Diagnostics:         []codesignal.Diagnostic{{Path: "assets/logo.bin", Kind: "unsupported_language", Message: "not analyzed"}},
				ProjectBaseAnalyzed: true,
				ProjectCoverage:     &domain.Coverage{Phase: "full", Complete: false},
				BaseProjectCoverage: &domain.Coverage{Phase: "full", Complete: false},
				HeadModelCoverage:   incompleteModelCoverage("src", "src/c.tsx"),
				BaseModelCoverage:   incompleteModelCoverage("src"),
			})

			Expect(diagnosticsAtPath(report, "src")).To(HaveLen(2))
			Expect(diagnosticsAtPath(report, "src/c.tsx")).To(HaveLen(1))
			Expect(report.Summary.FilesUnanalyzed).To(Equal(1), "only assets/logo.bin was a changed file left unanalyzed")
			Expect(report.Summary.FilesWithDiagnostics).To(Equal(1), "only assets/logo.bin is a changed file carrying a diagnostic")
		})
	})
})
