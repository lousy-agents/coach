package baseline_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

// recordingJudgeGateway records every Judge call and optionally delays or
// fails after N successful judgments (budget / packing acceptance).
type recordingJudgeGateway struct {
	inner modelgateway.Gateway

	mu        sync.Mutex
	reqs      []modelgateway.JudgmentRequest
	callCount atomic.Int32

	// delay is applied before each successful Judge (slow fake gateway).
	delay time.Duration

	// failAfterN, when > 0, returns ErrBudgetExceeded-style unavailability after
	// N successful Judge calls (used with short judgment wall via real sleep+wall).
	// When 0, all calls delegate to inner.
	//
	// For pack-budget tests we instead use a short MaxWallTime + delay so the
	// agentloop surfaces agentloop.ErrBudgetExceeded mid-phase.
	failAfterN int
	failErr    error

	// blockAfterN, when > 0, blocks on ctx.Done() for call number > N (1-based)
	// after recording the request. Used to force wall expiry on a specific pack.
	blockAfterN int

	// fixedJudgment, when non-nil, is returned instead of inner for non-blocking calls.
	fixedJudgment json.RawMessage
}

func newRecordingJudgeGateway(inner modelgateway.Gateway) *recordingJudgeGateway {
	if inner == nil {
		inner = modelgateway.NewStubGateway()
	}
	return &recordingJudgeGateway{inner: inner}
}

func (g *recordingJudgeGateway) Judge(ctx context.Context, req modelgateway.JudgmentRequest) (modelgateway.JudgmentResponse, error) {
	cloned := modelgateway.JudgmentRequest{
		RubricID:      req.RubricID,
		RubricVersion: req.RubricVersion,
		LogicalModel:  req.LogicalModel,
		OutputSchema:  append(json.RawMessage(nil), req.OutputSchema...),
		Messages:      append([]modelgateway.Message(nil), req.Messages...),
	}
	g.mu.Lock()
	g.reqs = append(g.reqs, cloned)
	n := len(g.reqs)
	g.mu.Unlock()
	g.callCount.Add(1)

	if g.failAfterN > 0 && n > g.failAfterN {
		err := g.failErr
		if err == nil {
			err = modelgateway.NewUnavailableError("injected gateway stop after pack budget", nil)
		}
		return modelgateway.JudgmentResponse{}, err
	}
	if g.blockAfterN > 0 && n > g.blockAfterN {
		<-ctx.Done()
		return modelgateway.JudgmentResponse{}, modelgateway.NewUnavailableError("context done", ctx.Err())
	}
	if g.delay > 0 {
		select {
		case <-ctx.Done():
			return modelgateway.JudgmentResponse{}, modelgateway.NewUnavailableError("context done", ctx.Err())
		case <-time.After(g.delay):
		}
	}
	if len(g.fixedJudgment) > 0 {
		return modelgateway.JudgmentResponse{
			JudgmentJSON:   append(json.RawMessage(nil), g.fixedJudgment...),
			LogicalModelID: modelgateway.LogicalModelStub,
		}, nil
	}
	return g.inner.Judge(ctx, req)
}

func (g *recordingJudgeGateway) requests() []modelgateway.JudgmentRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]modelgateway.JudgmentRequest, len(g.reqs))
	copy(out, g.reqs)
	return out
}
