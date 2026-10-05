package codesignal

import (
	"slices"
	"testing"
)

func TestMetricRuleTableIsNonEmptyWithUniqueRuleIDs(t *testing.T) {
	if len(metricRules) == 0 {
		t.Fatal("metric rule table is empty")
	}
	ruleIDs := make([]string, len(metricRules))
	for i, rule := range metricRules {
		ruleIDs[i] = rule.ruleID
	}
	slices.Sort(ruleIDs)
	if unique := slices.Compact(slices.Clone(ruleIDs)); len(unique) != len(ruleIDs) {
		t.Fatalf("metric rule IDs contain a duplicate: %v", ruleIDs)
	}
}
