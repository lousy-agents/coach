package modelgateway

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

// StubOptions configures optional StubGateway behavior (tests may inject typed errors).
type StubOptions struct {
	// JudgeErr, when non-nil, is returned from every Judge call.
	JudgeErr error
}

// StubGateway is the default deterministic Gateway: for rubric-judgment requests
// it returns canned schema-valid judgments from fixed fixtures. It is
// judgment-oriented, not a full agent script engine — scripted tool-call
// sequences stay in internal/acceptanceharness/agentloopharness.ScriptedGateway.
type StubGateway struct {
	judgeErr error
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

func (g *StubGateway) Judge(ctx context.Context, req JudgmentRequest) (JudgmentResponse, error) {
	if err := ctx.Err(); err != nil {
		return JudgmentResponse{}, NewUnavailableError("context done", err)
	}
	if g != nil && g.judgeErr != nil {
		return JudgmentResponse{}, g.judgeErr
	}

	if isBatchItemsOutputSchema(req.OutputSchema) {
		judgment := stubBatchJudgment(req)
		return JudgmentResponse{
			JudgmentJSON:   judgment,
			LogicalModelID: LogicalModelStub,
		}, nil
	}

	judgment, ok := stubJudgmentForRubric(req.RubricID)
	if !ok {
		return JudgmentResponse{}, NewValidationError("unknown rubric_id: " + req.RubricID)
	}
	if err := validateJudgmentJSON(judgment, req.OutputSchema); err != nil {
		return JudgmentResponse{}, err
	}

	return JudgmentResponse{
		JudgmentJSON:   judgment,
		LogicalModelID: LogicalModelStub,
	}, nil
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

var findingRefLine = regexp.MustCompile(`(?m)(?:^|\s)finding_ref:\s*(\S+)`)

func stubBatchJudgment(req JudgmentRequest) json.RawMessage {
	refs := extractFindingRefsFromMessages(req.Messages)
	if len(refs) == 0 {
		// No refs in messages: emit a single stub item so schema shape is valid.
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
		// Unreachable with fixed structs; keep Judge from panicking.
		return json.RawMessage(`{"items":[]}`)
	}
	return raw
}

func extractFindingRefsFromMessages(msgs []Message) []string {
	var refs []string
	seen := make(map[string]struct{})
	for _, m := range msgs {
		for _, match := range findingRefLine.FindAllStringSubmatch(m.Content, -1) {
			if len(match) < 2 {
				continue
			}
			ref := strings.TrimSpace(match[1])
			if ref == "" {
				continue
			}
			if _, ok := seen[ref]; ok {
				continue
			}
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	return refs
}
