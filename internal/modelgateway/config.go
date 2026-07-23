package modelgateway

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ConfigFromEnv builds an OpenAICompatConfig from environment variables:
// MODEL_GATEWAY_BASE_URL (required), MODEL_GATEWAY_MODEL (optional; defaults to
// DefaultLogicalModel), MODEL_GATEWAY_API_KEY (optional), MODEL_GATEWAY_TIMEOUT
// (optional duration, e.g. "10s").
func ConfigFromEnv() (OpenAICompatConfig, error) {
	baseURL := strings.TrimSpace(os.Getenv("MODEL_GATEWAY_BASE_URL"))
	if baseURL == "" {
		return OpenAICompatConfig{}, fmt.Errorf("modelgateway: MODEL_GATEWAY_BASE_URL is required")
	}
	model := strings.TrimSpace(os.Getenv("MODEL_GATEWAY_MODEL"))
	if model == "" {
		model = DefaultLogicalModel
	}
	apiKey := strings.TrimSpace(os.Getenv("MODEL_GATEWAY_API_KEY"))
	timeout := envDuration("MODEL_GATEWAY_TIMEOUT", DefaultHTTPClientTimeout)

	return OpenAICompatConfig{
		BaseURL:      baseURL,
		LogicalModel: model,
		APIKey:       apiKey,
		HTTPClient:   &http.Client{Timeout: timeout},
	}, nil
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return fallback
}
