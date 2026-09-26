package codesignalcli

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("RenderText project scope, provenance, and next-actions sections", func() {
	When("Report carries ProjectScope, ProjectProvenance, and ProjectNextActions", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			headSHA := "abc1234def5678ab"
			report = &codesignal.Report{
				SchemaVersion: "2",
				Scope: codesignal.Scope{
					AppliedScope: "all",
					Revision:     headSHA,
				},
				Summary: codesignal.Summary{FilesAnalyzed: 10},
				ProjectScope: &codesignal.ProjectScopeReport{
					InclusionRule: projectmodel.InclusionRuleTSConfigIncludesNoTestClassification,
					PatternSet:    "ts-source-sink-registry@1",
					Head: codesignal.ProjectScopeRevisionReport{
						Revision:        headSHA,
						Roots:           []projectmodel.ProjectScopeRoot{{Root: ".", CandidateFiles: 10, AnalyzedFiles: 10}},
						MatchedLayers:   []string{"handlers"},
						UnmatchedLayers: []string{"db"},
					},
				},
				ProjectProvenance: &codesignal.ProjectProvenance{
					Language:      "typescript",
					ConfigDigest:  "pcfg_abc123",
					SelectedRoots: []string{"."},
					Analyzer: codesignal.ProvenanceAnalyzer{
						Version:         "v0.1.0",
						Digest:          "sha256:1234",
						ProtocolVersion: 1,
					},
					Runtime: codesignal.ProvenanceRuntime{
						Kind:            "node",
						Version:         "v24.0.0",
						Origin:          "path",
						CompilerVersion: "5.4.0",
						CompilerOrigin:  "project",
					},
					Head: codesignal.ProvenanceRevision{
						Revision: headSHA,
						Coverage: codesignal.ProvenanceCoverage{
							Model:        "complete",
							Bypass:       "not_requested",
							Reachability: "complete",
						},
					},
				},
				ProjectNextActions: []codesignal.ProjectNextAction{
					{Kind: "record_baseline"},
					{Kind: "review_policy_coverage"},
				},
			}
		})

		It("renders the project_scope block BEFORE any findings section", func() {
			text := RenderText(report)

			scopeIdx := strings.Index(text, "Project scope:")
			noFindingsIdx := strings.Index(text, "No active CodeSignal findings")

			Expect(scopeIdx).To(BeNumerically(">=", 0), "Project scope: block must be present; text=\n%s", text)
			if noFindingsIdx >= 0 {
				Expect(scopeIdx).To(BeNumerically("<", noFindingsIdx), "Project scope: must precede no-findings verdict")
			}
			Expect(text).To(ContainSubstring("pattern_set: ts-source-sink-registry@1"))
			Expect(text).To(ContainSubstring("matched_layers: handlers"))
			Expect(text).To(ContainSubstring("unmatched_layers: db"))
		})

		It("renders project_scope BEFORE project findings when ProjectChanges are present", func() {
			reportWithFindings := *report
			reportWithFindings.ProjectChanges = []codesignal.ProjectChange{
				{SemanticKey: "test/key", RuleID: "test-rule"},
			}
			text := RenderText(&reportWithFindings)
			scopeIdx := strings.Index(text, "Project scope:")
			findingsIdx := strings.Index(text, "Project findings:")
			Expect(scopeIdx).To(BeNumerically(">=", 0))
			Expect(findingsIdx).To(BeNumerically(">=", 0))
			Expect(scopeIdx).To(BeNumerically("<", findingsIdx), "Project scope: must precede Project findings:")
		})

		It("includes a reference to #281 in the inclusion_rule description", func() {
			text := RenderText(report)
			Expect(text).To(ContainSubstring("#281"))
		})

		It("renders the complete_no_match sentence that does not imply compliance", func() {
			text := RenderText(report)
			Expect(text).To(ContainSubstring("Complete scan found no configured covered match."))
			Expect(text).NotTo(ContainSubstring("No active CodeSignal findings."))
			Expect(text).NotTo(ContainSubstring("compliant"))
			Expect(text).NotTo(ContainSubstring("clean architecture"))
			Expect(text).NotTo(ContainSubstring("no problems"))
		})

		It("renders record_baseline and review_policy_coverage next actions", func() {
			text := RenderText(report)
			Expect(text).To(ContainSubstring("record_baseline: retain the reported HEAD revision for a future `--base <revision>` comparison; Coach persists nothing"))
			Expect(text).To(ContainSubstring("review_policy_coverage: inspect selected roots and matched/unmatched layers against intent; revise the committed policy if they do not express it"))
		})

		It("renders a project_provenance text section", func() {
			text := RenderText(report)
			Expect(text).To(ContainSubstring("Project provenance:"))
			Expect(text).To(ContainSubstring("language: typescript"))
			Expect(text).To(ContainSubstring("config_digest: pcfg_abc123"))
			Expect(text).To(ContainSubstring("analyzer: v0.1.0 sha256:1234 (protocol 1)"))
			Expect(text).To(ContainSubstring("runtime: node v24.0.0 (path)"))
			Expect(text).To(ContainSubstring("compiler: 5.4.0 (project)"))
		})

		It("does not include absolute paths like /tmp or /home in provenance text", func() {
			text := RenderText(report)
			Expect(text).NotTo(ContainSubstring("/tmp"))
			Expect(text).NotTo(ContainSubstring("/home"))
			Expect(text).To(ContainSubstring("root: .  candidate_files: 10  analyzed_files: 10"))
			Expect(text).To(ContainSubstring("selected_roots: ."))
		})

		It("renders inspect_diagnostics only when that kind is in ProjectNextActions", func() {
			text := RenderText(report)
			Expect(text).NotTo(ContainSubstring("inspect_diagnostics"),
				"inspect_diagnostics must not appear when not in ProjectNextActions; text=\n%s", text)

			reportWithDiag := *report
			reportWithDiag.ProjectNextActions = []codesignal.ProjectNextAction{
				{Kind: "record_baseline"},
				{Kind: "review_policy_coverage"},
				{Kind: "inspect_diagnostics"},
			}
			textWithDiag := RenderText(&reportWithDiag)
			Expect(textWithDiag).To(ContainSubstring("inspect_diagnostics: resolve or explicitly accept reported limitations before trusting the absence of findings"))
		})

		It("renders next actions in the slice's declared order", func() {
			text := RenderText(report)
			rbIdx := strings.Index(text, "record_baseline")
			rpcIdx := strings.Index(text, "review_policy_coverage")
			Expect(rbIdx).To(BeNumerically(">=", 0))
			Expect(rpcIdx).To(BeNumerically(">=", 0))
			Expect(rbIdx).To(BeNumerically("<", rpcIdx), "record_baseline must appear before review_policy_coverage in slice order")
		})
	})

	When("Report has incomplete ProjectCoverage but no ProjectNextActions (Go-style incomplete)", func() {
		It("uses the existing incomplete verdict, not the complete_no_match sentence", func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				Scope:         codesignal.Scope{AppliedScope: "all"},
				ProjectCoverage: &projectmodel.Coverage{
					Phase:    "go_model_build",
					Complete: false,
				},
				Diagnostics: []codesignal.Diagnostic{
					{Kind: codesignal.DiagKindProjectCoverageIncomplete, Message: "incomplete"},
				},
			}

			text := RenderText(report)
			Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
			Expect(text).NotTo(ContainSubstring("Complete scan found no configured covered match."))
		})
	})

	When("Report has nil ProjectProvenance, ProjectScope, and ProjectNextActions (Go reports)", func() {
		It("does not mention #281 or Project scope:", func() {
			report := &codesignal.Report{
				SchemaVersion: "1",
				Scope:         codesignal.Scope{AppliedScope: "all"},
				Summary:       codesignal.Summary{FilesAnalyzed: 5},
			}

			text := RenderText(report)
			Expect(text).NotTo(ContainSubstring("#281"))
			Expect(text).NotTo(ContainSubstring("Project scope:"))
			Expect(text).NotTo(ContainSubstring("Project provenance:"))
		})
	})

	When("Report has a base revision in ProjectScope (diff mode)", func() {
		It("renders both head and base revision blocks", func() {
			headSHA := "headsha1234"
			baseSHA := "basesha5678"
			report := &codesignal.Report{
				SchemaVersion: "2",
				Scope: codesignal.Scope{
					AppliedScope: "all",
					Revision:     headSHA,
					Base:         baseSHA,
				},
				Summary: codesignal.Summary{FilesAnalyzed: 5},
				ProjectScope: &codesignal.ProjectScopeReport{
					InclusionRule: projectmodel.InclusionRuleTSConfigIncludesNoTestClassification,
					PatternSet:    "ts-source-sink-registry@1",
					Head: codesignal.ProjectScopeRevisionReport{
						Revision:        headSHA,
						Roots:           []projectmodel.ProjectScopeRoot{{Root: ".", CandidateFiles: 10, AnalyzedFiles: 10}},
						MatchedLayers:   []string{"handlers"},
						UnmatchedLayers: []string{"db"},
					},
					Base: &codesignal.ProjectScopeRevisionReport{
						Revision:        baseSHA,
						Roots:           []projectmodel.ProjectScopeRoot{{Root: ".", CandidateFiles: 9, AnalyzedFiles: 9}},
						MatchedLayers:   []string{"handlers"},
						UnmatchedLayers: []string{"db"},
					},
				},
				ProjectNextActions: []codesignal.ProjectNextAction{
					{Kind: "review_policy_coverage"},
				},
			}

			text := RenderText(report)
			Expect(text).To(ContainSubstring("head_revision: " + headSHA))
			Expect(text).To(ContainSubstring("base_revision: " + baseSHA))
			Expect(text).To(ContainSubstring("candidate_files: 9"))
		})
	})

	When("Report has FilesUnanalyzed > 0 alongside ProjectNextActions", func() {
		It("includes the unanalyzed-paths clause in the complete-no-match sentence", func() {
			report := &codesignal.Report{
				SchemaVersion: "2",
				Scope:         codesignal.Scope{AppliedScope: "all"},
				Summary:       codesignal.Summary{FilesAnalyzed: 5, FilesUnanalyzed: 2},
				ProjectNextActions: []codesignal.ProjectNextAction{
					{Kind: "inspect_diagnostics"},
				},
			}
			text := RenderText(report)
			Expect(text).To(ContainSubstring("Complete scan found no configured covered match, but 2 paths were not analyzed."))
			Expect(text).NotTo(ContainSubstring("No active CodeSignal findings."))
		})
	})
})
