package evalharness

// gateRecord is what the manifest holds of a gate that ran: the verdicts of both suites, the lowest score and the
// highest not-covered share, and whether neither suite had a valid mutant, with the reason when so.
func gateRecord(runs map[string]any, scores, shares []float64, valid int) map[string]any {
	fields := map[string]any{
		"mutation_checked": true, "mutation_runs": runs,
		"mutation_score": lowest(scores), "not_covered_share": highest(shares),
		"mutation_vacuous": valid == 0,
	}
	if valid == 0 {
		fields["mutation_vacuous_reason"] = vacuousReason
	}
	return fields
}

// vacuousReason is recorded when neither suite has a valid mutant, so that a reader who sees mutation_checked does not
// take the score for a measurement of the tests' strength.
const vacuousReason = "no valid mutant in either suite: the program and the oracle solution have no mutable code in the mutation scope"

func verdictFields(verdict MutationVerdict) map[string]any {
	return map[string]any{"valid": verdict.Valid, "score": verdict.Score, "not_covered_share": verdict.NotCoveredShare}
}

func lowest(values []float64) any {
	if len(values) == 0 {
		return nil
	}
	lowest := values[0]
	for _, value := range values {
		lowest = min(lowest, value)
	}
	return lowest
}

func highest(values []float64) any {
	if len(values) == 0 {
		return nil
	}
	highest := values[0]
	for _, value := range values {
		highest = max(highest, value)
	}
	return highest
}
