package modelgateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

const maxUpstreamResponseBytes = 8 << 20

func (c *OpenAICompatClient) callChatCompletions(ctx context.Context, logicalModel string, messages []Message) (servedModel, content string, err error) {
	body, err := marshalChatCompletionRequest(logicalModel, messages, c.disableThinking)
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
