package rubrics_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// recordingGateway is a test double that records every JudgmentRequest before
// delegating to an inner Gateway. Used to lock OutputSchema and Messages on the
// success path (StubGateway skips schema checks when OutputSchema is empty).
type recordingGateway struct {
	inner modelgateway.Gateway
	mu    sync.Mutex
	reqs  []modelgateway.JudgmentRequest
}

func newRecordingGateway(inner modelgateway.Gateway) *recordingGateway {
	return &recordingGateway{inner: inner}
}

func (g *recordingGateway) Judge(ctx context.Context, req modelgateway.JudgmentRequest) (modelgateway.JudgmentResponse, error) {
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
	return g.inner.Judge(ctx, req)
}

func (g *recordingGateway) requests() []modelgateway.JudgmentRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]modelgateway.JudgmentRequest, len(g.reqs))
	copy(out, g.reqs)
	return out
}

func expectJudgmentRequest(req modelgateway.JudgmentRequest, def rubrics.Definition, evidenceSubstrings ...string) {
	GinkgoHelper()
	Expect(req.RubricID).To(Equal(def.ID))
	Expect(req.RubricVersion).To(Equal(def.Version))
	Expect(req.OutputSchema).NotTo(BeEmpty(),
		"Gateway.Judge must receive OutputSchema so production validation enforces rubric enums")
	Expect(canonicalJSON(req.OutputSchema)).To(Equal(canonicalJSON(def.OutputSchema)),
		"Gateway.Judge OutputSchema must match seed definition for %s", def.ID)
	Expect(req.Messages).NotTo(BeEmpty(), "Gateway.Judge must receive evidence-bearing Messages")
	joined := joinedMessageContent(req.Messages)
	Expect(joined).NotTo(BeEmpty())
	for _, s := range evidenceSubstrings {
		Expect(joined).To(ContainSubstring(s), "Messages must carry deterministic evidence %q", s)
	}
}

func seedByID(id string) rubrics.Definition {
	GinkgoHelper()
	for _, def := range rubrics.Seed() {
		if def.ID == id {
			return def
		}
	}
	Fail("seed rubric not found: " + id)
	return rubrics.Definition{}
}

func joinedMessageContent(msgs []modelgateway.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
	}
	return b.String()
}

func canonicalJSON(raw []byte) []byte {
	var buf bytes.Buffer
	Expect(json.Compact(&buf, raw)).To(Succeed())
	return buf.Bytes()
}

func readGolden(name string) []byte {
	GinkgoHelper()
	path := filepath.Join("testdata", "golden", name)
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "golden fixture %s", name)
	return raw
}

func sampleHiddenMutationFinding() json.RawMessage {
	return json.RawMessage(`{
		"rule_id": "state.hidden_input_mutation",
		"kind": "hidden_input_mutation",
		"path": "pkg/example/service.go",
		"subject": "NewService",
		"evidence": "cfg.timeout = timeout"
	}`)
}

func sampleDeterministicFindings() json.RawMessage {
	return json.RawMessage(`[
		{
			"rule_id": "state.hidden_input_mutation",
			"kind": "hidden_input_mutation",
			"path": "pkg/example/service.go"
		},
		{
			"rule_id": "constructor.tight_init",
			"kind": "tight_constructor_init",
			"path": "pkg/example/client.go"
		}
	]`)
}
