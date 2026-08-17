package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// crossLangBypassWitness returns a LayerBypassWitness whose Source, Sink,
// RequiredLayer, Path and Confidence are identical regardless of
// algorithmVersion, so a Go-tagged and a TS-tagged witness built from this
// helper are structurally equivalent by construction -- any difference
// between the two evaluators' output must come from the evaluator called,
// not from a fixture difference.
func crossLangBypassWitness(algorithmVersion string) projectmodel.LayerBypassWitness {
	return projectmodel.LayerBypassWitness{
		ID:            "bypass:service:example.com/app/handlers.Handler->(*database/sql.DB).Query@" + algorithmVersion,
		Source:        "example.com/app/handlers.Handler",
		Sink:          "(*database/sql.DB).Query",
		RequiredLayer: "service",
		Path: []projectmodel.LayerBypassStep{
			{NodeID: "example.com/app/handlers.Handler", Path: "pkg/handlers/handlers.go", Line: 3},
			{NodeID: "(*database/sql.DB).Query"},
		},
		Confidence:       projectmodel.LayerBypassConfidenceHigh,
		AlgorithmVersion: algorithmVersion,
	}
}

func crossLangBypassResult(phase string, witnesses ...projectmodel.LayerBypassWitness) projectmodel.LayerBypassResult {
	algorithm := projectmodel.LayerBypassAlgorithm
	if len(witnesses) > 0 {
		algorithm = witnesses[0].AlgorithmVersion
	}
	return projectmodel.LayerBypassResult{
		Witnesses: witnesses,
		Algorithm: algorithm,
		Coverage:  projectmodel.Coverage{Phase: phase, Complete: true},
	}
}

// machineEvidenceWithoutLanguage returns a copy of evidence with the
// "language" key removed, so the two evaluators' MachineEvidence maps can be
// compared for equality everywhere except that one provenance key.
func machineEvidenceWithoutLanguage(evidence map[string]string) map[string]string {
	out := make(map[string]string, len(evidence))
	for k, v := range evidence {
		if k == "language" {
			continue
		}
		out[k] = v
	}
	return out
}

var _ = Describe("Go/TypeScript cross-language rule parity (issue #216 AC-6)", func() {
	Describe("architecture.layer_bypass: EvaluateGoLayerBypass vs EvaluateTypeScriptLayerBypass", func() {
		It("shares RuleID/Category/Severity/Confidence/lifecycle-relevant shape and differs only in language provenance", func() {
			goResult := crossLangBypassResult("go_layer_bypass", crossLangBypassWitness(projectmodel.LayerBypassAlgorithm))
			tsResult := crossLangBypassResult("ts_layer_bypass", crossLangBypassWitness(projectmodel.TSLayerBypassAlgorithm))

			goChanges, goDiagnostics := codesignal.EvaluateGoLayerBypass(goResult, "1", "backend-1", "digest-1")
			tsChanges, tsDiagnostics := codesignal.EvaluateTypeScriptLayerBypass(tsResult, "1", "backend-1", "digest-1")

			Expect(goDiagnostics).To(BeEmpty())
			Expect(tsDiagnostics).To(BeEmpty())
			Expect(goChanges).To(HaveLen(1))
			Expect(tsChanges).To(HaveLen(1))
			goChange, tsChange := goChanges[0], tsChanges[0]

			Expect(tsChange.RuleID).To(Equal(goChange.RuleID))
			Expect(tsChange.RuleID).To(Equal("architecture.layer_bypass"))
			Expect(tsChange.Kind).To(Equal(goChange.Kind))
			Expect(tsChange.Category).To(Equal(goChange.Category))
			Expect(tsChange.Severity).To(Equal(goChange.Severity))
			Expect(tsChange.Confidence).To(Equal(goChange.Confidence))
			Expect(tsChange.RuleVersion).To(Equal(goChange.RuleVersion))
			Expect(tsChange.BackendVersion).To(Equal(goChange.BackendVersion))
			Expect(tsChange.ConfigDigest).To(Equal(goChange.ConfigDigest))
			Expect(tsChange.SemanticKey).To(Equal(goChange.SemanticKey))
			Expect(tsChange.PrimaryAnchor).To(Equal(goChange.PrimaryAnchor))
			Expect(tsChange.PathSteps).To(Equal(goChange.PathSteps))
			Expect(tsChange.CausalEvidenceDigest).To(Equal(goChange.CausalEvidenceDigest))
			Expect(tsChange.WhyItMatters).To(Equal(goChange.WhyItMatters))
			Expect(tsChange.Recommendation).To(Equal(goChange.Recommendation))
			Expect(tsChange.Provenance).To(Equal(goChange.Provenance))

			// The only fields that differ are the ones that actually carry
			// language provenance: AlgorithmVersion (set from the witness, which
			// this fixture tags per-language) and MachineEvidence["language"]
			// (set by layerBypassChange only for the TS evaluator -- see
			// rule_layer_bypass.go).
			Expect(goChange.AlgorithmVersion).To(Equal(projectmodel.LayerBypassAlgorithm))
			Expect(tsChange.AlgorithmVersion).To(Equal(projectmodel.TSLayerBypassAlgorithm))
			Expect(goChange.MachineEvidence).NotTo(HaveKey("language"))
			Expect(tsChange.MachineEvidence).To(HaveKeyWithValue("language", "typescript"))
			Expect(machineEvidenceWithoutLanguage(tsChange.MachineEvidence)).To(Equal(goChange.MachineEvidence))
		})
	})

	// possible_call_reachability facts carry their language provenance in
	// Provenance.Language, set from the caller-supplied language argument to
	// ReachabilityProjectFacts (project_facts_reachability.go) -- the
	// ProjectFact analog of architecture.layer_bypass's
	// MachineEvidence["language"].
	Describe("possible_call_reachability: ReachabilityProjectFacts language provenance", func() {
		It("produces identically-shaped facts for Go- and TS-sourced results, differing only in Provenance.Language", func() {
			goResult := projectmodel.ReachabilityResult{
				Facts: []projectmodel.ReachabilityFact{{
					ID:         "reach:example.com/app/handlers.Handler->(*database/sql.DB).Query@" + projectmodel.ReachabilityAlgorithm,
					Kind:       projectmodel.KindPossibleCallReachability,
					Confidence: projectmodel.ReachabilityConfidenceResolvedDirect,
					Source:     "example.com/app/handlers.Handler",
					Sink:       "(*database/sql.DB).Query",
					Path: []projectmodel.ReachabilityStep{
						{NodeID: "example.com/app/handlers.Handler"},
						{NodeID: "(*database/sql.DB).Query"},
					},
					AlgorithmVersion: projectmodel.ReachabilityAlgorithm,
				}},
				Algorithm: projectmodel.ReachabilityAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "go_model_build", Complete: true},
			}
			tsResult := projectmodel.ReachabilityResult{
				Facts: []projectmodel.ReachabilityFact{{
					ID:         "reach:file:src/app.ts#getUsers->(PrismaClient).findMany@" + projectmodel.TSReachabilityAlgorithm,
					Kind:       projectmodel.KindPossibleCallReachability,
					Confidence: projectmodel.ReachabilityConfidenceResolvedDirect,
					Source:     "file:src/app.ts#getUsers",
					Sink:       "(PrismaClient).findMany",
					Path: []projectmodel.ReachabilityStep{
						{NodeID: "file:src/app.ts#getUsers"},
						{NodeID: "(PrismaClient).findMany"},
					},
					AlgorithmVersion: projectmodel.TSReachabilityAlgorithm,
				}},
				Algorithm: projectmodel.TSReachabilityAlgorithm,
				Coverage:  projectmodel.Coverage{Phase: "ts_sidecar_build", Complete: true},
			}

			goFacts := codesignal.ReachabilityProjectFacts(goResult, "go")
			tsFacts := codesignal.ReachabilityProjectFacts(tsResult, "typescript")
			Expect(goFacts).To(HaveLen(1))
			Expect(tsFacts).To(HaveLen(1))
			goFact, tsFact := goFacts[0], tsFacts[0]

			Expect(tsFact.Kind).To(Equal(goFact.Kind))
			Expect(tsFact.Kind).To(Equal(projectmodel.KindPossibleCallReachability))
			Expect(tsFact.PathSteps[0].Confidence).To(Equal(goFact.PathSteps[0].Confidence))

			// The only field that differs is the one that actually carries
			// language provenance: Provenance.Language, set from the language
			// argument each evaluator passes to ReachabilityProjectFacts.
			Expect(goFact.Provenance.Language).To(Equal("go"))
			Expect(tsFact.Provenance.Language).To(Equal("typescript"))
			goProvenanceSansLanguage, tsProvenanceSansLanguage := goFact.Provenance, tsFact.Provenance
			goProvenanceSansLanguage.Language, tsProvenanceSansLanguage.Language = "", ""
			Expect(tsProvenanceSansLanguage).To(Equal(goProvenanceSansLanguage))

			Expect(goFact.SemanticKey).To(ContainSubstring("example.com/app/handlers.Handler"))
			Expect(tsFact.SemanticKey).To(ContainSubstring("file:src/app.ts#getUsers"))
		})
	})

	// AC-7 false-green control, at the cross-language boundary: proves the
	// LayerBypassConfidenceHigh filter (layerBypassChange's caller loop in
	// rule_layer_bypass.go) is live on both evaluators from an otherwise
	// identical fixture, not merely "never triggered" because every fixture
	// used elsewhere already happens to be high-confidence. A naive,
	// suppression-free mapping would surface the medium-confidence witness
	// below; the real evaluators must not.
	Describe("false-green control: the high-confidence filter fires on both language paths", func() {
		It("suppresses a non-high-confidence witness and surfaces its high-confidence sibling on both Go and TS", func() {
			mediumGo := crossLangBypassWitness(projectmodel.LayerBypassAlgorithm)
			mediumGo.Confidence = "medium" // BuildGoLayerBypass never produces this; constructed directly to prove the guard fires.
			mediumTS := crossLangBypassWitness(projectmodel.TSLayerBypassAlgorithm)
			mediumTS.Confidence = "medium" // BuildTypeScriptLayerBypass never produces this either.

			goSuppressed, _ := codesignal.EvaluateGoLayerBypass(crossLangBypassResult("go_layer_bypass", mediumGo), "1", "backend-1", "digest-1")
			tsSuppressed, _ := codesignal.EvaluateTypeScriptLayerBypass(crossLangBypassResult("ts_layer_bypass", mediumTS), "1", "backend-1", "digest-1")
			Expect(goSuppressed).To(BeEmpty(), "a naive mapping with no confidence filter would have surfaced this witness")
			Expect(tsSuppressed).To(BeEmpty(), "a naive mapping with no confidence filter would have surfaced this witness")

			goSurfaced, _ := codesignal.EvaluateGoLayerBypass(crossLangBypassResult("go_layer_bypass", crossLangBypassWitness(projectmodel.LayerBypassAlgorithm)), "1", "backend-1", "digest-1")
			tsSurfaced, _ := codesignal.EvaluateTypeScriptLayerBypass(crossLangBypassResult("ts_layer_bypass", crossLangBypassWitness(projectmodel.TSLayerBypassAlgorithm)), "1", "backend-1", "digest-1")
			Expect(goSurfaced).To(HaveLen(1), "the otherwise-identical high-confidence witness must surface on the Go path")
			Expect(tsSurfaced).To(HaveLen(1), "the otherwise-identical high-confidence witness must surface on the TS path")
		})
	})
})
