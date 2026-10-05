package rubrics

import (
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/modelgateway"
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

// Discriminate from accidental singular payloads that might gain a results field later:
// pack envelopes always use a JSON array for results.

// AssembleHiddenMutationPackMessages builds gateway Messages for a multi-finding
// pack with short-rationale guidance and per-item span-local evidence.

// mapBatchJudgmentToPackResult maps a batch JudgmentJSON envelope onto one
// ToolResult per expected finding_ref. Missing/invalid items become diagnostics
// for that ref only (partial pack success).
func mapBatchJudgmentToPackResult(def Definition, expectedRefs []string, resp modelgateway.JudgmentResponse) ToolPackResult {
	byRef, parseDiags := parseBatchItems(resp.JudgmentJSON)
	identity := FormatModelIdentity(resp.LogicalModelID, resp.ServedModelID)
	logical := resp.LogicalModelID
	var served *string
	if resp.ServedModelID != "" {
		s := resp.ServedModelID
		served = &s
	}

	results := make([]ToolResult, 0, len(expectedRefs))
	seen := make(map[string]struct{}, len(expectedRefs))
	for _, ref := range expectedRefs {
		if ref == "" {
			continue
		}
		if _, dup := seen[ref]; dup {
			results = append(results, packItemDiagnostic(def, ref, "duplicate finding_ref in pack args"))
			continue
		}
		seen[ref] = struct{}{}

		if msg, bad := parseDiags[ref]; bad {
			results = append(results, packItemDiagnostic(def, ref, msg))
			continue
		}
		item, ok := byRef[ref]
		if !ok {
			results = append(results, packItemDiagnostic(def, ref, "batch response missing finding_ref"))
			continue
		}
		if err := validateBatchItem(item); err != nil {
			results = append(results, packItemDiagnostic(def, ref, err.Error()))
			continue
		}
		judgmentJSON, err := marshalBatchItemJudgment(item)
		if err != nil {
			results = append(results, packItemDiagnostic(def, ref, "failed to encode item judgment"))
			continue
		}
		id := identity
		log := logical
		results = append(results, ToolResult{
			FindingRef:     ref,
			RubricID:       def.ID,
			RubricVersion:  def.Version,
			ModelIdentity:  &id,
			LogicalModelID: &log,
			ServedModelID:  served,
			Judgment:       judgmentJSON,
		})
	}
	return ToolPackResult{Results: results}
}

// suggested_focus: string | null

func marshalBatchItemJudgment(it batchItemJudgment) (json.RawMessage, error) {
	// Emit singular v1 shape (no finding_ref) so payload matches seed schema consumers.
	type wire struct {
		Judgment       string          `json:"judgment"`
		Rationale      string          `json:"rationale"`
		Confidence     string          `json:"confidence"`
		SuggestedFocus json.RawMessage `json:"suggested_focus"`
	}
	return json.Marshal(wire{
		Judgment:       it.Judgment,
		Rationale:      it.Rationale,
		Confidence:     it.Confidence,
		SuggestedFocus: it.SuggestedFocus,
	})
}

func packItemDiagnostic(def Definition, ref, message string) ToolResult {
	return ToolResult{
		FindingRef:    ref,
		RubricID:      def.ID,
		RubricVersion: def.Version,
		Diagnostic: &Diagnostic{
			Scope:   diagnosticScope(def.ID),
			Message: message,
		},
	}
}
