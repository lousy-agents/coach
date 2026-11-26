package evalhistory

import (
	"slices"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// goRuleIDs returns the Go rules the corpus names. A rule the placement table does not hold refuses the repository,
// naming it, before any rule is compared, so no rule is written for the repository while another is refused.
func goRuleIDs(corpus evalcensus.Corpus) ([]string, error) {
	var ids []string
	for _, rule := range corpus.Rules {
		if !slices.Contains(rule.Languages, "go") {
			continue
		}
		if _, err := ruleLevel(rule.RuleID); err != nil {
			return nil, err
		}
		ids = append(ids, rule.RuleID)
	}
	return ids, nil
}

// compareRules compares each of ruleIDs in order.
func compareRules(ruleIDs []string, inventory Inventory, involved involvements, findings []finding, seed int64) ([]RuleResult, []placedFinding, error) {
	var results []RuleResult
	var placed []placedFinding
	for _, ruleID := range ruleIDs {
		ruleResult, found, err := compareRule(inventory, involved, findings, ruleID, seed)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, ruleResult)
		placed = append(placed, found...)
	}
	return results, placed, nil
}
