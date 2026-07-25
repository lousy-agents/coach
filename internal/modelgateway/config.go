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
// (optional duration, e.g. "10s"), MODEL_GATEWAY_DISABLE_THINKING (optional;
// "1"/"true" → DisableThinking so bodies include think:false; unset/0/false omit
// the field — document MODEL_GATEWAY_DISABLE_THINKING=1 on local llm Path B).
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
		BaseURL:         baseURL,
		LogicalModel:    model,
		APIKey:          apiKey,
		HTTPClient:      &http.Client{Timeout: timeout},
		DisableThinking: envBoolTrue("MODEL_GATEWAY_DISABLE_THINKING"),
	}, nil
}

// envBoolTrue reports whether key is set to a common true token (1/true/yes/on).
// Unset, empty, and false-like values return false (field omitted on wire).
func envBoolTrue(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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
