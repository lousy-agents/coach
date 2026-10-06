package codesignal

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// layerBypassPrimaryAnchor returns the first path step with a resolvable
// source position -- ordinarily steps[0] (the witness's Source function) --
// or a zero ProjectLocation when no step resolves one (see
// EvaluateGoLayerBypass's doc comment for what happens to the resulting
// change downstream).
func layerBypassPrimaryAnchor(steps []domain.LayerBypassStep) ProjectLocation {
	for _, step := range steps {
		if loc, ok := layerBypassStepLocation(step); ok {
			return loc
		}
	}
	return ProjectLocation{}
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
