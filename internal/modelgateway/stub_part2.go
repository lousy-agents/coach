package modelgateway

import (
	"encoding/json"

	"strings"
)

// isBatchItemsOutputSchema reports whether schema is a multi-finding batch
// envelope (object with an items array property). Used so the stub can return
// canned batch JSON without routing through the singular string|null validator.
func isBatchItemsOutputSchema(schema json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	var sch struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &sch); err != nil {
		return false
	}
	raw, ok := sch.Properties["items"]
	if !ok || len(raw) == 0 {
		return false
	}
	var itemsProp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &itemsProp); err != nil {
		return false
	}
	return strings.EqualFold(itemsProp.Type, "array")
}
func stubBatchJudgment(req JudgmentRequest) json.RawMessage {
	refs := extractFindingRefsFromMessages(req.Messages)
	if len(refs) == 0 {

		refs = []string{"stub-item-1"}
	}
	type item struct {
		FindingRef     string  `json:"finding_ref"`
		Judgment       string  `json:"judgment"`
		Rationale      string  `json:"rationale"`
		Confidence     string  `json:"confidence"`
		SuggestedFocus *string `json:"suggested_focus"`
	}
	items := make([]item, 0, len(refs))
	for _, ref := range refs {
		items = append(items, item{
			FindingRef:     ref,
			Judgment:       "acceptable",
			Rationale:      "stub: batch item for " + ref,
			Confidence:     "high",
			SuggestedFocus: nil,
		})
	}
	raw, err := json.Marshal(map[string]any{"items": items})
	if err != nil {

		return json.RawMessage(`{"items":[]}`)
	}
	return raw
}
func stubJudgmentForRubric(rubricID string) (json.RawMessage, bool) {
	switch rubricID {
	case "hidden_mutation", "hidden_mutation_contextualization":
		return json.RawMessage(`{
			"judgment": "acceptable",
			"rationale": "stub: no hidden mutation signals in fixture judgment",
			"confidence": "high",
			"suggested_focus": null
		}`), true
	case "change_cohesion":
		return json.RawMessage(`{
			"judgment": "focused",
			"rationale": "stub: change appears cohesive in fixture judgment",
			"confidence": "medium",
			"suggested_focus": null
		}`), true
	default:
		return nil, false
	}
}

// NewStubGateway returns a deterministic StubGateway. With no options it serves
// canned schema-valid judgments; StubOptions.JudgeErr forces a typed error path.
func NewStubGateway(opts ...StubOptions) *StubGateway {
	g := &StubGateway{}
	if len(opts) > 0 {
		g.judgeErr = opts[0].JudgeErr
	}
	return g
}
