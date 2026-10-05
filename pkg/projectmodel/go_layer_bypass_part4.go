package projectmodel

import (
	"fmt"
)

func (s *layerBypassSearch) collectWitnesses(source string, sinks []string, parents map[string]string, nodePositions map[string]layerBypassNodePosition, requiredLayer string, skip bool) {
	for _, sink := range sinks {
		if skip {
			s.truncatedPairs++
			continue
		}
		s.evaluated++
		stepPath, ok := reconstructReachabilityPath(parents, source, sink)
		if !ok {
			continue
		}
		if !stepPathFullyClassified(stepPath, nodePositions) {
			s.unclassifiedNodeSeen = true
			continue
		}
		s.witnesses = append(s.witnesses, LayerBypassWitness{
			ID:               fmt.Sprintf("bypass:%s:%s->%s@%s", requiredLayer, source, sink, LayerBypassAlgorithm),
			Source:           source,
			Sink:             sink,
			RequiredLayer:    requiredLayer,
			Path:             layerBypassSteps(stepPath, nodePositions),
			Confidence:       LayerBypassConfidenceHigh,
			AlgorithmVersion: LayerBypassAlgorithm,
		})
	}
}

// remapDiagnosticCodes returns a copy of diags with every Code present in
// codes rewritten to its mapped value, leaving any other diagnostic
// untouched. It is used to fold a shared helper's diagnostics (e.g.
// findGoReachabilitySources') into this evaluator's own diagnostic-code
// vocabulary rather than leaking a different feature's codes into
// LayerBypassResult.Coverage.
func remapDiagnosticCodes(diags []Diagnostic, codes map[string]string) []Diagnostic {
	if len(diags) == 0 {
		return diags
	}
	out := make([]Diagnostic, len(diags))
	for i, d := range diags {
		if mapped, ok := codes[d.Code]; ok {
			d.Code = mapped
		}
		out[i] = d
	}
	return out
}

// layerBypassContainsDir mirrors codesignal's layerContainsDir: "." matches
// every directory, otherwise a prefix matches dir itself or any "/"-
// separated descendant of it.
func layerBypassContainsDir(layer BypassLayer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if prefix == "." || dir == prefix || (len(dir) > len(prefix) && dir[:len(prefix)+1] == prefix+"/") {
			return true
		}
	}
	return false
}
