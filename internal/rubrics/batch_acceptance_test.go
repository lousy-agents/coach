package rubrics_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

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

var _ = Describe("batch hidden_mutation rubric (local-LLM pack path)", func() {
	Describe("FormatSpanWindow", func() {
		When("given full file content, a 0-based start_row, and a radius", func() {
			It("returns numbered window text covering ±radius lines with a marker on start_row", func() {
				// 40 lines (indices 0..39); start_row=20, radius=3 → lines 17..23
				content := sampleFileContent(40)
				got := rubrics.FormatSpanWindow(content, 20, 3)

				Expect(got).To(ContainSubstring("line-17-content"))
				Expect(got).To(ContainSubstring("line-20-content"))
				Expect(got).To(ContainSubstring("line-23-content"))
				Expect(got).NotTo(ContainSubstring("line-16-content"))
				Expect(got).NotTo(ContainSubstring("line-24-content"))

				// 1-based display numbers and a focus marker on the start_row line.
				Expect(got).To(MatchRegexp(`(?m)^>*\s*18\|`)) // 0-based 17 → display 18
				Expect(got).To(MatchRegexp(`(?m)^>\s*21\|.*line-20-content`))
				Expect(got).To(MatchRegexp(`(?m)^>*\s*24\|`)) // 0-based 23 → display 24
			})
		})

		When("radius is zero", func() {
			It("uses DefaultEvidenceWindowLines (15)", func() {
				content := sampleFileContent(50)
				got := rubrics.FormatSpanWindow(content, 25, 0)
				// ±15 around 25 → 10..40
				Expect(got).To(ContainSubstring("line-10-content"))
				Expect(got).To(ContainSubstring("line-40-content"))
				Expect(got).NotTo(ContainSubstring("line-9-content"))
				Expect(got).NotTo(ContainSubstring("line-41-content"))
			})
		})
	})

	Describe("pack tool Call", func() {
		When("the handler invokes hidden_mutation_contextualization with a 3-item pack and the stub gateway", func() {
			It("returns three valid judgments with matching finding_refs via the pack results envelope", func() {
				loop := newLoop()
				rec := newRecordingGateway(modelgateway.NewStubGateway())
				Expect(rubrics.RegisterTools(loop, rec)).To(Succeed())

				content := sampleFileContent(30)
				args, err := json.Marshal(map[string]any{
					"items": []any{
						packItemArgs("ref-a", "a.go", 5, content),
						packItemArgs("ref-b", "b.go", 10, content),
						packItemArgs("ref-c", "c.go", 15, content),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)
				Expect(err).NotTo(HaveOccurred())

				pack := decodePackResults(raw)
				Expect(pack.Results).To(HaveLen(3))

				refs := make([]string, 0, 3)
				for _, r := range pack.Results {
					Expect(r.FindingRef).NotTo(BeEmpty())
					refs = append(refs, r.FindingRef)
					Expect(r.Diagnostic).To(BeNil(), "finding_ref=%s", r.FindingRef)
					Expect(r.HasJudgment()).To(BeTrue(), "finding_ref=%s", r.FindingRef)
					Expect(r.RubricID).To(Equal(rubrics.IDHiddenMutationContextualization))
					Expect(r.RubricVersion).To(Equal(rubrics.Version1))
					Expect(r.ModelIdentity).NotTo(BeNil())
					Expect(*r.ModelIdentity).To(Equal(modelgateway.LogicalModelStub))

					var body map[string]any
					Expect(json.Unmarshal(r.Judgment, &body)).To(Succeed())
					Expect(body["judgment"]).To(BeElementOf("concern", "acceptable", "unclear"))
					Expect(body["confidence"]).To(BeElementOf("high", "medium", "low"))
					Expect(body["rationale"]).NotTo(BeEmpty())
				}
				Expect(refs).To(ConsistOf("ref-a", "ref-b", "ref-c"))

				// Gateway must receive batch OutputSchema (items envelope), not singular v1 only.
				judgeReqs := rec.requests()
				Expect(judgeReqs).To(HaveLen(1))
				Expect(judgeReqs[0].OutputSchema).NotTo(BeEmpty())
				var sch map[string]any
				Expect(json.Unmarshal(judgeReqs[0].OutputSchema, &sch)).To(Succeed())
				props, ok := sch["properties"].(map[string]any)
				Expect(ok).To(BeTrue())
				_, hasItems := props["items"]
				Expect(hasItems).To(BeTrue(), "pack Judge must use batch envelope OutputSchema with items")
				Expect(canonicalJSON(judgeReqs[0].OutputSchema)).To(Equal(
					canonicalJSON(rubrics.HiddenMutationBatchOutputSchema()),
				))
			})
		})

		When("the model batch response omits one finding_ref", func() {
			It("returns two valid judgments and one diagnostic for the missing ref only", func() {
				loop := newLoop()
				// Valid items for ref-a and ref-c only — ref-b missing (partial pack success).
				partial := json.RawMessage(`{
					"items": [
						{
							"finding_ref": "ref-a",
							"judgment": "acceptable",
							"rationale": "ok a",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-c",
							"judgment": "concern",
							"rationale": "ok c",
							"confidence": "medium",
							"suggested_focus": "review mutation"
						}
					]
				}`)
				gw := &fixedBatchGateway{judgment: partial}
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				content := sampleFileContent(20)
				args, err := json.Marshal(map[string]any{
					"items": []any{
						packItemArgs("ref-a", "a.go", 1, content),
						packItemArgs("ref-b", "b.go", 2, content),
						packItemArgs("ref-c", "c.go", 3, content),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)
				Expect(err).NotTo(HaveOccurred())

				pack := decodePackResults(raw)
				Expect(pack.Results).To(HaveLen(3))

				byRef := map[string]rubrics.ToolResult{}
				for _, r := range pack.Results {
					byRef[r.FindingRef] = r
				}
				Expect(byRef).To(HaveKey("ref-a"))
				Expect(byRef).To(HaveKey("ref-b"))
				Expect(byRef).To(HaveKey("ref-c"))

				Expect(byRef["ref-a"].HasJudgment()).To(BeTrue())
				Expect(byRef["ref-a"].Diagnostic).To(BeNil())
				Expect(byRef["ref-c"].HasJudgment()).To(BeTrue())
				Expect(byRef["ref-c"].Diagnostic).To(BeNil())

				Expect(byRef["ref-b"].HasJudgment()).To(BeFalse())
				Expect(byRef["ref-b"].Diagnostic).NotTo(BeNil())
				Expect(byRef["ref-b"].Diagnostic.Message).NotTo(BeEmpty())
				Expect(byRef["ref-b"].Diagnostic.Scope).To(ContainSubstring(rubrics.IDHiddenMutationContextualization))
			})
		})

		When("assembling messages for a multi-finding pack", func() {
			It("includes short-rationale guidance (≤2 sentences / ≤400 chars) in system or user prompt", func() {
				loop := newLoop()
				rec := newRecordingGateway(modelgateway.NewStubGateway())
				Expect(rubrics.RegisterTools(loop, rec)).To(Succeed())

				content := sampleFileContent(20)
				args, err := json.Marshal(map[string]any{
					"items": []any{
						packItemArgs("ref-a", "a.go", 1, content),
						packItemArgs("ref-b", "b.go", 2, content),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)
				Expect(err).NotTo(HaveOccurred())

				judgeReqs := rec.requests()
				Expect(judgeReqs).To(HaveLen(1))
				joined := joinedMessageContent(judgeReqs[0].Messages)
				Expect(joined).To(Or(
					ContainSubstring("2 sentences"),
					ContainSubstring("two sentences"),
				))
				Expect(joined).To(ContainSubstring("400"))
				// Evidence must still carry finding refs for stub/provenance.
				Expect(joined).To(ContainSubstring("ref-a"))
				Expect(joined).To(ContainSubstring("ref-b"))
			})
		})
	})

	Describe("singular path compatibility", func() {
		When("the handler still invokes hidden_mutation with legacy singular {finding,file} args", func() {
			It("returns a singular ToolResult envelope (not a pack results wrapper)", func() {
				loop := newLoop()
				Expect(rubrics.RegisterTools(loop, modelgateway.NewStubGateway())).To(Succeed())

				args := json.RawMessage(`{
					"finding": {
						"rule_id": "state.hidden_input_mutation",
						"kind": "hidden_input_mutation",
						"path": "pkg/example/service.go",
						"subject": "NewService"
					},
					"file": {
						"path": "pkg/example/service.go",
						"language": "go",
						"content": "package example\n"
					}
				}`)
				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)
				Expect(err).NotTo(HaveOccurred())

				// Must not look like a pack envelope.
				Expect(rubrics.IsToolPackResult(raw)).To(BeFalse())

				var out rubrics.ToolResult
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.HasJudgment()).To(BeTrue())
				Expect(out.Diagnostic).To(BeNil())
				Expect(out.RubricID).To(Equal(rubrics.IDHiddenMutationContextualization))
				Expect(canonicalJSON(out.Judgment)).To(Equal(
					canonicalJSON(readGolden("hidden_mutation_contextualization_v1.json")),
				))
			})
		})
	})

	Describe("stub gateway batch canned responses", func() {
		When("OutputSchema is the batch envelope and messages list finding_refs", func() {
			It("returns canned batch JSON that includes those finding_refs", func() {
				gw := modelgateway.NewStubGateway()
				schema := rubrics.HiddenMutationBatchOutputSchema()
				resp, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      rubrics.IDHiddenMutationContextualization,
					RubricVersion: rubrics.Version1,
					Messages: []modelgateway.Message{
						{Role: "user", Content: "finding_ref: pack-1\nfinding_ref: pack-2\n"},
					},
					OutputSchema: schema,
				})
				Expect(err).NotTo(HaveOccurred())

				var body struct {
					Items []struct {
						FindingRef string `json:"finding_ref"`
						Judgment   string `json:"judgment"`
						Rationale  string `json:"rationale"`
						Confidence string `json:"confidence"`
					} `json:"items"`
				}
				Expect(json.Unmarshal(resp.JudgmentJSON, &body)).To(Succeed())
				Expect(body.Items).To(HaveLen(2))
				Expect(body.Items[0].FindingRef).To(Equal("pack-1"))
				Expect(body.Items[1].FindingRef).To(Equal("pack-2"))
				for _, it := range body.Items {
					Expect(it.Judgment).To(BeElementOf("concern", "acceptable", "unclear"))
					Expect(it.Confidence).To(BeElementOf("high", "medium", "low"))
					Expect(it.Rationale).NotTo(BeEmpty())
				}
			})
		})
	})

	Describe("OpenAICompatClient batch OutputSchema", func() {
		When("Judge is called with HiddenMutationBatchOutputSchema and valid batch assistant JSON", func() {
			It("accepts the batch envelope schema and returns the validated judgment (upstream is reached)", func() {
				validBatch := `{
					"items": [
						{
							"finding_ref": "ref-a",
							"judgment": "acceptable",
							"rationale": "ok a",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-b",
							"judgment": "concern",
							"rationale": "ok b",
							"confidence": "medium",
							"suggested_focus": "review mutation"
						}
					]
				}`
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					Expect(r.Method).To(Equal(http.MethodPost))
					_, _ = io.ReadAll(r.Body)
					w.Header().Set("Content-Type", "application/json")
					body, err := json.Marshal(map[string]any{
						"id":     "chatcmpl-batch",
						"object": "chat.completion",
						"model":  "served-batch",
						"choices": []map[string]any{
							{
								"index": 0,
								"message": map[string]any{
									"role":    "assistant",
									"content": validBatch,
								},
								"finish_reason": "stop",
							},
						},
					})
					Expect(err).NotTo(HaveOccurred())
					_, _ = w.Write(body)
				}))
				DeferCleanup(srv.Close)

				httpClient := srv.Client()
				httpClient.Timeout = modelgateway.DefaultHTTPClientTimeout
				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   httpClient,
				})
				Expect(err).NotTo(HaveOccurred())

				resp, err := client.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      rubrics.IDHiddenMutationContextualization,
					RubricVersion: rubrics.Version1,
					Messages: []modelgateway.Message{
						{Role: "user", Content: "finding_ref: ref-a\nfinding_ref: ref-b\n"},
					},
					// Production pack path: batch envelope must not fail closed as
					// "items has unsupported schema type: array" before upstream.
					OutputSchema: rubrics.HiddenMutationBatchOutputSchema(),
				})
				Expect(err).NotTo(HaveOccurred(),
					"OpenAICompatClient must accept HiddenMutationBatchOutputSchema")
				// False-green guard: schema acceptance is proven by reaching upstream.
				// A pre-upstream schema reject would leave attempts at 0.
				Expect(attempts.Load()).To(Equal(int32(1)))
				Expect(resp.JudgmentJSON).NotTo(BeEmpty())
				Expect(resp.ServedModelID).To(Equal("served-batch"))

				var body struct {
					Items []struct {
						FindingRef string `json:"finding_ref"`
						Judgment   string `json:"judgment"`
						Confidence string `json:"confidence"`
					} `json:"items"`
				}
				Expect(json.Unmarshal(resp.JudgmentJSON, &body)).To(Succeed())
				Expect(body.Items).To(HaveLen(2))
				Expect(body.Items[0].FindingRef).To(Equal("ref-a"))
				Expect(body.Items[1].FindingRef).To(Equal("ref-b"))
			})
		})

		When("Judge is called with an unsupported non-batch array property schema", func() {
			It("still fails closed with zero upstream calls", func() {
				// Keep singular string|null + batch-only array-of-objects support;
				// bare integer/array-of-string must not open a hole.
				badSchema := json.RawMessage(`{
					"type": "object",
					"required": ["tags"],
					"properties": {
						"tags": {"type": "array", "items": {"type": "string"}}
					}
				}`)
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"tags\":[]}"}}]}`))
				}))
				DeferCleanup(srv.Close)

				httpClient := srv.Client()
				httpClient.Timeout = modelgateway.DefaultHTTPClientTimeout
				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   httpClient,
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      rubrics.IDHiddenMutationContextualization,
					RubricVersion: rubrics.Version1,
					Messages:      []modelgateway.Message{{Role: "user", Content: "x"}},
					OutputSchema:  badSchema,
				})
				Expect(err).To(HaveOccurred())
				Expect(attempts.Load()).To(Equal(int32(0)),
					"unsupported array element schemas must fail before upstream")
			})
		})
	})

	Describe("pack tool Call partial item validation", func() {
		When("the model batch response has an invalid judgment enum for one finding_ref", func() {
			It("returns a diagnostic for the bad ref only and judgments for the other refs", func() {
				loop := newLoop()
				mixed := json.RawMessage(`{
					"items": [
						{
							"finding_ref": "ref-a",
							"judgment": "acceptable",
							"rationale": "ok a",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-b",
							"judgment": "not-a-valid-enum",
							"rationale": "bad b",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-c",
							"judgment": "concern",
							"rationale": "ok c",
							"confidence": "medium",
							"suggested_focus": "focus c"
						}
					]
				}`)
				gw := &fixedBatchGateway{judgment: mixed}
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				content := sampleFileContent(20)
				args, err := json.Marshal(map[string]any{
					"items": []any{
						packItemArgs("ref-a", "a.go", 1, content),
						packItemArgs("ref-b", "b.go", 2, content),
						packItemArgs("ref-c", "c.go", 3, content),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)
				Expect(err).NotTo(HaveOccurred())

				pack := decodePackResults(raw)
				Expect(pack.Results).To(HaveLen(3))
				byRef := map[string]rubrics.ToolResult{}
				for _, r := range pack.Results {
					byRef[r.FindingRef] = r
				}

				Expect(byRef["ref-a"].HasJudgment()).To(BeTrue())
				Expect(byRef["ref-a"].Diagnostic).To(BeNil())
				Expect(byRef["ref-c"].HasJudgment()).To(BeTrue())
				Expect(byRef["ref-c"].Diagnostic).To(BeNil())

				Expect(byRef["ref-b"].HasJudgment()).To(BeFalse())
				Expect(byRef["ref-b"].Diagnostic).NotTo(BeNil())
				Expect(byRef["ref-b"].Diagnostic.Message).To(ContainSubstring("enum"))
			})
		})

		When("the model batch response has an invalid suggested_focus type for one finding_ref", func() {
			It("returns a diagnostic for the bad ref only and judgments for the other refs", func() {
				loop := newLoop()
				mixed := json.RawMessage(`{
					"items": [
						{
							"finding_ref": "ref-a",
							"judgment": "acceptable",
							"rationale": "ok a",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-b",
							"judgment": "concern",
							"rationale": "bad focus type",
							"confidence": "high",
							"suggested_focus": 42
						},
						{
							"finding_ref": "ref-c",
							"judgment": "unclear",
							"rationale": "ok c",
							"confidence": "low",
							"suggested_focus": null
						}
					]
				}`)
				gw := &fixedBatchGateway{judgment: mixed}
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				content := sampleFileContent(20)
				args, err := json.Marshal(map[string]any{
					"items": []any{
						packItemArgs("ref-a", "a.go", 1, content),
						packItemArgs("ref-b", "b.go", 2, content),
						packItemArgs("ref-c", "c.go", 3, content),
					},
				})
				Expect(err).NotTo(HaveOccurred())

				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)
				Expect(err).NotTo(HaveOccurred())

				pack := decodePackResults(raw)
				byRef := map[string]rubrics.ToolResult{}
				for _, r := range pack.Results {
					byRef[r.FindingRef] = r
				}

				Expect(byRef["ref-a"].HasJudgment()).To(BeTrue())
				Expect(byRef["ref-a"].Diagnostic).To(BeNil())
				Expect(byRef["ref-c"].HasJudgment()).To(BeTrue())
				Expect(byRef["ref-c"].Diagnostic).To(BeNil())

				Expect(byRef["ref-b"].HasJudgment()).To(BeFalse())
				Expect(byRef["ref-b"].Diagnostic).NotTo(BeNil())
				Expect(byRef["ref-b"].Diagnostic.Message).To(Or(
					ContainSubstring("suggested_focus"),
					ContainSubstring("schema validation"),
				))
			})
		})

		When("the pack args contain a duplicate finding_ref", func() {
			It("returns a diagnostic for the duplicate occurrence and a judgment for the first ref", func() {
				loop := newLoop()
				// Gateway returns a valid item for the shared ref; pack args order
				// decides which slot is the duplicate diagnostic.
				ok := json.RawMessage(`{
					"items": [
						{
							"finding_ref": "ref-dup",
							"judgment": "acceptable",
							"rationale": "ok dup",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-other",
							"judgment": "concern",
							"rationale": "ok other",
							"confidence": "medium",
							"suggested_focus": null
						}
					]
				}`)
				gw := &fixedBatchGateway{judgment: ok}
				Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

				content := sampleFileContent(20)
				args, err := json.Marshal(map[string]any{
					"items": []any{
						packItemArgs("ref-dup", "a.go", 1, content),
						packItemArgs("ref-other", "b.go", 2, content),
						packItemArgs("ref-dup", "c.go", 3, content), // duplicate
					},
				})
				Expect(err).NotTo(HaveOccurred())

				raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
					rubrics.IDHiddenMutationContextualization, args)
				Expect(err).NotTo(HaveOccurred())

				pack := decodePackResults(raw)
				Expect(pack.Results).To(HaveLen(3))

				// First ref-dup: judgment; second ref-dup: diagnostic; ref-other: judgment.
				Expect(pack.Results[0].FindingRef).To(Equal("ref-dup"))
				Expect(pack.Results[0].HasJudgment()).To(BeTrue())
				Expect(pack.Results[0].Diagnostic).To(BeNil())

				Expect(pack.Results[1].FindingRef).To(Equal("ref-other"))
				Expect(pack.Results[1].HasJudgment()).To(BeTrue())
				Expect(pack.Results[1].Diagnostic).To(BeNil())

				Expect(pack.Results[2].FindingRef).To(Equal("ref-dup"))
				Expect(pack.Results[2].HasJudgment()).To(BeFalse())
				Expect(pack.Results[2].Diagnostic).NotTo(BeNil())
				Expect(pack.Results[2].Diagnostic.Message).To(ContainSubstring("duplicate"))
			})
		})
	})
})
