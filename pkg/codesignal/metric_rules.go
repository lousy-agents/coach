package codesignal

import (
	"strconv"
	"strings"
)

// highSeverityThresholdMultiple is the multiple of an escalating metric rule's
// threshold at which its signal becomes high severity.
const highSeverityThresholdMultiple = 2

// metricRule describes a rule whose Evidence carries a numeric metric measured
// against a threshold. Rule constructors take rule ID, threshold, evidence, and
// severity from it and the sort reads magnitude through it, so a rule cannot
// escalate without ranking or rank without escalating by accident.
type metricRule struct {
	ruleID         string
	evidencePrefix string
	// threshold is the minimum metric that produces a signal; it is also the
	// unit magnitude is measured in.
	threshold int
	// escalates is false for a metric that is a whole-file additive total:
	// it grows with file length and a split clears it, so crossing twice the
	// threshold says nothing about how hard the code is to follow.
	escalates bool
}

var (
	cognitiveComplexityRule = metricRule{
		ruleID:         "complexity.cognitive_complexity",
		evidencePrefix: "cognitive_complexity=",
		threshold:      15,
		escalates:      true,
	}
	branchDensityRule = metricRule{
		ruleID:         "complexity.branch_density",
		evidencePrefix: "branch_sum=",
		threshold:      12,
	}
	maxNestingDepthRule = metricRule{
		ruleID:         "complexity.max_nesting_depth",
		evidencePrefix: "max_nesting_depth=",
		threshold:      4,
		escalates:      true,
	}
)

var metricRules = []metricRule{cognitiveComplexityRule, branchDensityRule, maxNestingDepthRule}

func (r metricRule) reaches(metric int) bool {
	return metric >= r.threshold
}

func (r metricRule) severity(metric int) Severity {
	if r.escalates && metric >= highSeverityThresholdMultiple*r.threshold {
		return "high"
	}
	return "medium"
}

func (r metricRule) evidence(metric int) string {
	return r.evidencePrefix + strconv.Itoa(metric)
}

// magnitude returns the metric as a multiple of the rule threshold. Reading it
// from Evidence rather than storing it on Signal lets it survive lifecycle
// copies and leaves the JSON shape and fingerprints untouched.
func (r metricRule) magnitude(sig Signal) (ratio float64, ok bool) {
	value, found := strings.CutPrefix(sig.Evidence, r.evidencePrefix)
	if !found {
		return 0, false
	}
	metric, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return float64(metric) / float64(r.threshold), true
}

// signalMagnitude returns the signal's metric as a multiple of its rule
// threshold, or ok=false when the rule has no numeric magnitude.
func signalMagnitude(sig Signal) (ratio float64, ok bool) {
	for _, rule := range metricRules {
		if rule.ruleID == sig.RuleID {
			return rule.magnitude(sig)
		}
	}
	return 0, false
}
