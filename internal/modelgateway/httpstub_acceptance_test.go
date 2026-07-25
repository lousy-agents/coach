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
