package rubrics

import (
	"encoding/json"
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

// IsToolPackResult reports whether raw is a multi-item pack results envelope
// ({"results":[...]}). Singular ToolResult envelopes return false.
func IsToolPackResult(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var probe struct {
		Results json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	if len(probe.Results) == 0 {
		return false
	}
	// Discriminate from accidental singular payloads that might gain a results field later:
	// pack envelopes always use a JSON array for results.
	var arr []json.RawMessage
	return json.Unmarshal(probe.Results, &arr) == nil
}
func packResultsForGatewayDegrade(def Definition, refs []string, r Result) ToolPackResult {
	msg := "judgment failed: empty result"
	if r.Diagnostic != nil && r.Diagnostic.Message != "" {
		msg = r.Diagnostic.Message
	}
	results := make([]ToolResult, 0, len(refs))
	for _, ref := range refs {
		results = append(results, packItemDiagnostic(def, ref, msg))
	}
	return ToolPackResult{Results: results}
}
