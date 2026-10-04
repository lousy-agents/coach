package modelgateway

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// normalizeBaseURL trims whitespace and trailing slashes, and strips a single
// trailing "/v1" so operators may pass either the origin or the OpenAI API root
// without producing /v1/v1/chat/completions.
func normalizeBaseURL(raw string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimRight(strings.TrimSuffix(base, "/v1"), "/")
	}
	if base == "" {
		return "", fmt.Errorf("modelgateway: BaseURL is required")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return "", fmt.Errorf("modelgateway: BaseURL is invalid: %w", err)
	}
	return base, nil
}
func resolveHTTPClient(c *http.Client) (*http.Client, error) {
	if c == nil {
		return &http.Client{Timeout: DefaultHTTPClientTimeout}, nil
	}
	if c == http.DefaultClient {
		return nil, fmt.Errorf("modelgateway: HTTPClient must not be http.DefaultClient")
	}
	if c.Timeout <= 0 {
		return nil, fmt.Errorf("modelgateway: HTTPClient.Timeout must be > 0")
	}
	return c, nil
}
func resolveLogicalModel(requestModel, clientDefault string) string {
	if m := strings.TrimSpace(requestModel); m != "" {
		return m
	}
	if m := strings.TrimSpace(clientDefault); m != "" {
		return m
	}
	return DefaultLogicalModel
}
func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
