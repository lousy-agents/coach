package codesignalcli

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/render"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("project-analysis text rendering", func() {
	It("renders active project observations, structured paths, and project coverage", func() {
		report := &codesignal.Report{
			SchemaVersion: "2",
			Scope:         codesignal.Scope{AppliedScope: "all"},
			Summary:       codesignal.Summary{FilesAnalyzed: 1},
			ProjectChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "project.import_cycle",
				Lifecycle:   codesignal.Lifecycle("introduced"),
				Changed:     true,
				Evidence:    "pkg/a -> pkg/b -> pkg/a",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "pkg/a/a.go",
					Location: semantics.Location{StartRow: 2},
				},
				PathSteps: []codesignal.ProjectPathStep{{
					NodeID:      "package:pkg/a",
					DisplayName: "pkg/a",
					Resolution:  "resolved",
					Confidence:  codesignal.Confidence("high"),
					SourceLocations: []codesignal.ProjectLocation{{
						Path: "pkg/a/a.go", Location: semantics.Location{StartRow: 2},
					}},
				}},
			}},
			ProjectSummary:  &codesignal.ProjectSummary{ActiveChanges: 1, IntroducedChanges: 1},
			ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true, Counts: map[string]int{"packages": 2}},
		}

		text := render.ReportText(report)
		Expect(text).To(ContainSubstring("Project findings:"))
		Expect(text).To(ContainSubstring("semantic_key: cycle:pkg/a<->pkg/b"))
		Expect(text).To(ContainSubstring("path: pkg/a/a.go"))
		Expect(text).To(ContainSubstring("path step: package:pkg/a (pkg/a), resolution: resolved, confidence: high"))
		Expect(text).To(ContainSubstring("Project coverage: phase=full, complete=true"))
	})

	It("renders machine_evidence keys in sorted order under Project findings", func() {
		report := &codesignal.Report{
			SchemaVersion: "2",
			Scope:         codesignal.Scope{AppliedScope: "all"},
			ProjectChanges: []codesignal.ProjectChange{{
				SemanticKey: "layer:domain->infra",
				RuleID:      "architecture.layer_violation",
				Lifecycle:   codesignal.Lifecycle("baseline"),
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "pkg/domain/d.go",
					Location: semantics.Location{StartRow: 0},
				},
				MachineEvidence: map[string]string{
					"importer":  "pkg/domain",
					"edge_kind": "internal",
					"importee":  "pkg/infra",
				},
			}},
			ProjectSummary: &codesignal.ProjectSummary{ActiveChanges: 1, BaselineChanges: 1},
		}

		text := render.ReportText(report)
		edge := strings.Index(text, "machine_evidence.edge_kind: internal")
		importee := strings.Index(text, "machine_evidence.importee: pkg/infra")
		importer := strings.Index(text, "machine_evidence.importer: pkg/domain")
		Expect(edge).To(BeNumerically(">=", 0))
		Expect(importee).To(BeNumerically(">", edge))
		Expect(importer).To(BeNumerically(">", importee))
	})

	It("renders facts-only observations under Facts without counting them as project findings", func() {
		report := &codesignal.Report{
			SchemaVersion: "2",
			Scope:         codesignal.Scope{AppliedScope: "all"},
			Summary:       codesignal.Summary{},
			ProjectFacts: []codesignal.ProjectFact{{
				Kind:        "possible_call_reachability",
				SemanticKey: "reach:handler->query",
				Evidence:    "Handler may reach Query",
				PathSteps: []codesignal.ProjectPathStep{{
					NodeID:      "func:Handler",
					DisplayName: "Handler",
					Resolution:  "static",
					Confidence:  codesignal.Confidence("medium"),
				}},
				Provenance: codesignal.Provenance{Producer: "projectmodel"},
			}},
			ProjectSummary: &codesignal.ProjectSummary{},
		}

		text := render.ReportText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings."))
		Expect(text).To(ContainSubstring("Facts:"))
		Expect(text).To(ContainSubstring("kind: possible_call_reachability"))
		Expect(text).To(ContainSubstring("path step: func:Handler (Handler), resolution: static, confidence: medium"))
		Expect(text).NotTo(ContainSubstring("Project findings:"))
	})

	It("presents a Build-projected project observation once in text with structured fields", func() {
		builder, err := codesignal.New(codesignal.Options{ProjectEnabled: true, Baseline: true})
		Expect(err).NotTo(HaveOccurred())
		report, err := builder.Build(context.Background(), codesignal.Input{
			Files: []codesignal.FileChange{{
				Path:   "file_local.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "file_local.go",
					Language:    semantics.LanguageGo,
					ParseStatus: "ok",
					Findings: []semantics.Finding{{
						Kind:     "mutates_input",
						Name:     "Update",
						Location: semantics.Location{StartRow: 0, EndRow: 0},
						Evidence: "input.value = 1",
					}},
				},
			}},
			ProjectChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				Category:    "structure",
				Severity:    "medium",
				Confidence:  "high",
				Evidence:    "domain imports infrastructure",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "pkg/a/a.go",
					Location: semantics.Location{StartRow: 2},
				},
				RelatedLocations: []codesignal.ProjectLocation{{
					Path: "pkg/b/b.go", Location: semantics.Location{StartRow: 4},
				}},
				PathSteps: []codesignal.ProjectPathStep{{
					NodeID:      "package:pkg/a",
					DisplayName: "pkg/a",
					Resolution:  "resolved",
					Confidence:  codesignal.Confidence("high"),
				}},
				Provenance: codesignal.Provenance{Producer: "projectmodel"},
			}},
			ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(report.Signals).NotTo(BeEmpty(), "Build must still project anchored project observations onto signals for JSON")
		Expect(report.ProjectChanges).To(HaveLen(1))

		text := render.ReportText(report)
		Expect(text).To(ContainSubstring("path: file_local.go"), "file-local signals must still render")
		Expect(strings.Count(text, "path: pkg/a/a.go")).To(Equal(1), "project primary path must appear once; text=\n%s", text)
		Expect(strings.Count(text, "evidence: domain imports infrastructure")).To(Equal(1), "project evidence must appear once; text=\n%s", text)
		Expect(strings.Count(text, "lifecycle:")).To(Equal(2), "one file-local + one project lifecycle line; text=\n%s", text)
		Expect(text).To(ContainSubstring("Project findings:"))
		Expect(text).To(ContainSubstring("semantic_key: cycle:pkg/a<->pkg/b"))
		Expect(text).To(ContainSubstring("related: pkg/b/b.go:5"))
		Expect(text).To(ContainSubstring("path step: package:pkg/a (pkg/a), resolution: resolved, confidence: high"))
	})
})

var _ = Describe("project-analysis handoff into AnalyzeBaseline/AnalyzeChanges", func() {
	It("threads baseline project results into a schema-2 report and skips the seam when project is nil", func() {
		dir := gitfixture.Init(GinkgoT())
		sha := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []gitrepo.SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				Category:    "structure",
				Severity:    "medium",
				Confidence:  "high",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "full", Complete: true, Counts: map[string]int{"packages": 1}},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: projectconfig.Digest(cfg),
			Backend:      backend,
		}

		withProject, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(withProject.SchemaVersion).To(Equal("2"))
		Expect(withProject.ProjectChanges).To(HaveLen(1))
		Expect(withProject.ProjectCoverage).NotTo(BeNil())
		Expect(withProject.ProjectCoverage.Counts["packages"]).To(Equal(1))
		Expect(withProject.Signals).To(HaveLen(1))
		Expect(backend.requests).To(HaveLen(1))
		Expect(backend.requests[0].HeadRevision).To(Equal(sha))
		Expect(backend.requests[0].Baseline).To(BeTrue())
		Expect(backend.requests[0].ConfigDigest).To(Equal(project.ConfigDigest))

		backend.requests = nil
		withoutProject, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(withoutProject.SchemaVersion).To(Equal("1"))
		Expect(withoutProject.ProjectChanges).To(BeEmpty())
		Expect(backend.requests).To(BeEmpty(), "no-config runs must never invoke the project backend seam")
	})

	It("threads diff head/base project observations and config identity into the builder", func() {
		dir := gitfixture.Init(GinkgoT())
		baseSHA := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\nfunc A() {}\n")
		headSHA := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\nfunc A() { println(1) }\n")
		files := []gitrepo.SelectedFile{{Path: "a.go", Language: "go", Status: "modified"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			BaseChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			BaseCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			BaseAnalyzed: true,
		}}
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       json.RawMessage(`{"schema_version":"1","roots":["."]}`),
			ConfigDigest: "pcfg_test",
			Backend:      backend,
		}

		report, err := AnalyzeChanges(context.Background(), dir, headSHA, baseSHA, files, nil, "all", nil, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.SchemaVersion).To(Equal("2"))
		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		Expect(backend.requests).To(HaveLen(1))
		Expect(backend.requests[0].HeadRevision).To(Equal(headSHA))
		Expect(backend.requests[0].BaseRevision).To(Equal(baseSHA))
		Expect(backend.requests[0].ConfigDigest).To(Equal("pcfg_test"))
		Expect(backend.requests[0].Baseline).To(BeFalse())
	})

	It("threads an incomplete HeadCoverage into a lifecycle-indeterminate report rather than claiming baseline", func() {
		dir := gitfixture.Init(GinkgoT())
		sha := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []gitrepo.SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				Category:    "structure",
				Severity:    "medium",
				Confidence:  "high",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: false},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: projectconfig.Digest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")), "an incomplete HeadCoverage must never be reported as lifecycle baseline")
		Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
		Expect(report.ProjectCoverage).NotTo(BeNil(), "HeadCoverage must reach the report, not be dropped as nil")
		Expect(report.ProjectCoverage.Complete).To(BeFalse())
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")), "the CLI-facing report must surface the same indeterminate diagnostic codesignal.Build contracts for partial coverage")
	})

	It("renders the no-active-findings verdict as project-incomplete, not path-skipped, for a real incomplete-coverage report", func() {
		dir := gitfixture.Init(GinkgoT())
		sha := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []gitrepo.SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: false},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: projectconfig.Digest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(BeEmpty(), "no project changes were returned by the backend")
		Expect(report.Signals).To(BeEmpty(), "the fixture file triggers no file-local finding")
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))

		text := render.ReportText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
		Expect(text).To(ContainSubstring("project analysis did not complete"))
		Expect(text).NotTo(ContainSubstring("not analyzed"), "project_coverage_incomplete/project_lifecycle_indeterminate diagnostics carry no Path, so no path count must be claimed")
	})

	It("renders project-incomplete for a real report whose base-side coverage was incomplete even though head coverage is complete", func() {
		dir := gitfixture.Init(GinkgoT())
		baseSHA := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\nfunc A() {}\n")
		headSHA := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\n// note\nfunc A() {}\n")
		files := []gitrepo.SelectedFile{{Path: "a.go", Language: "go", Status: "modified"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: true},
			BaseCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: false},
			BaseAnalyzed: true,
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: projectconfig.Digest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeChanges(context.Background(), dir, headSHA, baseSHA, files, nil, "all", nil, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(BeEmpty(), "the backend returned no observations on either side")
		Expect(report.Signals).To(BeEmpty(), "a comment-only change triggers no file-local finding")
		Expect(report.ProjectCoverage.Complete).To(BeTrue(), "head coverage alone is complete")
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")), "base-side incompleteness must still surface as a diagnostic")

		text := render.ReportText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
		Expect(text).To(ContainSubstring("project analysis did not complete"), "the verdict must not fall back to the generic 'additional diagnostics were recorded' clause when the diagnostic itself names project-analysis incompleteness")
	})

	It("renders the generic incomplete-analysis fallback for a real report whose only diagnostic is an anchorless project observation", func() {
		dir := gitfixture.Init(GinkgoT())
		sha := gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []gitrepo.SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				Kind:        "project_layer_violation",
				Provenance:  codesignal.Provenance{Producer: "fake-backend"},
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: projectconfig.Digest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(BeEmpty(), "the anchorless observation must be dropped, not rendered as an active finding")
		Expect(report.Signals).To(BeEmpty(), "the fixture file triggers no file-local finding")
		Expect(report.ProjectCoverage.Complete).To(BeTrue())
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_observation_missing_primary_path")))

		text := render.ReportText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete: additional diagnostics were recorded."))
	})
})

var _ = Describe("applyProjectBackend analyzer protocol version handoff", func() {
	It("defaults AnalyzerProtocolVersion to 1 for typescript when backend returns a result without a protocol version", func() {
		backend := identityHandoffBackend{result: &ProjectBackendResult{}}
		input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, &ProjectAnalysis{
			Backend:  backend,
			Language: "typescript",
		}, ".", "HEAD", "", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(input.AnalyzerProtocolVersion).To(Equal(1), "applyProjectBackend must default typescript AnalyzerProtocolVersion to 1 when backend returns 0")
	})

	It("copies a non-zero AnalyzerProtocolVersion from ProjectBackendResult onto codesignal.Input", func() {
		backend := identityHandoffBackend{result: &ProjectBackendResult{
			AnalyzerProtocolVersion: 1,
		}}
		input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, &ProjectAnalysis{
			Backend:  backend,
			Language: "typescript",
		}, ".", "HEAD", "", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(input.AnalyzerProtocolVersion).To(Equal(1))
	})

	It("does not default AnalyzerProtocolVersion for non-typescript language", func() {
		backend := identityHandoffBackend{result: &ProjectBackendResult{}}
		input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, &ProjectAnalysis{
			Backend:  backend,
			Language: "go",
		}, ".", "HEAD", "", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(input.AnalyzerProtocolVersion).To(Equal(0), "non-typescript backends must not get the typescript default")
	})
})

var _ = Describe("applyProjectBackend runtime identity handoff", func() {
	It("copies runtime kind, version, origin, compiler version, and compiler origin onto codesignal.Input", func() {
		backend := identityHandoffBackend{result: &ProjectBackendResult{
			RuntimeKind:     runtimeKindNode,
			RuntimeVersion:  "v24.9.9",
			RuntimeOrigin:   runtimeOriginPath,
			CompilerVersion: "7.0.2",
			CompilerOrigin:  tstoolchain.OriginProject,
		}}
		input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, &ProjectAnalysis{
			Backend: backend,
		}, ".", "HEAD", "", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(input.RuntimeKind).To(Equal(runtimeKindNode), "runtime identity must reach codesignal.Input, not stop on ProjectBackendResult")
		Expect(input.RuntimeVersion).To(Equal("v24.9.9"))
		Expect(input.RuntimeOrigin).To(Equal(runtimeOriginPath))
		Expect(input.CompilerVersion).To(Equal("7.0.2"))
		Expect(input.CompilerOrigin).To(Equal(tstoolchain.OriginProject))
	})

	It("copies all six phase coverages, project scopes, language, config digest, and selected roots onto codesignal.Input", func() {
		headScope := &projectmodel.ProjectScope{
			InclusionRule:   projectmodel.InclusionRuleTSConfigIncludesNoTestClassification,
			PatternSet:      projectmodel.TSReachabilityAlgorithm,
			Roots:           []projectmodel.ProjectScopeRoot{{Root: "src", CandidateFiles: 5, AnalyzedFiles: 5}},
			MatchedLayers:   []string{"api"},
			UnmatchedLayers: []string{"store"},
		}
		baseScope := &projectmodel.ProjectScope{
			InclusionRule:   projectmodel.InclusionRuleTSConfigIncludesNoTestClassification,
			PatternSet:      projectmodel.TSReachabilityAlgorithm,
			Roots:           []projectmodel.ProjectScopeRoot{{Root: "src", CandidateFiles: 4, AnalyzedFiles: 4}},
			MatchedLayers:   []string{"api"},
			UnmatchedLayers: []string{},
		}
		headModel := &projectmodel.Coverage{Phase: "model", Complete: true}
		baseModel := &projectmodel.Coverage{Phase: "model", Complete: true}
		headBypass := &projectmodel.Coverage{Phase: "bypass", Complete: true}
		baseBypass := &projectmodel.Coverage{Phase: "not_requested", Complete: true}
		headReach := &projectmodel.Coverage{Phase: "reachability", Complete: true}
		baseReach := &projectmodel.Coverage{Phase: "reachability", Complete: false}

		configJSON := json.RawMessage(`{"schema_version":"1","roots":["src"]}`)
		digest := projectconfig.Digest(configJSON)

		backend := identityHandoffBackend{result: &ProjectBackendResult{
			HeadProjectScope:         headScope,
			BaseProjectScope:         baseScope,
			HeadModelCoverage:        headModel,
			BaseModelCoverage:        baseModel,
			HeadBypassCoverage:       headBypass,
			BaseBypassCoverage:       baseBypass,
			HeadReachabilityCoverage: headReach,
			BaseReachabilityCoverage: baseReach,
		}}
		input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, &ProjectAnalysis{
			Backend:      backend,
			Language:     "typescript",
			Config:       configJSON,
			ConfigDigest: digest,
		}, ".", "HEAD", "BASE", false)

		Expect(err).NotTo(HaveOccurred())
		Expect(input.Language).To(Equal("typescript"), "Language must reach codesignal.Input from ProjectAnalysis")
		Expect(input.ConfigDigest).To(Equal(digest), "ConfigDigest must reach codesignal.Input from ProjectAnalysis")
		Expect(input.SelectedRoots).To(Equal([]string{"src"}), "SelectedRoots must be decoded from Config.roots and reach codesignal.Input")
		Expect(input.HeadProjectScope).To(Equal(headScope), "HeadProjectScope must reach codesignal.Input from ProjectBackendResult")
		Expect(input.BaseProjectScope).To(Equal(baseScope), "BaseProjectScope must reach codesignal.Input from ProjectBackendResult")
		Expect(input.HeadModelCoverage).To(Equal(headModel), "HeadModelCoverage must reach codesignal.Input from ProjectBackendResult")
		Expect(input.BaseModelCoverage).To(Equal(baseModel), "BaseModelCoverage must reach codesignal.Input from ProjectBackendResult")
		Expect(input.HeadBypassCoverage).To(Equal(headBypass), "HeadBypassCoverage must reach codesignal.Input from ProjectBackendResult")
		Expect(input.BaseBypassCoverage).To(Equal(baseBypass), "BaseBypassCoverage must reach codesignal.Input from ProjectBackendResult")
		Expect(input.HeadReachabilityCoverage).To(Equal(headReach), "HeadReachabilityCoverage must reach codesignal.Input from ProjectBackendResult")
		Expect(input.BaseReachabilityCoverage).To(Equal(baseReach), "BaseReachabilityCoverage must reach codesignal.Input from ProjectBackendResult")
	})
})
