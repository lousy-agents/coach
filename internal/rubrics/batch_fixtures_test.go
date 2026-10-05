package rubrics_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// blockUntilCancelGateway blocks Judge until ctx is done (wall-budget tests).
type blockUntilCancelGateway struct{}

func (blockUntilCancelGateway) Judge(ctx context.Context, _ modelgateway.JudgmentRequest) (modelgateway.JudgmentResponse, error) {
	<-ctx.Done()
	return modelgateway.JudgmentResponse{}, modelgateway.NewUnavailableError("context done", ctx.Err())
}

// fixedBatchGateway returns a canned JudgmentJSON (used for partial-pack cases).
type fixedBatchGateway struct {
	judgment json.RawMessage
	mu       sync.Mutex
	reqs     []modelgateway.JudgmentRequest
}

func (g *fixedBatchGateway) Judge(ctx context.Context, req modelgateway.JudgmentRequest) (modelgateway.JudgmentResponse, error) {
	if err := ctx.Err(); err != nil {
		return modelgateway.JudgmentResponse{}, modelgateway.NewUnavailableError("context done", err)
	}
	cloned := modelgateway.JudgmentRequest{
		RubricID:      req.RubricID,
		RubricVersion: req.RubricVersion,
		LogicalModel:  req.LogicalModel,
		OutputSchema:  append(json.RawMessage(nil), req.OutputSchema...),
		Messages:      append([]modelgateway.Message(nil), req.Messages...),
	}
	g.mu.Lock()
	g.reqs = append(g.reqs, cloned)
	g.mu.Unlock()
	return modelgateway.JudgmentResponse{
		JudgmentJSON:   append(json.RawMessage(nil), g.judgment...),
		LogicalModelID: modelgateway.LogicalModelStub,
	}, nil
}

func (g *fixedBatchGateway) requests() []modelgateway.JudgmentRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]modelgateway.JudgmentRequest, len(g.reqs))
	copy(out, g.reqs)
	return out
}

func sampleFileContent(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "line-%d-content\n", i)
	}
	return b.String()
}

func packItemArgs(ref, path string, startRow int, content string) map[string]any {
	return map[string]any{
		"finding_ref": ref,
		"finding": map[string]any{
			"rule_id":   "state.hidden_input_mutation",
			"kind":      "hidden_input_mutation",
			"path":      path,
			"subject":   "NewService",
			"start_row": startRow,
		},
		"file": map[string]any{
			"path":     path,
			"language": "go",
			"content":  content,
		},
	}
}

func decodePackResults(raw json.RawMessage) rubrics.ToolPackResult {
	GinkgoHelper()
	pack, err := rubrics.ParseToolPackResult(raw)
	Expect(err).NotTo(HaveOccurred())
	return pack
}
