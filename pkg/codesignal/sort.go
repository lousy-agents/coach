package codesignal

import (
	"sort"
	"strconv"
	"strings"
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

type magnitudeRule struct {
	evidencePrefix string
	threshold      int
}

// magnitudeRules lists the rules whose Evidence carries a numeric metric
// measured against a threshold. Magnitude is read from Evidence rather than
// stored on Signal so it survives lifecycle copies and leaves the JSON shape
// and fingerprints untouched.
var magnitudeRules = map[string]magnitudeRule{
	"complexity.cognitive_complexity": {"cognitive_complexity=", cognitiveComplexityThreshold},
	"complexity.branch_density":       {"branch_sum=", branchDensityThreshold},
	"complexity.max_nesting_depth":    {"max_nesting_depth=", maxNestingDepthThreshold},
}

// signalMagnitude returns the signal's metric as a multiple of its rule
// threshold, or ok=false when the rule has no numeric magnitude.
func signalMagnitude(sig Signal) (ratio float64, ok bool) {
	rule, known := magnitudeRules[sig.RuleID]
	if !known {
		return 0, false
	}
	value, found := strings.CutPrefix(sig.Evidence, rule.evidencePrefix)
	if !found {
		return 0, false
	}
	metric, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return float64(metric) / float64(rule.threshold), true
}

// sortSignals sorts signals by priority group, severity, confidence,
// magnitude (signals with a magnitude first, larger ratio first), path,
// location, rule, and ID.
func sortSignals(signals []Signal) {
	sort.SliceStable(signals, func(i, j int) bool {
		a, b := signals[i], signals[j]

		ga, gb := signalPriorityGroup(a), signalPriorityGroup(b)
		if ga != gb {
			return ga < gb
		}
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra > rb
		}
		if ra, rb := confidenceRank(a.Confidence), confidenceRank(b.Confidence); ra != rb {
			return ra > rb
		}
		ma, hasMagnitudeA := signalMagnitude(a)
		mb, hasMagnitudeB := signalMagnitude(b)
		if hasMagnitudeA != hasMagnitudeB {
			return hasMagnitudeA
		}
		if hasMagnitudeA && ma != mb {
			return ma > mb
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Location.StartRow != b.Location.StartRow {
			return a.Location.StartRow < b.Location.StartRow
		}
		if a.Location.StartCol != b.Location.StartCol {
			return a.Location.StartCol < b.Location.StartCol
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.ID < b.ID
	})
}
