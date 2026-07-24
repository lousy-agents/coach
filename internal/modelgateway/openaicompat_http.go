package modelgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxUpstreamResponseBytes = 8 << 20

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
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

func (c *OpenAICompatClient) callChatCompletions(ctx context.Context, logicalModel string, messages []Message) (servedModel, content string, err error) {
	body, err := marshalChatCompletionRequest(logicalModel, messages)
	if err != nil {
		return "", "", err
	}

	httpReq, err := c.newChatCompletionsRequest(ctx, body)
	if err != nil {
		return "", "", err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", "", NewUnavailableError("upstream request failed", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBytes))
	if err != nil {
		return "", "", NewUnavailableError("read upstream response", err)
	}
	if err := classifyUpstreamStatus(resp.StatusCode); err != nil {
		return "", "", err
	}
	return parseChatCompletionBody(respBody)
}

func marshalChatCompletionRequest(logicalModel string, messages []Message) ([]byte, error) {
	wireMsgs := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		wireMsgs = append(wireMsgs, chatMessage{Role: m.Role, Content: m.Content})
	}
	body, err := json.Marshal(chatCompletionRequest{
		Model:    logicalModel,
		Messages: wireMsgs,
		Stream:   false,
	})
	if err != nil {
		return nil, NewUnavailableError("encode request", err)
	}
	return body, nil
}

func (c *OpenAICompatClient) newChatCompletionsRequest(ctx context.Context, body []byte) (*http.Request, error) {
	endpoint := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, NewUnavailableError("build request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	c.applyAuthHeaders(httpReq)
	for k, v := range c.extraHeaders {
		httpReq.Header.Set(k, v)
	}
	return httpReq, nil
}

func (c *OpenAICompatClient) applyAuthHeaders(httpReq *http.Request) {
	if c.authHeader != "" {
		httpReq.Header.Set("Authorization", c.authHeader)
		return
	}
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func classifyUpstreamStatus(status int) error {
	if status >= 200 && status < 300 {
		return nil
	}
	// 5xx, 408, 429, and other non-2xx are unavailability of inference for callers.
	return NewUnavailableError(fmt.Sprintf("upstream HTTP %d", status), nil)
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
