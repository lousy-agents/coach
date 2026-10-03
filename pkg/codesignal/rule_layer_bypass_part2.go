package codesignal

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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
func EvaluateTypeScriptLayerBypass(result domain.LayerBypassResult, ruleVersion, backendVersion, configDigest string) ([]ProjectChange, []Diagnostic) {
	var diagnostics []Diagnostic
	if !result.Coverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    diagLayerBypassCoverageIncomplete,
			Message: "typescript layer-bypass search coverage is incomplete; absence of a witness for a source/sink pair in this run does not mean no bypass exists",
		})
	}

	witnesses := make([]domain.LayerBypassWitness, 0, len(result.Witnesses))
	for _, witness := range result.Witnesses {
		if witness.Confidence != domain.LayerBypassConfidenceHigh {
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

// layerBypassStepLocation converts step's 1-based Line to the 0-based
// StartRow ProjectLocation uses elsewhere in this package, mirroring
// rule_layer_violation.go's parseSiteLocation. It reports false when step
// carries no position (step.Path == ""), e.g. the sink.
func layerBypassStepLocation(step domain.LayerBypassStep) (ProjectLocation, bool) {
	if step.Path == "" {
		return ProjectLocation{}, false
	}
	if step.Line <= 0 {
		return ProjectLocation{Path: step.Path}, true
	}
	return ProjectLocation{Path: step.Path, Location: semantics.Location{StartRow: uint(step.Line - 1)}}, true
}
func layerBypassStepSourceLocations(step domain.LayerBypassStep) []ProjectLocation {
	loc, ok := layerBypassStepLocation(step)
	if !ok {
		return nil
	}
	return []ProjectLocation{loc}
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
