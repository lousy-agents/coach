package evalhistory

// CompareRulesFor compares ruleIDs against an empty inventory and returns the error the comparison gives. It lets a
// spec reach the comparison's own refusal, which the repository's earlier check otherwise hides.
func CompareRulesFor(ruleIDs []string) error {
	_, _, err := compareRules(ruleIDs, Inventory{}, involvements{}, nil, 0)
	return err
}
