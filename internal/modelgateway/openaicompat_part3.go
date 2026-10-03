package modelgateway

import (
	"net/http"
)

func resolveSchemaAttempts(n int) int {
	if n <= 0 {
		return DefaultSchemaValidationAttempts
	}
	return n
}
func (c *OpenAICompatClient) HTTPClient() *http.Client {
	if c == nil {
		return nil
	}
	return c.httpClient
}
