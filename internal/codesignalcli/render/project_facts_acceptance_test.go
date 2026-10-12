package render

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// AC-VER-5/AC-VER-6 (#334, epic #280): a reachability fact describes the
// current HEAD only, so its appearance or disappearance must never be
// rendered as an improvement or with ProjectChange lifecycle wording.
var _ = Describe("ReportText reachability facts", func() {
	It("changes only the fact's own block when a fact appears or disappears, even alongside a genuinely resolved project change", func() {
		fact := codesignal.ProjectFact{
			Kind:        "possible_call_reachability",
			SemanticKey: "reach:handler->query",
			Evidence:    "Handler may reach Query",
			Provenance:  codesignal.Provenance{Producer: "projectmodel"},
		}
		// The resolved change is present in both reports, so lifecycle wording
		// that legitimately belongs to it can be told apart from wording that
		// leaks onto the fact's own presence or absence.
		withoutFact := codesignal.Input{
			BaseProjectChanges: []codesignal.ProjectChange{{
				SemanticKey:   "cycle:pkg/a<->pkg/b",
				RuleID:        "project.import_cycle",
				RuleVersion:   "1",
				Kind:          "cycle",
				Category:      codesignal.Category("architecture"),
				Severity:      codesignal.Severity("medium"),
				Confidence:    codesignal.Confidence("high"),
				PrimaryAnchor: codesignal.ProjectLocation{Path: "pkg/a/a.go", Location: semantics.Location{StartRow: 1}},
				Evidence:      "pkg/a -> pkg/b -> pkg/a",
				Provenance:    codesignal.Provenance{Producer: "projectmodel"},
			}},
			ProjectBaseAnalyzed: true,
			ProjectCoverage:     &domain.Coverage{Phase: "full", Complete: true},
			BaseProjectCoverage: &domain.Coverage{Phase: "full", Complete: true},
		}
		withFact := withoutFact
		withFact.ProjectFacts = []codesignal.ProjectFact{fact}
		options := codesignal.Options{ProjectEnabled: true, IncludeResolved: true}

		reportWithFact := buildReport(options, withFact)
		reportWithoutFact := buildReport(options, withoutFact)
		Expect(reportWithFact.ProjectChanges).To(ConsistOf(HaveField("Lifecycle", codesignal.Lifecycle("resolved"))),
			"fixture must exercise a real resolved ProjectChange alongside the fact")

		textWithFact := ReportText(reportWithFact)
		textWithoutFact := ReportText(reportWithoutFact)
		factBlock := "\nFacts:\nkind: possible_call_reachability\nsemantic_key: reach:handler->query\nevidence: Handler may reach Query\n"
		Expect(textWithFact).To(ContainSubstring(factBlock))
		Expect(textWithoutFact).NotTo(ContainSubstring("Facts:"))
		Expect(strings.Replace(textWithFact, factBlock, "", 1)).To(Equal(textWithoutFact),
			"a fact's presence or absence must change only its own rendered block, never any other wording in the report")
		for _, word := range []string{"no longer reachable", "improved", "fixed"} {
			Expect(strings.ToLower(textWithFact)).NotTo(ContainSubstring(word))
			Expect(strings.ToLower(textWithoutFact)).NotTo(ContainSubstring(word))
		}
	})
})
