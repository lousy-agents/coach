package modelgateway

import (
	"net/http"
)

func (c *OpenAICompatClient) HTTPClient() *http.Client {
	if c == nil {
		return nil
	}
	return c.httpClient
}
