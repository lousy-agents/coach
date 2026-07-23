package modelgateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
		RubricID:      "hidden_mutation_contextualization",
		RubricVersion: "1",
		Messages: []modelgateway.Message{
			{Role: "system", Content: "You are a rubric judge."},
			{Role: "user", Content: "Judge this change for hidden mutation."},
		},
		OutputSchema: hiddenMutationSchema(),
	}
}

func newCompatClient(srv *httptest.Server, cfg modelgateway.OpenAICompatConfig) *modelgateway.OpenAICompatClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = srv.URL
	}
	if cfg.LogicalModel == "" {
		cfg.LogicalModel = modelgateway.DefaultLogicalModel
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = srv.Client()
	}
	client, err := modelgateway.NewOpenAICompatClient(cfg)
	Expect(err).NotTo(HaveOccurred())
	return client
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

				var gw modelgateway.Gateway = newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				resp, err := gw.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())

				Expect(gotPath).To(Equal("/v1/chat/completions"))
				Expect(gotBody["model"]).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(gotBody["stream"]).To(Equal(false))
				msgs, ok := gotBody["messages"].([]any)
				Expect(ok).To(BeTrue())
				Expect(msgs).To(HaveLen(2))
				msg0, ok := msgs[0].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(msg0["role"]).To(Equal("system"))
				Expect(msg0["content"]).To(Equal("You are a rubric judge."))
				msg1, ok := msgs[1].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(msg1["role"]).To(Equal("user"))
				Expect(msg1["content"]).To(Equal("Judge this change for hidden mutation."))
				// Portable OpenAI shape only — keys the client may emit.
				for key := range gotBody {
					Expect(key).To(BeElementOf("model", "messages", "stream"))
				}

				Expect(resp.LogicalModelID).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(resp.ServedModelID).To(Equal("served-concrete-model-v1"))
				Expect(resp.JudgmentJSON).NotTo(BeEmpty())

				var got hiddenMutationJudgment
				Expect(json.Unmarshal(resp.JudgmentJSON, &got)).To(Succeed())
				Expect(got.Judgment).To(Equal("acceptable"))
				Expect(got.Rationale).NotTo(BeEmpty())
				Expect(got.Confidence).To(Equal("high"))
				Expect(got.SuggestedFocus).To(BeNil())
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

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				resp, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.LogicalModelID).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(resp.ServedModelID).To(Equal("qwen2.5-coder-7b-instruct-q4_k_m"))
				Expect(resp.LogicalModelID).NotTo(Equal(resp.ServedModelID))
			})
		})

		When("OutputSchema uses types outside the seed-rubric subset", func() {
			It("returns typed ErrSchemaValidation for integer property schemas instead of accepting JSON numbers", func() {
				integerSchema := json.RawMessage(`{
					"type": "object",
					"required": ["score"],
					"properties": {
						"score": {"type": "integer"}
					}
				}`)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-x", `{"score": 1}`))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				req := defaultJudgeRequest()
				req.OutputSchema = integerSchema
				_, err := client.Judge(context.Background(), req)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeFalse())
			})
		})

		When("the assistant content is malformed or fails OutputSchema", func() {
			It("returns typed ErrSchemaValidation after exactly DefaultSchemaValidationAttempts on invalid enum", func() {
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-x", `{"judgment":"not-an-enum","rationale":"x","confidence":"high","suggested_focus":null}`))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				_, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeFalse())
				Expect(attempts.Load()).To(Equal(int32(modelgateway.DefaultSchemaValidationAttempts)))
			})

			It("returns typed ErrSchemaValidation after exactly DefaultSchemaValidationAttempts on non-JSON content", func() {
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-x", `this is not json`))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				_, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(attempts.Load()).To(Equal(int32(modelgateway.DefaultSchemaValidationAttempts)))
			})

			It("returns typed ErrSchemaValidation when a required OutputSchema property is missing", func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-x", `{"judgment":"acceptable","rationale":"x","confidence":"high"}`))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				_, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeFalse())
			})

			It("accepts assistant content that is a JSON string wrapping the judgment object", func() {
				wrapped, err := json.Marshal(validHiddenMutationContent())
				Expect(err).NotTo(HaveOccurred())
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-wrap", string(wrapped)))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				resp, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.ServedModelID).To(Equal("served-wrap"))
				var got hiddenMutationJudgment
				Expect(json.Unmarshal(resp.JudgmentJSON, &got)).To(Succeed())
				Expect(got.Judgment).To(Equal("acceptable"))
			})
		})

		When("the upstream is unavailable", func() {
			It("returns typed ErrUnavailable on HTTP 5xx with a single attempt", func() {
				var attempts atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{"error":"bad gateway"}`))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				_, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeFalse())
				Expect(attempts.Load()).To(Equal(int32(1)))
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

			It("returns typed ErrUnavailable when the upstream envelope has no choices", func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id":"chatcmpl-x","object":"chat.completion","model":"m","choices":[]}`))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{})
				_, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeFalse())
			})
		})

		When("optional credentials are configured", func() {
			It("sends Bearer Authorization from APIKey and extra headers", func() {
				var gotAuth, gotExtra string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotAuth = r.Header.Get("Authorization")
					gotExtra = r.Header.Get("X-Request-Source")
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-y", validHiddenMutationContent()))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{
					APIKey:       "test-key",
					ExtraHeaders: map[string]string{"X-Request-Source": "coach-acceptance"},
				})
				_, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(gotAuth).To(Equal("Bearer test-key"))
				Expect(gotExtra).To(Equal("coach-acceptance"))
			})

			It("sends AuthHeader as Authorization as-is when set", func() {
				var gotAuth string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotAuth = r.Header.Get("Authorization")
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("served-y", validHiddenMutationContent()))
				}))
				DeferCleanup(srv.Close)

				client := newCompatClient(srv, modelgateway.OpenAICompatConfig{
					APIKey:     "ignored-when-auth-header-set",
					AuthHeader: "Bearer edge-injected-token",
				})
				_, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(gotAuth).To(Equal("Bearer edge-injected-token"))
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
				Expect(client.HTTPClient().Timeout).To(Equal(modelgateway.DefaultHTTPClientTimeout))
				// Must not be the shared DefaultClient (Timeout is 0 and is process-global).
				Expect(client.HTTPClient()).NotTo(BeIdenticalTo(http.DefaultClient))
			})
		})
	})

	Describe("thin env config", func() {
		When("MODEL_GATEWAY_BASE_URL and MODEL_GATEWAY_MODEL are set", func() {
			It("constructs an OpenAICompatClient from environment and completes a judgment", func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(chatCompletionBody("env-served", validHiddenMutationContent()))
				}))
				DeferCleanup(srv.Close)

				origBase := os.Getenv("MODEL_GATEWAY_BASE_URL")
				origModel := os.Getenv("MODEL_GATEWAY_MODEL")
				origKey := os.Getenv("MODEL_GATEWAY_API_KEY")
				origTimeout := os.Getenv("MODEL_GATEWAY_TIMEOUT")
				DeferCleanup(func() {
					_ = os.Setenv("MODEL_GATEWAY_BASE_URL", origBase)
					_ = os.Setenv("MODEL_GATEWAY_MODEL", origModel)
					_ = os.Setenv("MODEL_GATEWAY_API_KEY", origKey)
					_ = os.Setenv("MODEL_GATEWAY_TIMEOUT", origTimeout)
				})
				Expect(os.Setenv("MODEL_GATEWAY_BASE_URL", srv.URL)).To(Succeed())
				Expect(os.Setenv("MODEL_GATEWAY_MODEL", modelgateway.DefaultLogicalModel)).To(Succeed())
				Expect(os.Setenv("MODEL_GATEWAY_API_KEY", "")).To(Succeed())
				Expect(os.Unsetenv("MODEL_GATEWAY_TIMEOUT")).To(Succeed())

				cfg, err := modelgateway.ConfigFromEnv()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.BaseURL).To(Equal(srv.URL))
				Expect(cfg.LogicalModel).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(cfg.HTTPClient).NotTo(BeNil())
				Expect(cfg.HTTPClient.Timeout).To(Equal(modelgateway.DefaultHTTPClientTimeout))

				cfg.HTTPClient = srv.Client()
				client, err := modelgateway.NewOpenAICompatClient(cfg)
				Expect(err).NotTo(HaveOccurred())

				resp, err := client.Judge(context.Background(), defaultJudgeRequest())
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.LogicalModelID).To(Equal(modelgateway.DefaultLogicalModel))
				Expect(resp.ServedModelID).To(Equal("env-served"))
			})
		})

		When("MODEL_GATEWAY_TIMEOUT is set", func() {
			It("applies the configured timeout to the HTTP client", func() {
				origBase := os.Getenv("MODEL_GATEWAY_BASE_URL")
				origTimeout := os.Getenv("MODEL_GATEWAY_TIMEOUT")
				DeferCleanup(func() {
					_ = os.Setenv("MODEL_GATEWAY_BASE_URL", origBase)
					_ = os.Setenv("MODEL_GATEWAY_TIMEOUT", origTimeout)
				})
				Expect(os.Setenv("MODEL_GATEWAY_BASE_URL", "http://127.0.0.1:1")).To(Succeed())
				Expect(os.Setenv("MODEL_GATEWAY_TIMEOUT", "250ms")).To(Succeed())

				cfg, err := modelgateway.ConfigFromEnv()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.HTTPClient).NotTo(BeNil())
				Expect(cfg.HTTPClient.Timeout).To(Equal(250 * time.Millisecond))
			})
		})

		When("MODEL_GATEWAY_BASE_URL is unset", func() {
			It("returns an error and does not invent a base URL", func() {
				origBase := os.Getenv("MODEL_GATEWAY_BASE_URL")
				DeferCleanup(func() {
					_ = os.Setenv("MODEL_GATEWAY_BASE_URL", origBase)
				})
				Expect(os.Unsetenv("MODEL_GATEWAY_BASE_URL")).To(Succeed())

				_, err := modelgateway.ConfigFromEnv()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("MODEL_GATEWAY_BASE_URL"))
			})
		})
	})
})
