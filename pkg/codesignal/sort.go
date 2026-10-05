package codesignal

import (
	"cmp"
	"slices"
)

func signalPriorityGroup(sig Signal) int {
	switch sig.Lifecycle {
	case "introduced":
		if sig.Changed {
			return 0
		}
		return 2
	case "existing":
		if sig.Changed {
			return 1
		}
		return 3
	case "resolved":
		return 4
	default:
		return 5
	}
}

// sortSignals sorts signals by priority group, severity, confidence, path,
// location, rule, and ID, then ranks each metric rule's signals by magnitude
// within every tier.
func sortSignals(signals []Signal) {
	slices.SortStableFunc(signals, compareSignals)
	rankMetricRulesWithinTiers(signals)
}

func compareSignals(a, b Signal) int {
	return cmp.Or(
		cmp.Compare(signalPriorityGroup(a), signalPriorityGroup(b)),
		cmp.Compare(severityRank(b.Severity), severityRank(a.Severity)),
		cmp.Compare(confidenceRank(b.Confidence), confidenceRank(a.Confidence)),
		cmp.Compare(a.Path, b.Path),
		cmp.Compare(a.Location.StartRow, b.Location.StartRow),
		cmp.Compare(a.Location.StartCol, b.Location.StartCol),
		cmp.Compare(a.RuleID, b.RuleID),
		cmp.Compare(a.ID, b.ID),
	)
}

// peerKey identifies signals that compete on magnitude: one metric rule within
// one tier. Magnitude is a ratio to the rule's own threshold, so it only ranks
// a rule against its peers; comparing it across rules would put unlike metrics
// (a whole-file branch total against nesting depth) ahead of signals that have
// no magnitude at all.
type peerKey struct {
	group, severity, confidence int
	ruleID                      string
}

type rankedSignal struct {
	signal Signal
	ratio  float64
}

// rankMetricRulesWithinTiers reorders each peer group across the positions it
// already occupies in the path-ordered signals, so signals without a magnitude
// and signals of other rules never move.
func rankMetricRulesWithinTiers(signals []Signal) {
	positions := make(map[peerKey][]int)
	for i, signal := range signals {
		if _, ok := signalMagnitude(signal); !ok {
			continue
		}
		key := peerKey{
			group:      signalPriorityGroup(signal),
			severity:   severityRank(signal.Severity),
			confidence: confidenceRank(signal.Confidence),
			ruleID:     signal.RuleID,
		}
		positions[key] = append(positions[key], i)
	}
	reordered := slices.Clone(signals)
	for _, peers := range positions {
		for i, signal := range rankedByMagnitude(signals, peers) {
			reordered[peers[i]] = signal
		}
	}
	copy(signals, reordered)
}

func rankedByMagnitude(signals []Signal, positions []int) []Signal {
	ranked := make([]rankedSignal, len(positions))
	for i, position := range positions {
		ratio, _ := signalMagnitude(signals[position])
		ranked[i] = rankedSignal{signal: signals[position], ratio: ratio}
	}
	slices.SortStableFunc(ranked, func(a, b rankedSignal) int {
		return cmp.Compare(b.ratio, a.ratio)
	})
	out := make([]Signal, len(ranked))
	for i, entry := range ranked {
		out[i] = entry.signal
	}
	return out
}
