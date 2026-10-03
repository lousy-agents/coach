package modelgateway

import (
	"encoding/json"
	"fmt"

	"net/http"
)

func (c *OpenAICompatClient) applyAuthHeaders(httpReq *http.Request) {
	if c.authHeader != "" {
		httpReq.Header.Set("Authorization", c.authHeader)
		return
	}
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
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
func classifyUpstreamStatus(status int) error {
	if status >= 200 && status < 300 {
		return nil
	}

	return NewUnavailableError(fmt.Sprintf("upstream HTTP %d", status), nil)
}
