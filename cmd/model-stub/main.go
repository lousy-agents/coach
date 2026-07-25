// Command model-stub is a minimal OpenAI-compatible /v1/chat/completions HTTP
// server for the local platform compose stack. It returns canned schema-valid
// rubric judgments so core smoke can exercise worker → Envoy AI Gateway →
// backend without model weights. Not a production inference backend.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultAddr = ":8090"

func main() {
	addr := os.Getenv("COACH_MODEL_STUB_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("POST /v1/chat/completions", handleChatCompletions)
	mux.HandleFunc("POST /chat/completions", handleChatCompletions)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("model-stub: listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("model-stub: %v", err)
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"invalid json"}}`, http.StatusBadRequest)
		return
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "stub"
	}
	content := judgmentForMessages(req.Messages)
	resp := map[string]any{
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
	_ = json.NewEncoder(w).Encode(resp)
}

func judgmentForMessages(msgs []chatMessage) string {
	joined := strings.Builder{}
	for _, m := range msgs {
		joined.WriteString(m.Content)
		joined.WriteByte('\n')
	}
	text := strings.ToLower(joined.String())
	switch {
	case strings.Contains(text, "hidden_input_mutation") ||
		strings.Contains(text, "hidden mutation") ||
		strings.Contains(text, "hidden_mutation"):
		return `{
			"judgment": "acceptable",
			"rationale": "model-stub: no surprising hidden mutation in fixture",
			"confidence": "high",
			"suggested_focus": null
		}`
	case strings.Contains(text, "cohes") ||
		strings.Contains(text, "cluster into coherent"):
		return `{
			"judgment": "focused",
			"rationale": "model-stub: findings cluster coherently in fixture",
			"confidence": "medium",
			"suggested_focus": null
		}`
	default:
		// Default to hidden-mutation shape; schema retries may still fail for
		// unknown prompts — that path is covered by unit tests elsewhere.
		return `{
			"judgment": "acceptable",
			"rationale": "model-stub: default canned judgment",
			"confidence": "low",
			"suggested_focus": null
		}`
	}
}
