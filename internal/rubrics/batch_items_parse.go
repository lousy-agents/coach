package rubrics

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseBatchItems(raw json.RawMessage) (map[string]batchItemJudgment, map[string]string) {
	byRef := make(map[string]batchItemJudgment)
	var env struct {
		Items []batchItemJudgment `json:"items"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || env.Items == nil {
		return byRef, nil
	}
	dup := make(map[string]string)
	for _, it := range env.Items {
		if it.FindingRef == "" {
			continue
		}
		if _, exists := byRef[it.FindingRef]; exists {
			dup[it.FindingRef] = "batch response has duplicate finding_ref"
			delete(byRef, it.FindingRef)
			continue
		}
		if _, marked := dup[it.FindingRef]; marked {
			continue
		}
		byRef[it.FindingRef] = it
	}
	return byRef, dup
}

func validateBatchItem(it batchItemJudgment) error {
	switch it.Judgment {
	case "concern", "acceptable", "unclear":
	default:
		return fmt.Errorf("schema validation failed: judgment value not in enum")
	}
	switch it.Confidence {
	case "high", "medium", "low":
	default:
		return fmt.Errorf("schema validation failed: confidence value not in enum")
	}
	if strings.TrimSpace(it.Rationale) == "" {
		return fmt.Errorf("schema validation failed: rationale must be a non-empty string")
	}
	if len(it.SuggestedFocus) == 0 {
		return fmt.Errorf("schema validation failed: missing required property: suggested_focus")
	}
	// suggested_focus: string | null
	if string(it.SuggestedFocus) != "null" {
		var s string
		if err := json.Unmarshal(it.SuggestedFocus, &s); err != nil {
			return fmt.Errorf("schema validation failed: suggested_focus must be string or null")
		}
	}
	return nil
}
