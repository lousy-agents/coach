package codesignal

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// diagLayerBypassCoverageIncomplete marks that result.Coverage (the
// pkg/projectmodel search that produced result.Witnesses) did not complete.
// See EvaluateGoLayerBypass's doc comment for what a caller must do with it.
const diagLayerBypassCoverageIncomplete = "project_layer_bypass_coverage_incomplete"

// ruleLayerBypassID identifies the architecture.layer_bypass rule in
// ProjectChange.RuleID.
const ruleLayerBypassID = "architecture.layer_bypass"

const layerBypassWhyItMatters = "A statically resolved call path from a handler-shaped source to a pinned sink that structurally never passes through a required intermediate layer means that layer's invariants (validation, authorization, caching, ...) can be silently skipped by this route, even if a separate compliant path also exists."

const layerBypassRecommendation = "Route the call through the required layer, or add an explicit exception/allowlist entry if the bypass is intentional so it stops surfacing as drift."

// EvaluateGoLayerBypass maps result's high-confidence LayerBypassWitnesses
// (see pkg/projectmodel.BuildGoLayerBypass) onto one architecture.layer_bypass
// ProjectChange per witness. A witness whose Confidence is not
// LayerBypassConfidenceHigh is silently skipped rather than downgraded: this
// re-checks a contract BuildGoLayerBypass already guarantees, defensively,
// across the package boundary.
//
// SemanticKey is keyed on (RequiredLayer, Source, Sink) -- the same triple
// LayerBypassWitness.ID uses -- so identity stays stable when only the
// witness's intermediate Path changes between two revisions. Callers that
// need Changed to reflect a route change must rely on CausalEvidenceDigest,
// which this function derives from the ordered Path node-ID sequence.
//
// PrimaryAnchor and each ProjectPathStep.SourceLocations are populated from
// the witness's own Path[i].Path/Line (a real repository-relative source
// position -- see pkg/projectmodel/go_layer_bypass.go) rather than a
// call-graph node identity string: PrimaryAnchor uses the first Path step
// with a resolvable position (ordinarily the witness's Source function
// itself; a later step only when Source's own position is unresolvable,
// e.g. a synthetic wrapper). When no step on the path resolves a position at
// all, PrimaryAnchor is left empty rather than fabricated -- callers must
// route this return value through Build's filterAnchorlessProjectChanges (or
// equivalent), which drops the anchorless change and emits a
// project_observation_missing_primary_path diagnostic instead of surfacing a
// fake location. The witness's SSA node identity is never lost: it stays on
// MachineEvidence["source"]/["path"] and every ProjectPathStep.NodeID.
//
// result.Coverage reports how much of BuildGoLayerBypass's own source/sink
// search actually completed; it is independent of Build's project-level
// Input.ProjectCoverage/BaseProjectCoverage gate. When result.Coverage is not
// complete, EvaluateGoLayerBypass still emits any witnesses found (they are
// real, statically resolved paths regardless of what else was truncated) but
// also returns a project_layer_bypass_coverage_incomplete Diagnostic,
// because the absence of a witness for some other source/sink pair in this
// same incomplete run is not a sound "no bypass" claim. Callers must fold
// result.Coverage into Input.ProjectCoverage/BaseProjectCoverage (the two
// Coverage values this rule's caller is responsible for combining, alongside
// every other project evaluator's) so projectLifecycleState (codesignal.go)
// degrades affected ProjectChanges to Lifecycle "unknown" rather than
// emitting "resolved" for a witness that only disappeared because the search
// was truncated, not because the bypass was actually fixed.
func EvaluateGoLayerBypass(result projectmodel.LayerBypassResult, ruleVersion, backendVersion, configDigest string) ([]ProjectChange, []Diagnostic) {
	var diagnostics []Diagnostic
	if !result.Coverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    diagLayerBypassCoverageIncomplete,
			Message: "go layer-bypass search coverage is incomplete; absence of a witness for a source/sink pair in this run does not mean no bypass exists",
		})
	}

	witnesses := make([]projectmodel.LayerBypassWitness, 0, len(result.Witnesses))
	for _, witness := range result.Witnesses {
		if witness.Confidence != projectmodel.LayerBypassConfidenceHigh {
			continue
		}
		witnesses = append(witnesses, witness)
	}
	if len(witnesses) == 0 {
		return nil, diagnostics
	}

	sort.SliceStable(witnesses, func(i, j int) bool {
		if witnesses[i].Source != witnesses[j].Source {
			return witnesses[i].Source < witnesses[j].Source
		}
		return witnesses[i].Sink < witnesses[j].Sink
	})

	changes := make([]ProjectChange, 0, len(witnesses))
	for _, witness := range witnesses {
		changes = append(changes, layerBypassChange(witness, ruleVersion, backendVersion, configDigest, ""))
	}
	return changes, diagnostics
}

// EvaluateTypeScriptLayerBypass is the TypeScript analog of
// EvaluateGoLayerBypass: it maps result's high-confidence LayerBypassWitnesses
// (see pkg/projectmodel.BuildTypeScriptLayerBypass) onto one
// architecture.layer_bypass ProjectChange per witness, under the same
// ruleLayerBypassID vocabulary, plus a MachineEvidence["language"] =
// "typescript" entry -- mirroring EvaluateTypeScriptLayerViolations'
// relationship to EvaluateGoLayerViolations (rule_layer_violation.go). See
// EvaluateGoLayerBypass's doc comment for the shared confidence-filtering,
// anchoring, and coverage-incompleteness contract this function reuses
// unchanged via layerBypassChange.
func EvaluateTypeScriptLayerBypass(result projectmodel.LayerBypassResult, ruleVersion, backendVersion, configDigest string) ([]ProjectChange, []Diagnostic) {
	var diagnostics []Diagnostic
	if !result.Coverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    diagLayerBypassCoverageIncomplete,
			Message: "typescript layer-bypass search coverage is incomplete; absence of a witness for a source/sink pair in this run does not mean no bypass exists",
		})
	}

	witnesses := make([]projectmodel.LayerBypassWitness, 0, len(result.Witnesses))
	for _, witness := range result.Witnesses {
		if witness.Confidence != projectmodel.LayerBypassConfidenceHigh {
			continue
		}
		witnesses = append(witnesses, witness)
	}
	if len(witnesses) == 0 {
		return nil, diagnostics
	}

	sort.SliceStable(witnesses, func(i, j int) bool {
		if witnesses[i].Source != witnesses[j].Source {
			return witnesses[i].Source < witnesses[j].Source
		}
		return witnesses[i].Sink < witnesses[j].Sink
	})

	changes := make([]ProjectChange, 0, len(witnesses))
	for _, witness := range witnesses {
		changes = append(changes, layerBypassChange(witness, ruleVersion, backendVersion, configDigest, "typescript"))
	}
	return changes, diagnostics
}

// layerBypassChange builds the shared architecture.layer_bypass ProjectChange
// shape for both language evaluators. language is added to
// MachineEvidence["language"] only when non-empty, so Go's evaluator (which
// passes "") keeps its already-frozen MachineEvidence keys while
// EvaluateTypeScriptLayerBypass gains a "language": "typescript" entry --
// mirroring rule_layer_violation.go's layerViolationChange split (issue
// #215).
func layerBypassChange(witness projectmodel.LayerBypassWitness, ruleVersion, backendVersion, configDigest, language string) ProjectChange {
	nodeIDs := make([]string, len(witness.Path))
	pathSteps := make([]ProjectPathStep, len(witness.Path))
	for i, step := range witness.Path {
		nodeIDs[i] = step.NodeID
		pathSteps[i] = ProjectPathStep{
			NodeID:          step.NodeID,
			Confidence:      Confidence("high"),
			SourceLocations: layerBypassStepSourceLocations(step),
		}
	}

	machineEvidence := map[string]string{
		"source":         witness.Source,
		"sink":           witness.Sink,
		"required_layer": witness.RequiredLayer,
		"path":           strings.Join(nodeIDs, "->"),
	}
	if language != "" {
		machineEvidence["language"] = language
	}

	return ProjectChange{
		SemanticKey:          "architecture.layer_bypass:" + witness.RequiredLayer + ":" + witness.Source + "->" + witness.Sink,
		RuleID:               ruleLayerBypassID,
		RuleVersion:          ruleVersion,
		BackendVersion:       backendVersion,
		AlgorithmVersion:     witness.AlgorithmVersion,
		ConfigDigest:         configDigest,
		Kind:                 ruleLayerBypassID,
		Category:             Category("architecture"),
		Severity:             Severity("advisory"),
		Confidence:           Confidence("high"),
		CausalEvidenceDigest: layerBypassCausalDigest(nodeIDs),
		PrimaryAnchor:        layerBypassPrimaryAnchor(witness.Path),
		PathSteps:            pathSteps,
		MachineEvidence:      machineEvidence,
		Evidence:             witness.Source + " reaches " + witness.Sink + " via a statically resolved path that never passes through required layer \"" + witness.RequiredLayer + "\"",
		WhyItMatters:         layerBypassWhyItMatters,
		Recommendation:       layerBypassRecommendation,
		Provenance:           Provenance{Producer: "projectmodel", FindingKind: ruleLayerBypassID},
	}
}

// layerBypassPrimaryAnchor returns the first path step with a resolvable
// source position -- ordinarily steps[0] (the witness's Source function) --
// or a zero ProjectLocation when no step resolves one (see
// EvaluateGoLayerBypass's doc comment for what happens to the resulting
// change downstream).
func layerBypassPrimaryAnchor(steps []projectmodel.LayerBypassStep) ProjectLocation {
	for _, step := range steps {
		if loc, ok := layerBypassStepLocation(step); ok {
			return loc
		}
	}
	return ProjectLocation{}
}

func layerBypassStepSourceLocations(step projectmodel.LayerBypassStep) []ProjectLocation {
	loc, ok := layerBypassStepLocation(step)
	if !ok {
		return nil
	}
	return []ProjectLocation{loc}
}

// layerBypassStepLocation converts step's 1-based Line to the 0-based
// StartRow ProjectLocation uses elsewhere in this package, mirroring
// rule_layer_violation.go's parseSiteLocation. It reports false when step
// carries no position (step.Path == ""), e.g. the sink.
func layerBypassStepLocation(step projectmodel.LayerBypassStep) (ProjectLocation, bool) {
	if step.Path == "" {
		return ProjectLocation{}, false
	}
	if step.Line <= 0 {
		return ProjectLocation{Path: step.Path}, true
	}
	return ProjectLocation{Path: step.Path, Location: semantics.Location{StartRow: uint(step.Line - 1)}}, true
}

// layerBypassCausalDigest hashes the ordered Path node-ID sequence so
// projectChangeChanged (project_lifecycle.go) can detect a route change
// (Changed == true) even when SemanticKey/Fingerprint identity, keyed only on
// (RequiredLayer, Source, Sink), stays the same.
func layerBypassCausalDigest(nodeIDs []string) string {
	var buf []byte
	for _, id := range nodeIDs {
		buf = appendLengthPrefixed(buf, id)
	}
	sum := sha256.Sum256(buf)
	return "cev_" + hex.EncodeToString(sum[:])
}
