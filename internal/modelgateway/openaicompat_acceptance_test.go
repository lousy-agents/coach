package modelgateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

func validHiddenMutationContent() string {
	return `{
		"judgment": "acceptable",
		"rationale": "fixture: no hidden mutation signals",
		"confidence": "high",
		"suggested_focus": null
	}`
}

func chatCompletionBody(servedModel, content string) []byte {
	body, err := json.Marshal(map[string]any{
		"id":     "chatcmpl-fixture",
		"object": "chat.completion",
		"model":  servedModel,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     1,
			"completion_tokens": 1,
			"total_tokens":      2,
		},
	})
	Expect(err).NotTo(HaveOccurred())
	return body
}

func defaultJudgeRequest() modelgateway.JudgmentRequest {
	return modelgateway.JudgmentRequest{
		RubricID:      "hidden_mutation",
		RubricVersion: "1",
		Messages: []modelgateway.Message{
			{Role: "system", Content: "You are a rubric judge."},
			{Role: "user", Content: "Judge this change for hidden mutation."},
		},
		OutputSchema: hiddenMutationSchema(),
	}
}

var _ = Describe("modelgateway.OpenAICompatClient", func() {
	Describe("Gateway contract over OpenAI-compatible chat completions", func() {
		When("a successful non-streaming chat completion is returned", func() {
			It("POSTs {baseURL}/v1/chat/completions with the logical model and returns schema-valid judgment plus provenance", func() {
				var gotPath string
				var gotBody map[string]any
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotPath = r.URL.Path
					Expect(r.Method).To(Equal(http.MethodPost))
					raw, err := io.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(json.Unmarshal(raw, &gotBody)).To(Succeed())
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-concrete-model-v1", validHiddenMutationContent()))
				}))
				DeferCleanup(srv.Close)

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   srv.Client(),
				})
				Expect(err).NotTo(HaveOccurred())

				var gw modelgateway.Gateway = client
				resp, err := gw.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())

				Expect(gotPath).To(Equal("/v1/chat/completions"))
				Expect(gotBody["model"]).To(Equal(modelgateway.DefaultLogicalModel))
				msgs, ok := gotBody["messages"].([]any)
				Expect(ok).To(BeTrue())
				Expect(msgs).NotTo(BeEmpty())
				// Portable OpenAI chat-completions shape only — no vendor-specific request fields.
				for key := range gotBody {
					Expect(key).To(BeElementOf(
						"model", "messages", "stream", "response_format",
						"temperature", "max_tokens", "n",
					))
				}

				Expect(resp.LogicalModelID).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(resp.ServedModelID).To(Equal("served-concrete-model-v1"))
				Expect(resp.JudgmentJSON).NotTo(BeEmpty())

				var got hiddenMutationJudgment
				Expect(json.Unmarshal(resp.JudgmentJSON, &got)).To(Succeed())
				Expect(got.Judgment).To(Equal("acceptable"))
				Expect(got.Rationale).NotTo(BeEmpty())
				Expect(got.Confidence).To(Equal("high"))
			})
		})

		When("an edge virtualizes the logical model name to a different concrete served id", func() {
			It("keeps both LogicalModelID and ServedModelID without client failure", func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					raw, err := io.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					var body map[string]any
					Expect(json.Unmarshal(raw, &body)).To(Succeed())
					Expect(body["model"]).To(Equal(modelgateway.DefaultLogicalModel))
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("qwen2.5-coder-7b-instruct-q4_k_m", validHiddenMutationContent()))
				}))
				DeferCleanup(srv.Close)

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   srv.Client(),
				})
				Expect(err).NotTo(HaveOccurred())

				resp, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.LogicalModelID).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(resp.ServedModelID).To(Equal("qwen2.5-coder-7b-instruct-q4_k_m"))
				Expect(resp.LogicalModelID).NotTo(Equal(resp.ServedModelID))
			})
		})

		When("the assistant content is malformed or fails OutputSchema", func() {
			It("returns typed ErrSchemaValidation after bounded retries on validation/parse only", func() {
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					// Valid envelope, invalid judgment JSON relative to OutputSchema.
					_, _ = w.Write(chatCompletionBody("served-x", `{"judgment":"not-an-enum","rationale":"x","confidence":"high","suggested_focus":null}`))
				}))
				DeferCleanup(srv.Close)

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   srv.Client(),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeFalse())

				// Bounded retries: more than one attempt, but not unbounded.
				n := attempts.Load()
				Expect(n).To(BeNumerically(">=", 2))
				Expect(n).To(BeNumerically("<=", 4))
			})

			It("retries on non-JSON assistant content then fails with ErrSchemaValidation", func() {
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-x", `this is not json`))
				}))
				DeferCleanup(srv.Close)

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   srv.Client(),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(attempts.Load()).To(BeNumerically(">=", 2))
				Expect(attempts.Load()).To(BeNumerically("<=", 4))
			})
		})

		When("the upstream is unavailable", func() {
			It("returns typed ErrUnavailable on HTTP 5xx without schema-validation retries", func() {
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{"error":"bad gateway"}`))
				}))
				DeferCleanup(srv.Close)

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   srv.Client(),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeFalse())
				// Prefer prompt typed unavailable — do not busy-retry 5xx like validation failures.
				Expect(attempts.Load()).To(BeNumerically("<=", 2))
			})

			It("returns typed ErrUnavailable on connection refused", func() {
				closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				baseURL := closed.URL
				closed.Close()

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      baseURL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   &http.Client{Timeout: 2 * time.Second},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeFalse())
			})

			It("returns typed ErrUnavailable on client timeout", func() {
				// Block longer than the client timeout so http.Client.Do fails with a
				// timeout (not connection refused / 5xx). Unblock on cleanup so the
				// handler can exit before the server closes.
				block := make(chan struct{})
				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					<-block
				}))
				DeferCleanup(func() {
					close(block)
					srv.Close()
				})

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					HTTPClient:   &http.Client{Timeout: 50 * time.Millisecond},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeFalse())
			})
		})

		When("optional API key and extra headers are configured", func() {
			It("sends Authorization and extra headers on the chat-completions request", func() {
				var gotAuth, gotExtra string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotAuth = r.Header.Get("Authorization")
					gotExtra = r.Header.Get("X-Request-Source")
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-y", validHiddenMutationContent()))
				}))
				DeferCleanup(srv.Close)

				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      srv.URL,
					LogicalModel: modelgateway.DefaultLogicalModel,
					APIKey:       "test-key",
					ExtraHeaders: map[string]string{"X-Request-Source": "coach-acceptance"},
					HTTPClient:   srv.Client(),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(gotAuth).To(Equal("Bearer test-key"))
				Expect(gotExtra).To(Equal("coach-acceptance"))
			})
		})

		When("HTTPClient is nil", func() {
			It("uses a client with a finite default timeout (never bare http.DefaultClient)", func() {
				client, err := modelgateway.NewOpenAICompatClient(modelgateway.OpenAICompatConfig{
					BaseURL:      "http://127.0.0.1:1",
					LogicalModel: modelgateway.DefaultLogicalModel,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.HTTPClient()).NotTo(BeNil())
				Expect(client.HTTPClient().Timeout).To(BeNumerically(">", 0))
				// Must not be the shared DefaultClient (Timeout is 0 and is process-global).
				Expect(client.HTTPClient()).NotTo(BeIdenticalTo(http.DefaultClient))
			})
		})
	})

	Describe("thin env config", func() {
		When("MODEL_GATEWAY_BASE_URL and MODEL_GATEWAY_MODEL are set", func() {
			It("constructs an OpenAICompatClient from environment without provider-specific types", func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("env-served", validHiddenMutationContent()))
				}))
				DeferCleanup(srv.Close)

				origBase := os.Getenv("MODEL_GATEWAY_BASE_URL")
				origModel := os.Getenv("MODEL_GATEWAY_MODEL")
				origKey := os.Getenv("MODEL_GATEWAY_API_KEY")
				DeferCleanup(func() {
					_ = os.Setenv("MODEL_GATEWAY_BASE_URL", origBase)
					_ = os.Setenv("MODEL_GATEWAY_MODEL", origModel)
					_ = os.Setenv("MODEL_GATEWAY_API_KEY", origKey)
				})
				Expect(os.Setenv("MODEL_GATEWAY_BASE_URL", srv.URL)).To(Succeed())
				Expect(os.Setenv("MODEL_GATEWAY_MODEL", modelgateway.DefaultLogicalModel)).To(Succeed())
				Expect(os.Setenv("MODEL_GATEWAY_API_KEY", "")).To(Succeed())

				cfg, err := modelgateway.ConfigFromEnv()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.BaseURL).To(Equal(srv.URL))
				Expect(cfg.LogicalModel).To(Equal(modelgateway.DefaultLogicalModel))
				// Config surface is URL/model/api-key/timeout only.
				Expect(strings.ToLower(cfg.BaseURL + cfg.LogicalModel)).NotTo(ContainSubstring("llamacpp"))
				Expect(strings.ToLower(cfg.BaseURL + cfg.LogicalModel)).NotTo(ContainSubstring("sglang"))

				cfg.HTTPClient = srv.Client()
				client, err := modelgateway.NewOpenAICompatClient(cfg)
				Expect(err).NotTo(HaveOccurred())

				resp, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.LogicalModelID).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(resp.ServedModelID).To(Equal("env-served"))
			})
		})
	})
})
