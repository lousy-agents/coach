package rubrics

import (
	"encoding/json"
	"fmt"
)

// ToolPackResult is the JSON envelope returned by multi-finding pack tool calls
// for hidden_mutation_contextualization. The coachapi handler should parse
// this via ParseToolPackResult and map each Results[i] like a singular
// ToolResult, using FindingRef as the hash discriminator for PayloadHash.
type ToolPackResult struct {
	Results []ToolResult `json:"results"`
}

// HiddenMutationPackItem is one finding + file evidence in a pack tool args list.
type HiddenMutationPackItem struct {
	FindingRef string          `json:"finding_ref"`
	Finding    json.RawMessage `json:"finding"`
	File       FileContext     `json:"file"`
}

// HiddenMutationPackEvidence is multi-finding input for pack judgment.
type HiddenMutationPackEvidence struct {
	Items []HiddenMutationPackItem
}

// batchItemJudgment is one element of the model batch envelope.
type batchItemJudgment struct {
	FindingRef     string          `json:"finding_ref"`
	Judgment       string          `json:"judgment"`
	Rationale      string          `json:"rationale"`
	Confidence     string          `json:"confidence"`
	SuggestedFocus json.RawMessage `json:"suggested_focus"`
}

// HiddenMutationBatchOutputSchema returns the batch envelope OutputSchema used
// when a pack contains multiple findings.
func HiddenMutationBatchOutputSchema() json.RawMessage {
	return mustSchema(schemaHiddenMutationBatchV1)
}

// ParseToolPackResult decodes a pack tool Call response envelope.
func ParseToolPackResult(raw json.RawMessage) (ToolPackResult, error) {
	if !IsToolPackResult(raw) {
		return ToolPackResult{}, fmt.Errorf("rubrics: not a tool pack result envelope")
	}
	var pack ToolPackResult
	if err := json.Unmarshal(raw, &pack); err != nil {
		return ToolPackResult{}, fmt.Errorf("rubrics: decoding tool pack result: %w", err)
	}
	return pack, nil
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

func marshalToolPackResult(p ToolPackResult) (json.RawMessage, error) {
	type itemWire struct {
		FindingRef     string          `json:"finding_ref,omitempty"`
		RubricID       string          `json:"rubric_id"`
		RubricVersion  string          `json:"rubric_version"`
		ModelIdentity  *string         `json:"model_identity"`
		LogicalModelID *string         `json:"logical_model_id,omitempty"`
		ServedModelID  *string         `json:"served_model_id,omitempty"`
		Judgment       json.RawMessage `json:"judgment"`
		Diagnostic     *Diagnostic     `json:"diagnostic"`
	}
	wire := struct {
		Results []itemWire `json:"results"`
	}{
		Results: make([]itemWire, len(p.Results)),
	}
	for i, r := range p.Results {
		w := itemWire{
			FindingRef:     r.FindingRef,
			RubricID:       r.RubricID,
			RubricVersion:  r.RubricVersion,
			ModelIdentity:  r.ModelIdentity,
			LogicalModelID: r.LogicalModelID,
			ServedModelID:  r.ServedModelID,
			Diagnostic:     r.Diagnostic,
		}
		if len(r.Judgment) == 0 {
			w.Judgment = json.RawMessage("null")
		} else {
			w.Judgment = r.Judgment
		}
		wire.Results[i] = w
	}
	return json.Marshal(wire)
}
