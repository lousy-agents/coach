package modelgateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

var _ = Describe("modelgateway HTTP stub (compose model-stub backend)", func() {
	var srv *httptest.Server

	BeforeEach(func() {
		srv = httptest.NewServer(modelgateway.NewHTTPStubHandler())
		DeferCleanup(srv.Close)
	})

	When("GET /healthz is requested", func() {
		It("returns 200 so compose healthchecks can mark the stub ready", func() {
			resp, err := http.Get(srv.URL + "/healthz")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(ContainSubstring("ok"))
		})
	})

	When("POST /v1/chat/completions carries a hidden_input_mutation rubric prompt", func() {
		It("returns assistant content that validates against the hidden_mutation seed schema", func() {
			client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
				BaseURL:      srv.URL,
				LogicalModel: "stub",
				HTTPClient:   &http.Client{Timeout: 5 * time.Second},
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.Judge(context.Background(), modelgateway.JudgmentRequest{
				RubricID: "hidden_mutation_contextualization",
				Messages: []modelgateway.Message{
					{Role: "system", Content: "Evaluate one deterministic hidden_input_mutation finding."},
					{Role: "user", Content: "path: widget/update.go"},
				},
				OutputSchema: hiddenMutationSchema(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.JudgmentJSON).NotTo(BeEmpty())

			var j hiddenMutationJudgment
			Expect(json.Unmarshal(resp.JudgmentJSON, &j)).To(Succeed())
			Expect(j.Judgment).To(Equal("acceptable"))
			Expect(j.Confidence).To(Equal("high"))
		})
	})

	When("POST /v1/chat/completions carries a change_cohesion rubric prompt", func() {
		It("returns assistant content that validates against the change_cohesion seed schema", func() {
			client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
				BaseURL:      srv.URL,
				LogicalModel: "stub",
				HTTPClient:   &http.Client{Timeout: 5 * time.Second},
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.Judge(context.Background(), modelgateway.JudgmentRequest{
				RubricID: "change_cohesion",
				Messages: []modelgateway.Message{
					{Role: "system", Content: "Do the analyzed files and findings cluster into coherent areas of concern?"},
					{Role: "user", Content: "findings: [...]"},
				},
				OutputSchema: changeCohesionSchema(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.JudgmentJSON).NotTo(BeEmpty())

			var j changeCohesionJudgment
			Expect(json.Unmarshal(resp.JudgmentJSON, &j)).To(Succeed())
			Expect(j.Judgment).To(Equal("focused"))
			Expect(j.Confidence).To(Equal("medium"))
		})
	})

	When("POST /v1/chat/completions carries a multi-finding hidden-mutation pack prompt", func() {
		It("returns a batch envelope that validates against the batch OutputSchema (compose platform-smoke path)", func() {
			// Mirrors worker → aigw → model-stub when smoke packs 2 fixture signals.
			batchSchema := json.RawMessage(`{
				"type":"object",
				"required":["items"],
				"additionalProperties":false,
				"properties":{
					"items":{
						"type":"array",
						"items":{
							"type":"object",
							"required":["finding_ref","judgment","rationale","confidence","suggested_focus"],
							"additionalProperties":false,
							"properties":{
								"finding_ref":{"type":"string"},
								"judgment":{"type":"string","enum":["concern","acceptable","unclear"]},
								"rationale":{"type":"string"},
								"confidence":{"type":"string","enum":["high","medium","low"]},
								"suggested_focus":{"type":["string","null"]}
							}
						}
					}
				}
			}`)
			client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
				BaseURL:      srv.URL,
				LogicalModel: "local",
				HTTPClient:   &http.Client{Timeout: 5 * time.Second},
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.Judge(context.Background(), modelgateway.JudgmentRequest{
				RubricID: "hidden_mutation_contextualization",
				Messages: []modelgateway.Message{
					{Role: "system", Content: "You are a code-quality judge for a hidden-mutation judgment pack."},
					{Role: "user", Content: "## Hidden-mutation judgment pack\nfinding_ref: hash-a\nhidden_input_mutation\nfinding_ref: hash-b\nhidden_input_mutation\n"},
				},
				OutputSchema: batchSchema,
			})
			Expect(err).NotTo(HaveOccurred(),
				"HTTP model-stub must return batch items JSON for pack prompts; singular JSON fails schema validation and platform-smoke loses source=agent")
			var env struct {
				Items []struct {
					FindingRef string `json:"finding_ref"`
					Judgment   string `json:"judgment"`
				} `json:"items"`
			}
			Expect(json.Unmarshal(resp.JudgmentJSON, &env)).To(Succeed())
			Expect(env.Items).To(HaveLen(2))
			refs := []string{env.Items[0].FindingRef, env.Items[1].FindingRef}
			Expect(refs).To(ConsistOf("hash-a", "hash-b"))
		})
	})

	When("POST /v1/chat/completions receives invalid JSON", func() {
		It("returns 400 rather than a fake completion", func() {
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/chat/completions", bytes.NewReader([]byte(`{not-json`)))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})
})
