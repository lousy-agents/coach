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

func joinedMessageContent(msgs []modelgateway.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
	}
	return b.String()
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

var _ = Describe("internal/rubrics seed LLM-as-judge definitions", func() {
	Describe("seed set", func() {
		When("the platform seeds the baseline LLM-as-judge rubrics", func() {
			It("exposes exactly the two versioned seed rubrics decided for v1", func() {
				seed := rubrics.Seed()
				Expect(seed).To(HaveLen(2))

				byID := map[string]rubrics.Definition{}
				for _, def := range seed {
					byID[def.ID] = def
				}

				hidden, ok := byID[rubrics.IDHiddenMutationContextualization]
				Expect(ok).To(BeTrue(), "missing hidden_mutation_contextualization")
				Expect(hidden.Version).To(Equal(rubrics.Version1))
				Expect(hidden.OutputSchema).NotTo(BeEmpty())

				cohesion, ok := byID[rubrics.IDChangeCohesion]
				Expect(ok).To(BeTrue(), "missing change_cohesion")
				Expect(cohesion.Version).To(Equal(rubrics.Version1))
				Expect(cohesion.OutputSchema).NotTo(BeEmpty())
			})
		})
	})

	Describe("output schemas and golden stub judgments", func() {
		When("each seed rubric's output schema is applied to the stub gateway canned judgment", func() {
			It("validates successfully and matches the committed golden judgment byte-identically after canonicalization", func() {
				cases := []struct {
					id         string
					goldenFile string
				}{
					{rubrics.IDHiddenMutationContextualization, "hidden_mutation_contextualization_v1.json"},
					{rubrics.IDChangeCohesion, "change_cohesion_v1.json"},
				}

				gw := modelgateway.NewStubGateway()
				for _, tc := range cases {
					def := seedByID(tc.id)
					resp, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
						RubricID:      def.ID,
						RubricVersion: def.Version,
						Messages: []modelgateway.Message{
							{Role: "user", Content: "fixture judgment"},
						},
						OutputSchema: def.OutputSchema,
					})
					Expect(err).NotTo(HaveOccurred(), "schema must accept stub judgment for %s", tc.id)
					Expect(resp.LogicalModelID).To(Equal(modelgateway.LogicalModelStub))

					golden := readGolden(tc.goldenFile)
					Expect(canonicalJSON(resp.JudgmentJSON)).To(Equal(canonicalJSON(golden)),
						"stub judgment for %s must match golden byte-identically (canonical JSON)", tc.id)

					// Round-trip: golden must also be accepted by the same schema path.
					resp2, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
						RubricID:      def.ID,
						RubricVersion: def.Version,
						Messages:      []modelgateway.Message{{Role: "user", Content: "round-trip"}},
						OutputSchema:  def.OutputSchema,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(canonicalJSON(resp2.JudgmentJSON)).To(Equal(canonicalJSON(golden)))
				}
			})
		})

		When("a schema describes the seed rubric output contracts", func() {
			It("uses only the modelgateway validation subset (object, required, string enums, string|null)", func() {
				seed := rubrics.Seed()
				Expect(seed).NotTo(BeEmpty())
				for _, def := range seed {
					var sch map[string]any
					Expect(json.Unmarshal(def.OutputSchema, &sch)).To(Succeed())
					Expect(sch["type"]).To(Equal("object"))

					required, ok := sch["required"].([]any)
					Expect(ok).To(BeTrue())
					Expect(required).To(ConsistOf("judgment", "rationale", "confidence", "suggested_focus"))

					props, ok := sch["properties"].(map[string]any)
					Expect(ok).To(BeTrue())

					for _, key := range []string{"judgment", "rationale", "confidence", "suggested_focus"} {
						_, present := props[key]
						Expect(present).To(BeTrue(), "%s schema missing property %s", def.ID, key)
					}

					// suggested_focus must allow string|null (subset type array).
					sf, ok := props["suggested_focus"].(map[string]any)
					Expect(ok).To(BeTrue())
					Expect(sf["type"]).To(ConsistOf("string", "null"))

					// judgment/confidence are string enums — reject unknown enum via stub schema path
					// by ensuring a known-bad judgment fails validation when schema is enforced.
					// (Stub returns valid canned JSON; prove schema rejects invalid values via gateway
					// Judge path is covered by degrade tests. Here lock enum membership.)
					judgmentProp := props["judgment"].(map[string]any)
					Expect(judgmentProp["type"]).To(Equal("string"))
					enumVals, ok := judgmentProp["enum"].([]any)
					Expect(ok).To(BeTrue())
					switch def.ID {
					case rubrics.IDHiddenMutationContextualization:
						Expect(enumVals).To(ConsistOf("concern", "acceptable", "unclear"))
					case rubrics.IDChangeCohesion:
						Expect(enumVals).To(ConsistOf("focused", "diffuse", "unclear"))
					}

					confProp := props["confidence"].(map[string]any)
					Expect(confProp["enum"]).To(ConsistOf("high", "medium", "low"))
				}
			})
		})
	})

	Describe("prompt assembly", func() {
		When("assembling a hidden_mutation_contextualization judgment request", func() {
			It("attaches one deterministic hidden_input_mutation finding and baseline file context into gateway Messages", func() {
				finding := sampleHiddenMutationFinding()
				file := rubrics.FileContext{
					Path:     "pkg/example/service.go",
					Language: "go",
					Content:  "package example\n\nfunc NewService(cfg *Config) *Service { cfg.timeout = 1; return &Service{} }\n",
				}

				msgs := rubrics.AssembleHiddenMutationMessages(rubrics.HiddenMutationEvidence{
					Finding: finding,
					File:    file,
				})
				Expect(msgs).NotTo(BeEmpty())

				joined := ""
				for _, m := range msgs {
					Expect(m.Role).NotTo(BeEmpty())
					Expect(m.Content).NotTo(BeEmpty())
					joined += m.Content
				}
				Expect(joined).To(ContainSubstring("state.hidden_input_mutation"))
				Expect(joined).To(ContainSubstring("hidden_input_mutation"))
				Expect(joined).To(ContainSubstring("pkg/example/service.go"))
				Expect(joined).To(ContainSubstring("cfg.timeout = 1"))
				Expect(joined).To(ContainSubstring("NewService"))
			})
		})

		When("assembling a change_cohesion judgment request", func() {
			It("attaches the full set of deterministic findings and file metadata into gateway Messages", func() {
				findings := sampleDeterministicFindings()
				files := []rubrics.FileMeta{
					{Path: "pkg/example/service.go", Language: "go"},
					{Path: "pkg/example/client.go", Language: "go"},
				}

				msgs := rubrics.AssembleChangeCohesionMessages(rubrics.ChangeCohesionEvidence{
					Findings: findings,
					Files:    files,
				})
				Expect(msgs).NotTo(BeEmpty())

				joined := ""
				for _, m := range msgs {
					Expect(m.Role).NotTo(BeEmpty())
					Expect(m.Content).NotTo(BeEmpty())
					joined += m.Content
				}
				Expect(joined).To(ContainSubstring("state.hidden_input_mutation"))
				Expect(joined).To(ContainSubstring("constructor.tight_init"))
				Expect(joined).To(ContainSubstring("pkg/example/service.go"))
				Expect(joined).To(ContainSubstring("pkg/example/client.go"))
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

				// change_cohesion
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

				// Lock Gateway.Judge request contract: OutputSchema + evidence Messages.
				// StubGateway skips schema checks when OutputSchema is empty — this must fail
				// if Run/tools drop OutputSchema or Messages.
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

				// Shared args buffer the tool actually receives (findings embedded).
				// Asserting an unrelated local that never reaches the handler is a false-green.
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

				// Deterministic evidence remains available to the caller independently.
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
				// Tool must not fail the whole job hard — degrade via result envelope.
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

				// Deterministic evidence still intact for the job handler.
				Expect(deterministic).To(ContainSubstring("state.hidden_input_mutation"))

				// Same contract via Run API used by handlers that skip the loop for a single judgment.
				run := rubrics.Run(context.Background(), gw, seedByID(rubrics.IDHiddenMutationContextualization),
					rubrics.AssembleHiddenMutationMessages(rubrics.HiddenMutationEvidence{
						Finding: sampleHiddenMutationFinding(),
						File:    rubrics.FileContext{Path: "pkg/example/service.go", Language: "go"},
					}))
				Expect(run.Judgment).To(BeNil())
				Expect(run.Diagnostic).NotTo(BeNil())
				Expect(run.Diagnostic.Message).NotTo(BeEmpty())
			})
		})

		When("the model response fails rubric schema validation", func() {
			It("records a diagnostic and does not emit a source=agent judgment", func() {
				// False-green guard: exercise ErrSchemaValidation path, not unavailable.
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

				// Prove the injected error really is schema validation (not a substitute path).
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
				result := rubrics.Run(context.Background(), rec, def, msgs)

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

				// Lock that Run forwards seed OutputSchema and evidence Messages to Gateway.Judge.
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
				// One call per seed rubric so deleting OutputSchema or Messages cannot stay green.
				cases := []struct {
					id         string
					msgs       []modelgateway.Message
					evidence   []string
					goldenFile string
				}{
					{
						id: rubrics.IDHiddenMutationContextualization,
						msgs: rubrics.AssembleHiddenMutationMessages(rubrics.HiddenMutationEvidence{
							Finding: sampleHiddenMutationFinding(),
							File: rubrics.FileContext{
								Path:     "pkg/example/service.go",
								Language: "go",
								Content:  "package example\nfunc NewService(cfg *Config) *Service { cfg.timeout = 1; return &Service{} }\n",
							},
						}),
						evidence: []string{
							"state.hidden_input_mutation",
							"hidden_input_mutation",
							"pkg/example/service.go",
							"cfg.timeout = 1",
							"NewService",
						},
						goldenFile: "hidden_mutation_contextualization_v1.json",
					},
					{
						id: rubrics.IDChangeCohesion,
						msgs: rubrics.AssembleChangeCohesionMessages(rubrics.ChangeCohesionEvidence{
							Findings: sampleDeterministicFindings(),
							Files: []rubrics.FileMeta{
								{Path: "pkg/example/service.go", Language: "go"},
								{Path: "pkg/example/client.go", Language: "go"},
							},
						}),
						evidence: []string{
							"state.hidden_input_mutation",
							"constructor.tight_init",
							"pkg/example/service.go",
							"pkg/example/client.go",
						},
						goldenFile: "change_cohesion_v1.json",
					},
				}

				for _, tc := range cases {
					def := seedByID(tc.id)
					rec := newRecordingGateway(modelgateway.NewStubGateway())
					result := rubrics.Run(context.Background(), rec, def, tc.msgs)
					Expect(result.Diagnostic).To(BeNil(), "rubric %s", tc.id)
					Expect(result.Judgment).NotTo(BeNil(), "rubric %s", tc.id)
					Expect(canonicalJSON(result.Judgment.JudgmentJSON)).To(Equal(
						canonicalJSON(readGolden(tc.goldenFile)),
					))

					judgeReqs := rec.requests()
					Expect(judgeReqs).To(HaveLen(1), "rubric %s", tc.id)
					expectJudgmentRequest(judgeReqs[0], def, tc.evidence...)
				}
			})
		})
	})
})
