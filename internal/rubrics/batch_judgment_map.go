package rubrics

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

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
