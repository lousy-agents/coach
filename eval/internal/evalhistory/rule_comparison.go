package evalhistory

// placedFinding is a finding in the unit holding it, with the first fix
// that touched that unit ("" for none).
type placedFinding struct {
	finding
	level Level
	fix   string
}

// compareRule compares ruleID's flagged units with controls drawn from the
// unflagged units at the same level.
func compareRule(inventory Inventory, involved involvements, findings []finding, ruleID string, seed int64) (RuleResult, []placedFinding, error) {
	flagged, err := FlaggedUnits(inventory, signalsOf(findings), ruleID)
	if err != nil {
		return RuleResult{}, nil, err
	}
	pool, involvement := inventory.Functions, involved.functions
	if flagged.Level == FileLevel {
		pool, involvement = inventory.Files, involved.files
	}
	draw := DrawControls(pool, flagged.Units, ControlsPerFlagged, seed)
	result := RuleResult{
		RuleID: ruleID, Level: flagged.Level,
		Flagged: len(flagged.Units), FlaggedInvolved: countInvolved(involvement, flagged.Units),
		Controls: len(draw.Controls), ControlsInvolved: countInvolved(involvement, draw.Controls),
		Shortfall: draw.Shortfall, Unplaced: flagged.Unplaced, UnplacedReasons: flagged.UnplacedReasons,
	}
	association := RiskRatio(result.FlaggedInvolved, result.Flagged, result.ControlsInvolved, result.Controls)
	result.RiskRatio, result.Low, result.High, result.Reason = association.RiskRatio, association.Low, association.High, association.Reason
	return result, placeFindings(inventory, involvement, findings, ruleID, flagged.Level), nil
}

func countInvolved(involvement Involvement, units []int) int {
	count := 0
	for _, index := range units {
		if involvement.FixSHA[index] != "" {
			count++
		}
	}
	return count
}

func placeFindings(inventory Inventory, involvement Involvement, findings []finding, ruleID string, level Level) []placedFinding {
	var placed []placedFinding
	for _, finding := range findings {
		if finding.RuleID != ruleID {
			continue
		}
		if index, ok := placeSignal(inventory, finding.Signal, level); ok {
			placed = append(placed, placedFinding{finding: finding, level: level, fix: involvement.FixSHA[index]})
		}
	}
	return placed
}
