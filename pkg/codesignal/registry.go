package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

// densityGateThreshold is the minimum per-file, per-kind finding count
// required before a gated rule (constructor_func, pointer_return) emits any
// signals. Below this threshold the kind is treated the same as an
// unrecognized kind: silently ignored.
const densityGateThreshold = 2

// gatedFindingKinds are Finding.Kind values only surfaced once their
// per-file count (computed independently per side) reaches
// densityGateThreshold.
var gatedFindingKinds = map[string]bool{
	"constructor_func": true,
	"pointer_return":   true,
}

// ruleRegistry maps semantics.Finding.Kind to the Signal constructor for
// that kind. Kinds absent from this map are silently ignored: this is the
// single dispatch point processHeadResult and extractBaseSignals use, so
// adding a rule means adding an entry here rather than a new conditional.
var ruleRegistry = map[string]func(path string, finding semantics.Finding) Signal{
	"mutates_input":    newHiddenInputMutationSignal,
	"tight_coupling":   newTightCouplingSignal,
	"constructor_func": newConstructorDensitySignal,
	"pointer_return":   newPointerReturnDensitySignal,
}

// findingCountsByKind counts findings in one side's Findings slice by Kind,
// for use in per-file density gating. Callers must pass only one side's
// findings at a time (fc.Head.Findings or fc.Base.Findings) so gating stays
// independent per side.
func findingCountsByKind(findings []semantics.Finding) map[string]int {
	counts := make(map[string]int, len(findings))
	for _, finding := range findings {
		counts[finding.Kind]++
	}
	return counts
}

// signalsFromFindings maps findings to Signals via ruleRegistry, applying
// the density gate to gatedFindingKinds using counts (the per-kind counts
// within this same findings slice, from findingCountsByKind).
func signalsFromFindings(path string, findings []semantics.Finding, counts map[string]int) []Signal {
	var signals []Signal
	for _, finding := range findings {
		newSignal, recognized := ruleRegistry[finding.Kind]
		if !recognized {
			continue
		}
		if gatedFindingKinds[finding.Kind] && counts[finding.Kind] < densityGateThreshold {
			continue
		}
		signals = append(signals, newSignal(path, finding))
	}
	return signals
}
