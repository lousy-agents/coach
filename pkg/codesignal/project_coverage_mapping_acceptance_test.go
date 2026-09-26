package codesignal_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// headProjectScope returns a minimal non-nil ProjectScope for use in tests.
func headProjectScope() *projectmodel.ProjectScope {
	return &projectmodel.ProjectScope{
		InclusionRule:   projectmodel.InclusionRuleTSConfigIncludesNoTestClassification,
		PatternSet:      projectmodel.TSReachabilityAlgorithm,
		Roots:           []projectmodel.ProjectScopeRoot{{Root: ".", CandidateFiles: 10, AnalyzedFiles: 10}},
		MatchedLayers:   []string{"handlers"},
		UnmatchedLayers: []string{"db"},
	}
}

// baseProjectScope returns a distinct ProjectScope for the base revision in diff-mode tests.
// Its candidate_files (7) and layers differ from headProjectScope's so base/head conflation
// is detectable without asserting pointer equality.
func baseProjectScope() *projectmodel.ProjectScope {
	return &projectmodel.ProjectScope{
		InclusionRule:   projectmodel.InclusionRuleTSConfigIncludesNoTestClassification,
		PatternSet:      projectmodel.TSReachabilityAlgorithm,
		Roots:           []projectmodel.ProjectScopeRoot{{Root: ".", CandidateFiles: 7, AnalyzedFiles: 5}},
		MatchedLayers:   []string{"api"},
		UnmatchedLayers: []string{"cache", "db"},
	}
}

// completeCoverage returns a complete Coverage for a given phase name.
func completeCoverage(phase string) *projectmodel.Coverage {
	return &projectmodel.Coverage{Phase: phase, Complete: true}
}

// incompleteCoverage returns an incomplete Coverage for a given phase name.
func incompleteCoverage(phase string) *projectmodel.Coverage {
	return &projectmodel.Coverage{Phase: phase, Complete: false}
}

// notRequestedCoverage returns a Coverage with Phase="not_requested" and Complete=true,
// as the TypeScript backend produces when no required_layer is configured.
func notRequestedCoverage() *projectmodel.Coverage {
	return &projectmodel.Coverage{Phase: "not_requested", Complete: true}
}

// backendUnavailableCoverage returns a Coverage containing DiagBackendUnavailable,
// as produced when the TS sidecar cannot be reached.
func backendUnavailableCoverage() *projectmodel.Coverage {
	return &projectmodel.Coverage{
		Phase:    "model",
		Complete: false,
		Diagnostics: []projectmodel.Diagnostic{
			{Code: projectmodel.DiagBackendUnavailable, Message: "sidecar unavailable"},
		},
	}
}

// extractProvenance unmarshals the project_provenance key from a report's JSON.
func extractProvenance(report *codesignal.Report) map[string]json.RawMessage {
	fields := rawReportFields(report)
	if _, ok := fields["project_provenance"]; !ok {
		return nil
	}
	var prov map[string]json.RawMessage
	Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
	return prov
}

// extractHeadCoverage reads project_provenance.head.coverage from a report.
func extractHeadCoverage(report *codesignal.Report) map[string]json.RawMessage {
	prov := extractProvenance(report)
	Expect(prov).NotTo(BeNil(), "project_provenance must be present")
	var head map[string]json.RawMessage
	Expect(json.Unmarshal(prov["head"], &head)).To(Succeed())
	var cov map[string]json.RawMessage
	Expect(json.Unmarshal(head["coverage"], &cov)).To(Succeed())
	return cov
}

// nextActionKinds returns the kind strings from project_next_actions.
func nextActionKinds(report *codesignal.Report) []string {
	fields := rawReportFields(report)
	raw, ok := fields["project_next_actions"]
	if !ok {
		return nil
	}
	var actions []map[string]json.RawMessage
	Expect(json.Unmarshal(raw, &actions)).To(Succeed())
	kinds := make([]string, 0, len(actions))
	for _, a := range actions {
		var kind string
		Expect(json.Unmarshal(a["kind"], &kind)).To(Succeed())
		kinds = append(kinds, kind)
	}
	return kinds
}

// layerViolationChange returns a ProjectChange with the given rule ID
// (architecture.layer_violation or architecture.layer_bypass) that has a
// valid anchor so it reaches the active report set.
func layerViolationChange(ruleID string) codesignal.ProjectChange {
	return projectChange("layer:domain->infra", ruleID)
}

var _ = Describe("Project coverage mapping, provenance, scope, and next-actions (AC-EVD-2/6/9/10, AC-D7)", func() {

	// AC-1: TypeScript baseline with complete model, bypass not_requested,
	// and incomplete reachability. Reachability incompleteness must not
	// block complete_no_match.
	When("a TypeScript baseline report has complete model, bypass=not_requested, and incomplete reachability", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			report = build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:                 "typescript",
				ConfigDigest:             "pcfg_abc",
				SelectedRoots:            []string{"."},
				Scope:                    codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:         headProjectScope(),
				HeadModelCoverage:        completeCoverage("model"),
				HeadBypassCoverage:       notRequestedCoverage(),
				HeadReachabilityCoverage: incompleteCoverage("reachability"),
				ProjectCoverage:          &projectmodel.Coverage{Phase: "full", Complete: true},
				RuntimeKind:              "node",
				RuntimeVersion:           "v24.9.9",
				RuntimeOrigin:            "path",
				CompilerVersion:          "7.0.2",
				CompilerOrigin:           "project",
			})
		})

		It("maps model=complete, bypass=not_requested, reachability=incomplete in project_provenance.head.coverage", func() {
			cov := extractHeadCoverage(report)
			var model, bypass, reachability string
			Expect(json.Unmarshal(cov["model"], &model)).To(Succeed())
			Expect(json.Unmarshal(cov["bypass"], &bypass)).To(Succeed())
			Expect(json.Unmarshal(cov["reachability"], &reachability)).To(Succeed())
			Expect(model).To(Equal("complete"))
			Expect(bypass).To(Equal("not_requested"))
			Expect(reachability).To(Equal("incomplete"))
		})

		It("emits project_provenance with language=typescript, config_digest, selected_roots, and decoded runtime field values", func() {
			prov := extractProvenance(report)
			Expect(prov).NotTo(BeNil())
			var language, configDigest string
			Expect(json.Unmarshal(prov["language"], &language)).To(Succeed())
			Expect(language).To(Equal("typescript"))
			Expect(json.Unmarshal(prov["config_digest"], &configDigest)).To(Succeed())
			Expect(configDigest).To(Equal("pcfg_abc"))
			var selectedRoots []string
			Expect(json.Unmarshal(prov["selected_roots"], &selectedRoots)).To(Succeed())
			Expect(selectedRoots).To(Equal([]string{"."}))
			var runtime map[string]json.RawMessage
			Expect(json.Unmarshal(prov["runtime"], &runtime)).To(Succeed())
			var kind, version, origin, compilerVersion, compilerOrigin string
			Expect(json.Unmarshal(runtime["kind"], &kind)).To(Succeed())
			Expect(kind).To(Equal("node"))
			Expect(json.Unmarshal(runtime["version"], &version)).To(Succeed())
			Expect(version).To(Equal("v24.9.9"))
			Expect(json.Unmarshal(runtime["origin"], &origin)).To(Succeed())
			Expect(origin).To(Equal("path"))
			Expect(json.Unmarshal(runtime["compiler_version"], &compilerVersion)).To(Succeed())
			Expect(compilerVersion).To(Equal("7.0.2"))
			Expect(json.Unmarshal(runtime["compiler_origin"], &compilerOrigin)).To(Succeed())
			Expect(compilerOrigin).To(Equal("project"))
		})

		It("emits project_scope.head because HeadProjectScope is non-nil", func() {
			fields := rawReportFields(report)
			Expect(fields).To(HaveKey("project_scope"))
			var scope map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_scope"], &scope)).To(Succeed())
			Expect(scope).To(HaveKey("head"))
		})

		It("emits project_next_actions with [record_baseline, review_policy_coverage] and no inspect_diagnostics", func() {
			kinds := nextActionKinds(report)
			Expect(kinds).To(Equal([]string{"record_baseline", "review_policy_coverage"}))
		})
	})

	// AC-2: incomplete model phase blocks complete_no_match and omits next_actions,
	// but project_scope still appears because HeadProjectScope is non-nil.
	When("the model phase is incomplete", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			report = build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:                 "typescript",
				Scope:                    codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:         headProjectScope(),
				HeadModelCoverage:        incompleteCoverage("model"),
				HeadBypassCoverage:       notRequestedCoverage(),
				HeadReachabilityCoverage: completeCoverage("reachability"),
				ProjectCoverage:          &projectmodel.Coverage{Phase: "full", Complete: false},
			})
		})

		It("maps model=incomplete in project_provenance.head.coverage", func() {
			cov := extractHeadCoverage(report)
			var model string
			Expect(json.Unmarshal(cov["model"], &model)).To(Succeed())
			Expect(model).To(Equal("incomplete"))
		})

		It("omits project_next_actions because required coverage is not complete", func() {
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_next_actions"))
		})

		It("still emits project_scope because HeadProjectScope is non-nil", func() {
			fields := rawReportFields(report)
			Expect(fields).To(HaveKey("project_scope"))
		})
	})

	// AC-3: HeadProjectScope nil AND HeadModelCoverage with DiagBackendUnavailable
	// → model="not_run", no project_scope, no project_next_actions.
	When("HeadProjectScope is nil and model coverage carries DiagBackendUnavailable", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			report = build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:          "typescript",
				Scope:             codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:  nil,
				HeadModelCoverage: backendUnavailableCoverage(),
			})
		})

		It("maps model=not_run in project_provenance.head.coverage", func() {
			cov := extractHeadCoverage(report)
			var model string
			Expect(json.Unmarshal(cov["model"], &model)).To(Succeed())
			Expect(model).To(Equal("not_run"))
		})

		It("omits project_scope because HeadProjectScope is nil", func() {
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_scope"))
		})

		It("omits project_next_actions because required coverage is not complete", func() {
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_next_actions"))
		})
	})

	// AC-D7 conjunction pins: "not_run" requires BOTH scopeNil AND DiagBackendUnavailable.
	// Removing either operand from mapCoveragePhase must make these specs fail.
	When("HeadProjectScope is nil but model coverage carries no DiagBackendUnavailable", func() {
		It("maps model=incomplete, not not_run, because the diagnostic operand is absent", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:          "typescript",
				Scope:             codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:  nil,
				HeadModelCoverage: incompleteCoverage("model"),
			})
			cov := extractHeadCoverage(report)
			var model string
			Expect(json.Unmarshal(cov["model"], &model)).To(Succeed())
			Expect(model).To(Equal("incomplete"))
		})
	})

	When("HeadProjectScope is non-nil but model coverage carries DiagBackendUnavailable", func() {
		It("maps model=incomplete, not not_run, because the scopeNil operand is absent", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:          "typescript",
				Scope:             codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:  headProjectScope(),
				HeadModelCoverage: backendUnavailableCoverage(),
			})
			cov := extractHeadCoverage(report)
			var model string
			Expect(json.Unmarshal(cov["model"], &model)).To(Succeed())
			Expect(model).To(Equal("incomplete"))
		})
	})

	// AC-4: bypass Coverage with Phase="not_requested" maps to "not_requested"
	// even when Complete=true.
	When("HeadBypassCoverage has Phase=not_requested with Complete=true", func() {
		It("maps bypass=not_requested regardless of Complete value", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:           "typescript",
				Scope:              codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:   headProjectScope(),
				HeadModelCoverage:  completeCoverage("model"),
				HeadBypassCoverage: &projectmodel.Coverage{Phase: "not_requested", Complete: true},
				ProjectCoverage:    &projectmodel.Coverage{Phase: "full", Complete: true},
			})
			cov := extractHeadCoverage(report)
			var bypass string
			Expect(json.Unmarshal(cov["bypass"], &bypass)).To(Succeed())
			Expect(bypass).To(Equal("not_requested"))
		})
	})

	// AC-5: diff mode, complete both sides, zero introduced/resolved/existing changes
	// → next_actions=[review_policy_coverage] only (no record_baseline).
	When("diff mode with complete coverage on both sides and zero project changes", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			report = build(codesignal.Options{ProjectEnabled: true, Baseline: false}, codesignal.Input{
				Language:                 "typescript",
				Scope:                    codesignal.Scope{Revision: "HEAD_SHA", Base: "BASE_SHA"},
				ProjectBaseAnalyzed:      true,
				HeadProjectScope:         headProjectScope(),
				BaseProjectScope:         baseProjectScope(),
				HeadModelCoverage:        completeCoverage("model"),
				HeadBypassCoverage:       notRequestedCoverage(),
				HeadReachabilityCoverage: completeCoverage("reachability"),
				BaseModelCoverage:        completeCoverage("model"),
				BaseBypassCoverage:       notRequestedCoverage(),
				BaseReachabilityCoverage: incompleteCoverage("reachability"),
				ProjectCoverage:          &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage:      &projectmodel.Coverage{Phase: "full", Complete: true},
			})
		})

		It("emits project_next_actions with [review_policy_coverage] only", func() {
			kinds := nextActionKinds(report)
			Expect(kinds).To(Equal([]string{"review_policy_coverage"}))
		})

		It("emits project_provenance.base with revision and coverage", func() {
			prov := extractProvenance(report)
			Expect(prov).NotTo(BeNil())
			Expect(prov).To(HaveKey("base"))
		})

		It("emits project_scope with inclusion_rule, pattern_set, head/base revisions, roots, matched/unmatched layers", func() {
			fields := rawReportFields(report)
			Expect(fields).To(HaveKey("project_scope"))
			var scope map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_scope"], &scope)).To(Succeed())

			var inclusionRule, patternSet string
			Expect(json.Unmarshal(scope["inclusion_rule"], &inclusionRule)).To(Succeed())
			Expect(inclusionRule).To(Equal(string(projectmodel.InclusionRuleTSConfigIncludesNoTestClassification)))
			Expect(json.Unmarshal(scope["pattern_set"], &patternSet)).To(Succeed())
			Expect(patternSet).To(Equal(string(projectmodel.TSReachabilityAlgorithm)))

			var head map[string]json.RawMessage
			Expect(json.Unmarshal(scope["head"], &head)).To(Succeed())
			var headRevision string
			Expect(json.Unmarshal(head["revision"], &headRevision)).To(Succeed())
			Expect(headRevision).To(Equal("HEAD_SHA"))
			var roots []json.RawMessage
			Expect(json.Unmarshal(head["roots"], &roots)).To(Succeed())
			Expect(roots).To(HaveLen(1))
			var matchedLayers, unmatchedLayers []string
			Expect(json.Unmarshal(head["matched_layers"], &matchedLayers)).To(Succeed())
			Expect(matchedLayers).To(Equal([]string{"handlers"}))
			Expect(json.Unmarshal(head["unmatched_layers"], &unmatchedLayers)).To(Succeed())
			Expect(unmatchedLayers).To(Equal([]string{"db"}))

			Expect(scope).To(HaveKey("base"))
			var base map[string]json.RawMessage
			Expect(json.Unmarshal(scope["base"], &base)).To(Succeed())
			var baseRevision string
			Expect(json.Unmarshal(base["revision"], &baseRevision)).To(Succeed())
			Expect(baseRevision).To(Equal("BASE_SHA"))
		})

		// Base scope must be populated from BaseProjectScope, not conflated with HeadProjectScope.
		It("maps base scope roots[0].candidate_files from BaseProjectScope, distinct from head's 10", func() {
			fields := rawReportFields(report)
			var scope map[string]json.RawMessage
			Expect(json.Unmarshal(fields["project_scope"], &scope)).To(Succeed())
			var base map[string]json.RawMessage
			Expect(json.Unmarshal(scope["base"], &base)).To(Succeed())
			var baseRoots []json.RawMessage
			Expect(json.Unmarshal(base["roots"], &baseRoots)).To(Succeed())
			Expect(baseRoots).To(HaveLen(1))
			var root map[string]json.RawMessage
			Expect(json.Unmarshal(baseRoots[0], &root)).To(Succeed())
			var candidateFiles int
			Expect(json.Unmarshal(root["candidate_files"], &candidateFiles)).To(Succeed())
			Expect(candidateFiles).To(Equal(7), "base scope must carry its own distinct candidate_files from BaseProjectScope, not head's 10")
			var matchedLayers []string
			Expect(json.Unmarshal(base["matched_layers"], &matchedLayers)).To(Succeed())
			Expect(matchedLayers).To(Equal([]string{"api"}), "base matched_layers must differ from head's [handlers]")
		})

		// Base provenance coverage must be populated from BaseXxxCoverage, not HeadXxxCoverage.
		It("maps project_provenance.base.coverage.reachability from BaseReachabilityCoverage, distinct from head's complete", func() {
			prov := extractProvenance(report)
			var base map[string]json.RawMessage
			Expect(json.Unmarshal(prov["base"], &base)).To(Succeed())
			var baseCov map[string]json.RawMessage
			Expect(json.Unmarshal(base["coverage"], &baseCov)).To(Succeed())
			var reachability string
			Expect(json.Unmarshal(baseCov["reachability"], &reachability)).To(Succeed())
			Expect(reachability).To(Equal("incomplete"), "base provenance reachability must reflect BaseReachabilityCoverage, distinct from head's complete")
		})
	})

	// AC-6: baseline complete_no_match PLUS a non-empty diagnostics list →
	// inspect_diagnostics appears last in next_actions.
	When("baseline complete_no_match holds and the report has a diagnostic", func() {
		It("includes inspect_diagnostics as the last next_action", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:           "typescript",
				Scope:              codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:   headProjectScope(),
				HeadModelCoverage:  completeCoverage("model"),
				HeadBypassCoverage: notRequestedCoverage(),
				ProjectCoverage:    &projectmodel.Coverage{Phase: "full", Complete: true},
				Diagnostics: []codesignal.Diagnostic{
					{Kind: "project_coverage_incomplete", Message: "some partial coverage"},
				},
			})
			kinds := nextActionKinds(report)
			Expect(kinds).To(Equal([]string{"record_baseline", "review_policy_coverage", "inspect_diagnostics"}))
		})
	})

	// Base-side model incompleteness blocks complete_no_match in diff mode.
	When("diff mode has complete head coverage but incomplete base model coverage", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			report = build(codesignal.Options{ProjectEnabled: true, Baseline: false}, codesignal.Input{
				Language:                 "typescript",
				Scope:                    codesignal.Scope{Revision: "HEAD_SHA", Base: "BASE_SHA"},
				ProjectBaseAnalyzed:      true,
				HeadProjectScope:         headProjectScope(),
				BaseProjectScope:         baseProjectScope(),
				HeadModelCoverage:        completeCoverage("model"),
				HeadBypassCoverage:       notRequestedCoverage(),
				HeadReachabilityCoverage: completeCoverage("reachability"),
				BaseModelCoverage:        incompleteCoverage("model"),
				BaseBypassCoverage:       notRequestedCoverage(),
				BaseReachabilityCoverage: completeCoverage("reachability"),
				ProjectCoverage:          &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage:      &projectmodel.Coverage{Phase: "full", Complete: true},
			})
		})

		It("omits project_next_actions because base model coverage is not required-complete", func() {
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_next_actions"))
		})

		It("maps project_provenance.base.coverage.model=incomplete", func() {
			prov := extractProvenance(report)
			var base map[string]json.RawMessage
			Expect(json.Unmarshal(prov["base"], &base)).To(Succeed())
			var baseCov map[string]json.RawMessage
			Expect(json.Unmarshal(base["coverage"], &baseCov)).To(Succeed())
			var model string
			Expect(json.Unmarshal(baseCov["model"], &model)).To(Succeed())
			Expect(model).To(Equal("incomplete"))
		})
	})

	// Head bypass incompleteness blocks complete_no_match even when the model is complete.
	When("baseline has complete head model but incomplete head bypass coverage", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			report = build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:           "typescript",
				Scope:              codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:   headProjectScope(),
				HeadModelCoverage:  completeCoverage("model"),
				HeadBypassCoverage: incompleteCoverage("bypass"),
				ProjectCoverage:    &projectmodel.Coverage{Phase: "full", Complete: true},
			})
		})

		It("omits project_next_actions because bypass coverage is not required-complete", func() {
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_next_actions"))
		})

		It("maps project_provenance.head.coverage.bypass=incomplete", func() {
			cov := extractHeadCoverage(report)
			var bypass string
			Expect(json.Unmarshal(cov["bypass"], &bypass)).To(Succeed())
			Expect(bypass).To(Equal("incomplete"))
		})
	})

	// AC-7: an active layer-rule ProjectChange blocks complete_no_match even when coverage is
	// fully complete; the table exercises both rule IDs.
	DescribeTable("omits project_next_actions when an active layer-rule finding is present",
		func(ruleID string) {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:           "typescript",
				Scope:              codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:   headProjectScope(),
				HeadModelCoverage:  completeCoverage("model"),
				HeadBypassCoverage: notRequestedCoverage(),
				ProjectCoverage:    &projectmodel.Coverage{Phase: "full", Complete: true},
				ProjectChanges:     []codesignal.ProjectChange{layerViolationChange(ruleID)},
			})
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_next_actions"))
		},
		Entry("architecture.layer_violation", "architecture.layer_violation"),
		Entry("architecture.layer_bypass", "architecture.layer_bypass"),
	)

	// analyzer.protocol_version defaults to 1 for TypeScript reports when
	// Input.AnalyzerProtocolVersion is the zero value.
	When("a TypeScript baseline has AnalyzerProtocolVersion not explicitly set (zero value)", func() {
		It("emits analyzer.protocol_version == 1 in project_provenance by default for typescript", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:           "typescript",
				Scope:              codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:   headProjectScope(),
				HeadModelCoverage:  completeCoverage("model"),
				HeadBypassCoverage: notRequestedCoverage(),
				ProjectCoverage:    &projectmodel.Coverage{Phase: "full", Complete: true},
				// AnalyzerProtocolVersion deliberately left as zero value
			})
			prov := extractProvenance(report)
			var analyzer map[string]json.RawMessage
			Expect(json.Unmarshal(prov["analyzer"], &analyzer)).To(Succeed())
			var protocolVersion int
			Expect(json.Unmarshal(analyzer["protocol_version"], &protocolVersion)).To(Succeed())
			Expect(protocolVersion).To(Equal(1), "typescript analyzer.protocol_version must default to 1, not 0")
		})
	})

	When("a TypeScript baseline has AnalyzerProtocolVersion explicitly set to 1", func() {
		It("emits the supplied analyzer.protocol_version == 1 in project_provenance", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:                "typescript",
				Scope:                   codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:        headProjectScope(),
				HeadModelCoverage:       completeCoverage("model"),
				HeadBypassCoverage:      notRequestedCoverage(),
				ProjectCoverage:         &projectmodel.Coverage{Phase: "full", Complete: true},
				AnalyzerProtocolVersion: 1,
			})
			prov := extractProvenance(report)
			var analyzer map[string]json.RawMessage
			Expect(json.Unmarshal(prov["analyzer"], &analyzer)).To(Succeed())
			var protocolVersion int
			Expect(json.Unmarshal(analyzer["protocol_version"], &protocolVersion)).To(Succeed())
			Expect(protocolVersion).To(Equal(1))
		})
	})

	// AC-8: Options.ProjectEnabled with Language="" or "go" → project_provenance,
	// project_scope, and project_next_actions are omitted even when HeadProjectScope is set.
	When("ProjectEnabled is true but Language is not typescript", func() {
		DescribeTable("omits project_provenance, project_scope, and project_next_actions",
			func(language string) {
				report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
					Language:          language,
					Scope:             codesignal.Scope{Revision: "HEAD_SHA"},
					HeadProjectScope:  headProjectScope(),
					HeadModelCoverage: completeCoverage("model"),
					ProjectCoverage:   &projectmodel.Coverage{Phase: "full", Complete: true},
				})
				fields := rawReportFields(report)
				Expect(fields).NotTo(HaveKey("project_provenance"))
				Expect(fields).NotTo(HaveKey("project_scope"))
				Expect(fields).NotTo(HaveKey("project_next_actions"))
			},
			Entry("empty language", ""),
			Entry("go language", "go"),
		)
	})

	// AC-EVD-6: diff mode with IncludeResolved:false and a base-only
	// architecture.layer_violation classified as resolved. The resolved change
	// is filtered out of the active set, so complete_no_match must still be
	// blocked — ResolvedChanges on the summary (counted before the filter) must
	// prevent it.
	When("diff mode has a base-only architecture.layer_violation with IncludeResolved:false", func() {
		It("omits project_next_actions because the resolved change blocks complete_no_match", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: false}, codesignal.Input{
				Language:            "typescript",
				Scope:               codesignal.Scope{Revision: "HEAD_SHA", Base: "BASE_SHA"},
				ProjectBaseAnalyzed: true,
				HeadProjectScope:    headProjectScope(),
				BaseProjectScope:    headProjectScope(),
				HeadModelCoverage:   completeCoverage("model"),
				HeadBypassCoverage:  notRequestedCoverage(),
				BaseModelCoverage:   completeCoverage("model"),
				BaseBypassCoverage:  notRequestedCoverage(),
				ProjectCoverage:     &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
				BaseProjectChanges:  []codesignal.ProjectChange{layerViolationChange("architecture.layer_violation")},
			})
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(1), "the base-only violation must be counted as resolved")
			Expect(report.ProjectChanges).To(BeEmpty(), "resolved changes must not appear in the active set when IncludeResolved is false")
			fields := rawReportFields(report)
			Expect(fields).NotTo(HaveKey("project_next_actions"), "a resolved change must block complete_no_match even when filtered from the active set")
		})
	})

	// AC-9: file-local Signals with complete TS coverage and zero project
	// findings still trigger complete_no_match (facts-only observations do not
	// block it either).
	When("a file-local Signal is present alongside complete TS project coverage with no project findings", func() {
		It("still holds complete_no_match and emits project_next_actions", func() {
			report := build(codesignal.Options{ProjectEnabled: true, Baseline: true}, codesignal.Input{
				Language:           "typescript",
				Scope:              codesignal.Scope{Revision: "HEAD_SHA"},
				HeadProjectScope:   headProjectScope(),
				HeadModelCoverage:  completeCoverage("model"),
				HeadBypassCoverage: notRequestedCoverage(),
				ProjectCoverage:    &projectmodel.Coverage{Phase: "full", Complete: true},
				Files: []codesignal.FileChange{
					{Path: "src/state.go", Status: "modified", Head: cleanResult("src/state.go", mutation("Update", 1))},
				},
				ProjectFacts: []codesignal.ProjectFact{
					{Kind: "possible_call_reachability", SemanticKey: "reach:a->b", Provenance: codesignal.Provenance{Producer: "projectmodel"}},
				},
			})
			kinds := nextActionKinds(report)
			Expect(kinds).NotTo(BeEmpty(), "complete_no_match must hold even when file-local signals are present")
			Expect(kinds).To(ContainElement("review_policy_coverage"))
		})
	})
})

var _ = DescribeTable("RequiredCoverageIncomplete (AC-EVD-7 predicate contract)",
	func(report *codesignal.Report, want bool) {
		Expect(codesignal.RequiredCoverageIncomplete(report)).To(Equal(want))
	},
	Entry("nil report returns false",
		(*codesignal.Report)(nil), false),
	Entry("no provenance, no diagnostics returns false",
		&codesignal.Report{}, false),
	Entry("no provenance, unrelated diagnostic returns false",
		&codesignal.Report{Diagnostics: []codesignal.Diagnostic{{Kind: "unrelated"}}}, false),
	Entry("no provenance, project_backend_unavailable diagnostic returns true",
		&codesignal.Report{Diagnostics: []codesignal.Diagnostic{{Kind: "project_backend_unavailable"}}}, true),
	Entry("provenance with head model=complete, bypass=not_requested returns false",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested"}},
		}}, false),
	Entry("provenance with head model=complete, bypass=complete returns false",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "complete"}},
		}}, false),
	Entry("provenance with head model=incomplete returns true",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "incomplete", Bypass: "not_requested"}},
		}}, true),
	Entry("provenance with head bypass=incomplete returns true",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "incomplete"}},
		}}, true),
	Entry("provenance with reachability=incomplete only returns false (reachability alone does not count)",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested", Reachability: "incomplete"}},
		}}, false),
	Entry("provenance with complete head and complete base returns false",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested"}},
			Base: &codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested"}},
		}}, false),
	Entry("provenance with complete head but incomplete base model returns true",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested"}},
			Base: &codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "incomplete", Bypass: "not_requested"}},
		}}, true),
	Entry("provenance with complete head but incomplete base bypass returns true",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_requested"}},
			Base: &codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "incomplete"}},
		}}, true),
	Entry("provenance with head model=not_run returns true",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "not_run", Bypass: "not_requested"}},
		}}, true),
	Entry("provenance with head bypass=not_run returns true",
		&codesignal.Report{ProjectProvenance: &codesignal.ProjectProvenance{
			Head: codesignal.ProvenanceRevision{Coverage: codesignal.ProvenanceCoverage{Model: "complete", Bypass: "not_run"}},
		}}, true),
)
