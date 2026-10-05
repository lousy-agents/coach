package rubrics_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// recordingGateway is a test double that records every JudgmentRequest before
// delegating to an inner Gateway. Used to lock OutputSchema and Messages on the
// success path (StubGateway skips schema checks when OutputSchema is empty).
type recordingGateway struct {
	inner modelgateway.Gateway
	mu    sync.Mutex
	reqs  []modelgateway.JudgmentRequest
}

var _ = Describe("internal/rubrics seed LLM-as-judge definitions", func() {
	Describe("seed set", func() {
		When("the platform seeds the baseline LLM-as-judge rubrics", func() {
			It("exposes exactly the two versioned seed rubrics decided for v1", func() {
				body_rubricsAcceptanceTest_exposesExactlyTheTwoVersionedSeedRubricsDecidedF_33()
			})
		})
	})

	Describe("output schemas and golden stub judgments", func() {
		When("each seed rubric's output schema is applied to the stub gateway canned judgment", func() {
			It("validates successfully and matches the committed golden judgment byte-identically after canonicalization", func() {
				body_rubricsAcceptanceTest_validatesSuccessfullyAndMatchesTheCommittedGolde_57()
			})
		})

		When("a schema describes the seed rubric output contracts", func() {
			It("uses only the modelgateway validation subset (object, required, string enums, string|null)", func() {
				body_rubricsAcceptanceTest_usesOnlyTheModelgatewayValidationSubsetObjectReq_97()
			})
		})
	})

	Describe("prompt assembly", func() {
		When("assembling a hidden_mutation_contextualization judgment request", func() {
			It("attaches one deterministic hidden_input_mutation finding and baseline file context into gateway Messages", func() {
				body_rubricsAcceptanceTest_attachesOneDeterministicHiddenInputMutationFindi_141()
			})
		})

		When("assembling a change_cohesion judgment request", func() {
			It("attaches the full set of deterministic findings and file metadata into gateway Messages", func() {
				body_rubricsAcceptanceTest_attachesTheFullSetOfDeterministicFindingsAndFile_170()
			})
		})
	})

	Describe("agent-loop tool registration and judgment results", func() {
		When("a job handler registers the seed rubric tools on an agentloop.Loop", func() {
			It("registers tools named per ADR-005 and returns provenance-tagged judgments from the stub gateway", func() {
				loop := newLoop()
				rec := newRecordingGateway(modelgateway.NewStubGateway())

				Expect(rubrics.RegisterTools(loop, rec)).To(Succeed())

				hiddenDef := seedByID(rubrics.IDHiddenMutationContextualization)
				hiddenArgs := json.RawMessage(`{
					"finding": {
						"rule_id": "state.hidden_input_mutation",
						"kind": "hidden_input_mutation",
						"path": "pkg/example/service.go",
						"subject": "NewService",
						"evidence": "cfg.timeout = timeout"
					},
					"file": {
						"path": "pkg/example/service.go",
						"language": "go",
						"content": "package example\n"
					}
				}`)

				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, hiddenArgs)
				Expect(err).NotTo(HaveOccurred())

				var hiddenOut rubrics.ToolResult
				Expect(json.Unmarshal(raw, &hiddenOut)).To(Succeed())
				Expect(hiddenOut.RubricID).To(Equal(rubrics.IDHiddenMutationContextualization))
				Expect(hiddenOut.RubricVersion).To(Equal(rubrics.Version1))
				Expect(hiddenOut.Diagnostic).To(BeNil())
				Expect(hiddenOut.HasJudgment()).To(BeTrue())
				Expect(hiddenOut.ModelIdentity).NotTo(BeNil())
				Expect(*hiddenOut.ModelIdentity).To(Equal(modelgateway.LogicalModelStub))
				Expect(hiddenOut.LogicalModelID).NotTo(BeNil())
				Expect(*hiddenOut.LogicalModelID).To(Equal(modelgateway.LogicalModelStub))
				Expect(canonicalJSON(hiddenOut.Judgment)).To(Equal(
					canonicalJSON(readGolden("hidden_mutation_contextualization_v1.json")),
				))

				cohesionDef := seedByID(rubrics.IDChangeCohesion)
				cohesionArgs := json.RawMessage(`{
					"findings": [
						{"rule_id":"state.hidden_input_mutation","kind":"hidden_input_mutation","path":"pkg/example/service.go"},
						{"rule_id":"constructor.tight_init","kind":"tight_constructor_init","path":"pkg/example/client.go"}
					],
					"files": [
						{"path":"pkg/example/service.go","language":"go"},
						{"path":"pkg/example/client.go","language":"go"}
					]
				}`)
				raw2, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDChangeCohesion, cohesionArgs)
				Expect(err).NotTo(HaveOccurred())

				var cohesionOut rubrics.ToolResult
				Expect(json.Unmarshal(raw2, &cohesionOut)).To(Succeed())
				Expect(cohesionOut.RubricID).To(Equal(rubrics.IDChangeCohesion))
				Expect(cohesionOut.RubricVersion).To(Equal(rubrics.Version1))
				Expect(cohesionOut.Diagnostic).To(BeNil())
				Expect(cohesionOut.HasJudgment()).To(BeTrue())
				Expect(cohesionOut.ModelIdentity).NotTo(BeNil())
				Expect(*cohesionOut.ModelIdentity).To(Equal(modelgateway.LogicalModelStub))
				Expect(canonicalJSON(cohesionOut.Judgment)).To(Equal(
					canonicalJSON(readGolden("change_cohesion_v1.json")),
				))

				recorded := loop.Calls()
				Expect(recorded).To(HaveLen(2))
				Expect(recorded[0].Name).To(Equal(rubrics.IDHiddenMutationContextualization))
				Expect(recorded[0].Source).To(Equal(agentloop.CallSourceHandler))
				Expect(recorded[1].Name).To(Equal(rubrics.IDChangeCohesion))
				Expect(recorded[1].Source).To(Equal(agentloop.CallSourceHandler))

				judgeReqs := rec.requests()
				Expect(judgeReqs).To(HaveLen(2))
				expectJudgmentRequest(judgeReqs[0], hiddenDef,
					"state.hidden_input_mutation",
					"hidden_input_mutation",
					"pkg/example/service.go",
					"NewService",
				)
				expectJudgmentRequest(judgeReqs[1], cohesionDef,
					"state.hidden_input_mutation",
					"constructor.tight_init",
					"pkg/example/service.go",
					"pkg/example/client.go",
				)
			})
		})

		When("a rubric tool is invoked with deterministic findings as evidence", func() {
			It("does not modify or suppress the deterministic findings provided as input", func() {
				loop := newLoop()
				gw := modelgateway.NewStubGateway()
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				findingsBuf := sampleDeterministicFindings()
				rawArgs, err := json.Marshal(struct {
					Findings json.RawMessage    `json:"findings"`
					Files    []rubrics.FileMeta `json:"files"`
				}{
					Findings: findingsBuf,
					Files:    []rubrics.FileMeta{{Path: "pkg/example/service.go", Language: "go"}},
				})
				Expect(err).NotTo(HaveOccurred())
				args := json.RawMessage(rawArgs)
				argsBefore := append(json.RawMessage(nil), args...)

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDChangeCohesion, args)
				Expect(err).NotTo(HaveOccurred())

				Expect([]byte(args)).To(Equal([]byte(argsBefore)),
					"tool must not mutate the caller's args buffer (including embedded findings)")
				var parsed struct {
					Findings json.RawMessage `json:"findings"`
				}
				Expect(json.Unmarshal(args, &parsed)).To(Succeed())
				Expect(canonicalJSON(parsed.Findings)).To(Equal(canonicalJSON(findingsBuf)),
					"deterministic findings embedded in tool args must be unchanged after Call")
			})
		})
	})

	Describe("Story 5 unwanted path: judgment failure degrades gracefully", func() {
		When("the model gateway is unavailable for a rubric judgment", func() {
			It("records a diagnostic and does not emit a judgment payload suitable for source=agent findings", func() {
				loop := newLoop()
				gw := modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewUnavailableError("upstream timeout", context.DeadlineExceeded),
				})
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				deterministic := sampleDeterministicFindings()

				args := json.RawMessage(`{
					"finding": {
						"rule_id": "state.hidden_input_mutation",
						"kind": "hidden_input_mutation",
						"path": "pkg/example/service.go"
					},
					"file": {"path": "pkg/example/service.go", "language": "go"}
				}`)
				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)

				Expect(err).NotTo(HaveOccurred())

				var out rubrics.ToolResult
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.RubricID).To(Equal(rubrics.IDHiddenMutationContextualization))
				Expect(out.RubricVersion).To(Equal(rubrics.Version1))
				Expect(out.HasJudgment()).To(BeFalse())
				Expect(out.ModelIdentity).To(BeNil())
				Expect(out.Diagnostic).NotTo(BeNil())
				Expect(out.Diagnostic.Scope).To(ContainSubstring(rubrics.IDHiddenMutationContextualization))
				Expect(out.Diagnostic.Message).NotTo(BeEmpty())
				Expect(out.Diagnostic.Message).To(Or(
					ContainSubstring("unavailable"),
					ContainSubstring("timeout"),
				))
				Expect(errors.Is(modelgateway.NewUnavailableError("x", nil), modelgateway.ErrUnavailable)).To(BeTrue())

				Expect(deterministic).To(ContainSubstring("state.hidden_input_mutation"))

				run, runErr := rubrics.Run(context.Background(), gw, seedByID(rubrics.IDHiddenMutationContextualization),
					rubrics.AssembleHiddenMutationMessages(rubrics.HiddenMutationEvidence{
						Finding: sampleHiddenMutationFinding(),
						File:    rubrics.FileContext{Path: "pkg/example/service.go", Language: "go"},
					}))
				Expect(runErr).NotTo(HaveOccurred())
				Expect(run.Judgment).To(BeNil())
				Expect(run.Diagnostic).NotTo(BeNil())
				Expect(run.Diagnostic.Message).NotTo(BeEmpty())
			})
		})

		When("the model response fails rubric schema validation", func() {
			It("records a diagnostic and does not emit a source=agent judgment", func() {

				loop := newLoop()
				gw := modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewValidationError("judgment missing required field: confidence"),
				})
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				args := json.RawMessage(`{
					"findings": [{"rule_id":"state.hidden_input_mutation","path":"a.go"}],
					"files": [{"path":"a.go","language":"go"}]
				}`)
				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDChangeCohesion, args)
				Expect(err).NotTo(HaveOccurred())

				var out rubrics.ToolResult
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.HasJudgment()).To(BeFalse())
				Expect(out.ModelIdentity).To(BeNil())
				Expect(out.Diagnostic).NotTo(BeNil())
				Expect(out.Diagnostic.Scope).To(ContainSubstring(rubrics.IDChangeCohesion))
				Expect(out.Diagnostic.Message).To(ContainSubstring("schema"))

				_, judgeErr := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      rubrics.IDChangeCohesion,
					RubricVersion: rubrics.Version1,
					Messages:      []modelgateway.Message{{Role: "user", Content: "x"}},
					OutputSchema:  seedByID(rubrics.IDChangeCohesion).OutputSchema,
				})
				Expect(judgeErr).To(HaveOccurred())
				Expect(errors.Is(judgeErr, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(errors.Is(judgeErr, modelgateway.ErrUnavailable)).To(BeFalse())
			})
		})

		When("judgment fails because the owning context was canceled", func() {
			It("hard-fails the tool call and Run instead of soft-degrading to a diagnostic success", func() {

				loop := newLoop()
				gw := modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewUnavailableError("context done", context.Canceled),
				})
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				args := json.RawMessage(`{
					"findings": [{"rule_id":"state.hidden_input_mutation","path":"a.go"}],
					"files": [{"path":"a.go","language":"go"}]
				}`)
				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDChangeCohesion, args)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, context.Canceled)).To(BeTrue())
				Expect(raw).To(BeNil())

				timeoutGW := modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewUnavailableError("upstream timeout", context.DeadlineExceeded),
				})
				timeoutLoop := newLoop()
				Expect(rubrics.RegisterTools(timeoutLoop, timeoutGW)).To(Succeed())
				timeoutRaw, timeoutErr := timeoutLoop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDChangeCohesion, args)
				Expect(timeoutErr).NotTo(HaveOccurred())
				var timeoutOut rubrics.ToolResult
				Expect(json.Unmarshal(timeoutRaw, &timeoutOut)).To(Succeed())
				Expect(timeoutOut.HasJudgment()).To(BeFalse())
				Expect(timeoutOut.Diagnostic).NotTo(BeNil())

				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				run, runErr := rubrics.Run(ctx, modelgateway.NewStubGateway(),
					seedByID(rubrics.IDChangeCohesion), nil)
				Expect(runErr).To(HaveOccurred())
				Expect(errors.Is(runErr, context.Canceled)).To(BeTrue())
				Expect(run.Judgment).To(BeNil())
				Expect(run.Diagnostic).To(BeNil())
			})
		})
	})

	Describe("Run judgment API for job handlers", func() {
		When("a successful judgment is produced", func() {
			It("exposes rubric_id, rubric_version, model identity, and judgment payload for later source=agent findings", func() {
				rec := newRecordingGateway(modelgateway.NewStubGateway())
				def := seedByID(rubrics.IDHiddenMutationContextualization)
				msgs := rubrics.AssembleHiddenMutationMessages(rubrics.HiddenMutationEvidence{
					Finding: sampleHiddenMutationFinding(),
					File: rubrics.FileContext{
						Path:     "pkg/example/service.go",
						Language: "go",
						Content:  "package example\nfunc NewService(cfg *Config) *Service { cfg.timeout = 1; return &Service{} }\n",
					},
				})

				result, err := rubrics.Run(context.Background(), rec, def, msgs)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Diagnostic).To(BeNil())
				Expect(result.Judgment).NotTo(BeNil())
				Expect(result.Judgment.RubricID).To(Equal(rubrics.IDHiddenMutationContextualization))
				Expect(result.Judgment.RubricVersion).To(Equal(rubrics.Version1))
				Expect(result.Judgment.ModelIdentity).To(Equal(modelgateway.LogicalModelStub))
				Expect(result.Judgment.LogicalModelID).To(Equal(modelgateway.LogicalModelStub))
				Expect(result.Judgment.JudgmentJSON).NotTo(BeEmpty())
				Expect(canonicalJSON(result.Judgment.JudgmentJSON)).To(Equal(
					canonicalJSON(readGolden("hidden_mutation_contextualization_v1.json")),
				))

				judgeReqs := rec.requests()
				Expect(judgeReqs).To(HaveLen(1))
				expectJudgmentRequest(judgeReqs[0], def,
					"state.hidden_input_mutation",
					"hidden_input_mutation",
					"pkg/example/service.go",
					"cfg.timeout = 1",
					"NewService",
				)
			})
		})

		When("each seed rubric is judged via Run", func() {
			It("forwards that rubric's OutputSchema and evidence-bearing Messages to Gateway.Judge", func() {
				body_rubricsAcceptanceTest_forwardsThatRubricSOutputSchemaAndEvidenceBearin_499()
			})
		})
	})
})

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

func newLoop() *agentloop.Loop {
	GinkgoHelper()
	loop, err := agentloop.New(agentloop.Options{
		SemanticsAnalyze: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"ok":true}`), nil
		},
		CodeSignalReport: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"ok":true}`), nil
		},
	})
	Expect(err).NotTo(HaveOccurred())
	return loop
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
