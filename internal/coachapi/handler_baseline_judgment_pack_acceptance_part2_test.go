package coachapi_test

import (
	"context"
	"encoding/json"

	"os"
	"path/filepath"

	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

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

func countHiddenMutationToolCalls(loops []*agentloop.Loop) int {
	n := 0
	for _, loop := range loops {
		if loop == nil {
			continue
		}
		(&sigcountHiddenMutationToolCallsS1{loop: loop, n: &n}).call()

	}
	return n
}

func newRecordingJudgeGateway(inner modelgateway.Gateway) *recordingJudgeGateway {
	if inner == nil {
		inner = modelgateway.NewStubGateway()
	}
	return &recordingJudgeGateway{inner: inner}
}

// multiHiddenMutationFixtureRoot builds a temp tree with ≥12 hidden_input_mutation
// signals across ≥3 paths and one hot path with ≥6 signals (Story 1 pack fixture).
func multiHiddenMutationFixtureRoot() string {
	GinkgoHelper()
	root := GinkgoT().TempDir()

	hot := `package hot

type S struct {
	A, B, C, D, E, F, G, H string
}

func M1(s *S, v string) { s.A = v }
func M2(s *S, v string) { s.B = v }
func M3(s *S, v string) { s.C = v }
func M4(s *S, v string) { s.D = v }
func M5(s *S, v string) { s.E = v }
func M6(s *S, v string) { s.F = v }
func M7(s *S, v string) { s.G = v }
func M8(s *S, v string) { s.H = v }
`

	coldA := `package colda

type S struct {
	X, Y, Z string
}

func A1(s *S, v string) { s.X = v }
func A2(s *S, v string) { s.Y = v }
func A3(s *S, v string) { s.Z = v }
`
	coldB := `package coldb

type S struct {
	X, Y, Z string
}

func B1(s *S, v string) { s.X = v }
func B2(s *S, v string) { s.Y = v }
func B3(s *S, v string) { s.Z = v }
`
	Expect(os.WriteFile(filepath.Join(root, "hot.go"), []byte(hot), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "cold_a.go"), []byte(coldA), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "cold_b.go"), []byte(coldB), 0o644)).To(Succeed())
	return root
}

func (g *recordingJudgeGateway) requests() []modelgateway.JudgmentRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]modelgateway.JudgmentRequest, len(g.reqs))
	copy(out, g.reqs)
	return out
}
