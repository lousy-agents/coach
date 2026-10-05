package modelgateway

import (
	"encoding/json"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	// Think is Ollama-style; omitempty keeps portable OpenAI bodies clean.
	Think *bool `json:"think,omitempty"`
}

type chatCompletionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func marshalChatCompletionRequest(logicalModel string, messages []Message, disableThinking bool) ([]byte, error) {
	wireMsgs := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		wireMsgs = append(wireMsgs, chatMessage{Role: m.Role, Content: m.Content})
	}
	req := chatCompletionRequest{
		Model:    logicalModel,
		Messages: wireMsgs,
		Stream:   false,
	}
	if disableThinking {
		// Ollama-style: think:false disables reasoning-only channels.
		f := false
		req.Think = &f
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, NewUnavailableError("encode request", err)
	}
	return body, nil
}

func parseChatCompletionBody(respBody []byte) (servedModel, content string, err error) {
	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", "", NewUnavailableError("decode upstream response", err)
	}
	if len(parsed.Choices) == 0 {
		return "", "", NewUnavailableError("upstream response missing choices", nil)
	}
	return parsed.Model, parsed.Choices[0].Message.Content, nil
}
