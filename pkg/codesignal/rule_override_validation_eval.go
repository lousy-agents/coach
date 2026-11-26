//go:build coach_eval_override

package codesignal

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const overrideMaxThresholdMultiple = 10

// overrideSurface maps each overrideable threshold key to its compiled value.
var overrideSurface = map[string]int{
	cognitiveComplexityRule.ruleID:                     cognitiveComplexityRule.threshold,
	branchDensityRule.ruleID:                           branchDensityRule.threshold,
	maxNestingDepthRule.ruleID:                         maxNestingDepthRule.threshold,
	deepRelativeImportRuleID:                           deepRelativeImportDepthThreshold,
	gatedRuleIDsByKind["constructor_func"]:             densityGateThreshold,
	gatedRuleIDsByKind["pointer_return"]:               densityGateThreshold,
	reactOrchestrationDensityRuleID + ".state_domains": reactStateDomainMinUnique,
	reactOrchestrationDensityRuleID + ".transitions":   reactStateDomainMinTransitions,
	reactOrchestrationDensityRuleID + ".branches":      reactStateDomainMinBranches,
}

var disableableRuleIDs = []string{
	cognitiveComplexityRule.ruleID,
	branchDensityRule.ruleID,
	maxNestingDepthRule.ruleID,
	deepRelativeImportRuleID,
	gatedRuleIDsByKind["constructor_func"],
	gatedRuleIDsByKind["pointer_return"],
	reactOrchestrationDensityRuleID,
	"state.hidden_input_mutation",
	"coupling.tight_constructor_init",
	"security.toctou_check_then_act",
}

func validatedThresholds(raw map[string]json.RawMessage) (map[string]int, error) {
	thresholds := make(map[string]int, len(raw))
	for key, literal := range raw {
		compiled, known := overrideSurface[key]
		if !known {
			return nil, fmt.Errorf("threshold %q is not in the override surface", key)
		}
		value, err := strconv.Atoi(string(literal))
		if err != nil {
			return nil, fmt.Errorf("threshold %q must be an integer, got %s", key, literal)
		}
		if value < 1 || value > overrideMaxThresholdMultiple*compiled {
			return nil, fmt.Errorf("threshold %q must be from 1 to %d, got %d", key, overrideMaxThresholdMultiple*compiled, value)
		}
		thresholds[key] = value
	}
	return thresholds, nil
}

func validatedDisabledRules(ruleIDs []string) (map[string]bool, error) {
	allowed := make(map[string]bool, len(disableableRuleIDs))
	for _, id := range disableableRuleIDs {
		allowed[id] = true
	}
	disabled := make(map[string]bool, len(ruleIDs))
	for _, id := range ruleIDs {
		if !allowed[id] {
			return nil, fmt.Errorf("disable %q is not a rule the override can disable", id)
		}
		disabled[id] = true
	}
	return disabled, nil
}
