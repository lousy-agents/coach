package modelgateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// NewHTTPStubHandler returns an OpenAI-compatible HTTP handler for local
// compose smoke: POST /v1/chat/completions (and /chat/completions) with canned
// schema-valid rubric judgments, plus GET /healthz. Used by cmd/model-stub
// behind Envoy AI Gateway; not a production inference backend.
func NewHTTPStubHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("POST /v1/chat/completions", handleHTTPStubChatCompletions)
	mux.HandleFunc("POST /chat/completions", handleHTTPStubChatCompletions)
	return mux
}

// ListenAndServeHTTPStub serves NewHTTPStubHandler on addr until the server
// exits. ReadHeaderTimeout is set per AGENTS.md outbound/inbound HTTP policy.
func ListenAndServeHTTPStub(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           NewHTTPStubHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

func handleHTTPStubChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req chatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"invalid json"}}`, http.StatusBadRequest)
		return
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = LogicalModelStub
	}
	content := httpStubJudgmentForMessages(req.Messages)
	out := map[string]any{
		"id":      "chatcmpl-model-stub",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func httpStubJudgmentForMessages(msgs []chatMessage) string {
	var b strings.Builder
	gwMsgs := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		b.WriteString(m.Content)
		b.WriteByte('\n')
		gwMsgs = append(gwMsgs, Message{Role: m.Role, Content: m.Content})
	}
	text := strings.ToLower(b.String())

	// Multi-finding pack prompts (local-LLM judgment packing) require a batch
	// envelope with finding_ref items. Returning singular JSON fails OpenAI-compat
	// schema validation and zeros source=agent rows in compose platform-smoke.
	if isHTTPStubPackPrompt(text, gwMsgs) {
		return string(stubBatchJudgment(JudgmentRequest{Messages: gwMsgs}))
	}

	switch {
	case strings.Contains(text, "hidden_input_mutation") ||
		strings.Contains(text, "hidden mutation") ||
		strings.Contains(text, "hidden_mutation"):
		if raw, ok := stubJudgmentForRubric("hidden_mutation_contextualization"); ok {
			return string(raw)
		}
	case strings.Contains(text, "cohes") ||
		strings.Contains(text, "cluster into coherent"):
		if raw, ok := stubJudgmentForRubric("change_cohesion"); ok {
			return string(raw)
		}
	}
	if raw, ok := stubJudgmentForRubric("hidden_mutation_contextualization"); ok {
		return string(raw)
	}
	return `{"judgment":"unclear","rationale":"model-stub: no canned judgment","confidence":"low","suggested_focus":null}`
}

func isHTTPStubPackPrompt(lowerText string, msgs []Message) bool {
	if strings.Contains(lowerText, "judgment pack") ||
		strings.Contains(lowerText, "hidden-mutation judgment pack") {
		return true
	}
	// Two or more finding_ref lines ⇒ pack prompt from AssembleHiddenMutationPackMessages.
	return len(extractFindingRefsFromMessages(msgs)) >= 2
}
