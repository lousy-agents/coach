package modelgateway_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

func body_openaicompatAcceptanceTest_POSTsBaseURLV1ChatCompletionsWithTheLogicalModel_87() {
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
}

func body_openaicompatAcceptanceTest_includesThinkFalseOnTheChatCompletionsRequestBod_564() {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(raw, &gotBody)).To(Succeed())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatCompletionBody("served-think", validHiddenMutationContent()))
	}))
	DeferCleanup(srv.Close)

	client := newCompatClient(srv, modelgateway.OpenAICompatConfig{
		DisableThinking: true,
	})
	_, err := client.Judge(context.Background(), defaultJudgeRequest())
	Expect(err).NotTo(HaveOccurred())
	Expect(gotBody).To(HaveKeyWithValue("think", false))
	for key := range gotBody {
		Expect(key).To(BeElementOf("model", "messages", "stream", "think"))
	}
}

func body_openaicompatAcceptanceTest_omitsTheThinkFieldPortableOpenAIShape_588() {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(raw, &gotBody)).To(Succeed())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatCompletionBody("served-no-think", validHiddenMutationContent()))
	}))
	DeferCleanup(srv.Close)

	client := newCompatClient(srv, modelgateway.OpenAICompatConfig{
		DisableThinking: false,
	})
	_, err := client.Judge(context.Background(), defaultJudgeRequest())
	Expect(err).NotTo(HaveOccurred())
	Expect(gotBody).NotTo(HaveKey("think"))
	for key := range gotBody {
		Expect(key).To(BeElementOf("model", "messages", "stream"))
	}
}
