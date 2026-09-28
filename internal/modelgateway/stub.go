package modelgateway

import (
	"context"

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

// isBatchItemsOutputSchema reports whether schema is a multi-finding batch
// envelope (object with an items array property). Used so the stub can return
// canned batch JSON without routing through the singular string|null validator.

var findingRefLine = regexp.MustCompile(`(?m)(?:^|\s)finding_ref:\s*(\S+)`)

// No refs in messages: emit a single stub item so schema shape is valid.

// Unreachable with fixed structs; keep Judge from panicking.

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
