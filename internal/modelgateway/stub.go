package modelgateway

import (
	"context"
	"encoding/json"
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
