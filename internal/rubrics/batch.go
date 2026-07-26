package rubrics

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

// ToolPackResult is the JSON envelope returned by multi-finding pack tool calls
// for hidden_mutation_contextualization. Task 3 (coachapi handler) should parse
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
// when a pack contains multiple findings (Story 1).
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

// AssembleHiddenMutationPackMessages builds gateway Messages for a multi-finding
// pack with short-rationale guidance (Story 3) and per-item span-local evidence.
func AssembleHiddenMutationPackMessages(ev HiddenMutationPackEvidence) []modelgateway.Message {
	var b strings.Builder
	b.WriteString("## Hidden-mutation judgment pack\n")
	b.WriteString("Judge each item independently. Map every input finding_ref to exactly one output item.\n")
	b.WriteString(shortRationaleGuidance)
	b.WriteByte('\n')
	for _, it := range ev.Items {
		b.WriteString("\n### Item\n")
		b.WriteString("finding_ref: ")
		b.WriteString(it.FindingRef)
		b.WriteByte('\n')
		b.WriteString("## Deterministic finding (hidden_input_mutation)\n")
		b.WriteString(formatJSONEvidence(it.Finding))
		b.WriteString("\n\n## Baseline file context\n")
		b.WriteString(fmt.Sprintf("path: %s\n", it.File.Path))
		b.WriteString(fmt.Sprintf("language: %s\n", it.File.Language))
		if it.File.Content != "" {
			b.WriteString("content (span window):\n```\n")
			b.WriteString(it.File.Content)
			if !strings.HasSuffix(it.File.Content, "\n") {
				b.WriteByte('\n')
			}
			b.WriteString("```\n")
		}
	}
	return []modelgateway.Message{
		{Role: "system", Content: hiddenMutationPackSystemPrompt},
		{Role: "user", Content: b.String()},
	}
}

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
