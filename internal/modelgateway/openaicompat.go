package modelgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultHTTPClientTimeout bounds outbound chat-completions calls when
// OpenAICompatConfig.HTTPClient is nil.
const DefaultHTTPClientTimeout = 10 * time.Second

// DefaultSchemaValidationAttempts is the maximum number of upstream calls
// when assistant content fails JSON parse or OutputSchema validation.
const DefaultSchemaValidationAttempts = 3

// OpenAICompatConfig configures the OpenAI-compatible HTTP Gateway adapter.
// It is intentionally provider-neutral: base URL, logical model, optional
// credentials/headers, and HTTP client only.
type OpenAICompatConfig struct {
	BaseURL      string
	LogicalModel string
	APIKey       string
	// AuthHeader, when set, is used as the Authorization header value as-is
	// instead of "Bearer "+APIKey.
	AuthHeader   string
	ExtraHeaders map[string]string
	// HTTPClient is optional; when nil a client with DefaultHTTPClientTimeout is used.
	// When non-nil it must not be http.DefaultClient and must have Timeout > 0.
	HTTPClient *http.Client
	// SchemaValidationAttempts overrides DefaultSchemaValidationAttempts when > 0.
	SchemaValidationAttempts int
}

// OpenAICompatClient implements Gateway via POST {baseURL}/v1/chat/completions.
type OpenAICompatClient struct {
	baseURL                  string
	logicalModel             string
	apiKey                   string
	authHeader               string
	extraHeaders             map[string]string
	httpClient               *http.Client
	schemaValidationAttempts int
}

var _ Gateway = (*OpenAICompatClient)(nil)

func NewOpenAICompatClient(cfg OpenAICompatConfig) (*OpenAICompatClient, error) {
	base, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	logical := strings.TrimSpace(cfg.LogicalModel)
	if logical == "" {
		logical = DefaultLogicalModel
	}

	httpClient, err := resolveHTTPClient(cfg.HTTPClient)
	if err != nil {
		return nil, err
	}

	attempts := cfg.SchemaValidationAttempts
	if attempts <= 0 {
		attempts = DefaultSchemaValidationAttempts
	}

	var extra map[string]string
	if len(cfg.ExtraHeaders) > 0 {
		extra = make(map[string]string, len(cfg.ExtraHeaders))
		for k, v := range cfg.ExtraHeaders {
			extra[k] = v
		}
	}

	return &OpenAICompatClient{
		baseURL:                  base,
		logicalModel:             logical,
		apiKey:                   cfg.APIKey,
		authHeader:               cfg.AuthHeader,
		extraHeaders:             extra,
		httpClient:               httpClient,
		schemaValidationAttempts: attempts,
	}, nil
}

// normalizeBaseURL trims whitespace and trailing slashes, and strips a single
// trailing "/v1" so operators may pass either the origin or the OpenAI API root
// without producing /v1/v1/chat/completions.
func normalizeBaseURL(raw string) (string, error) {
	base := strings.TrimSpace(raw)
	base = strings.TrimRight(base, "/")
	if base == "" {
		return "", fmt.Errorf("modelgateway: BaseURL is required")
	}
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
		base = strings.TrimRight(base, "/")
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

func (c *OpenAICompatClient) HTTPClient() *http.Client {
	if c == nil {
		return nil
	}
	return c.httpClient
}

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

func (c *OpenAICompatClient) Judge(ctx context.Context, req JudgmentRequest) (JudgmentResponse, error) {
	if c == nil {
		return JudgmentResponse{}, NewUnavailableError("nil client", nil)
	}
	if err := ctx.Err(); err != nil {
		return JudgmentResponse{}, NewUnavailableError("context done", err)
	}

	logical := strings.TrimSpace(req.LogicalModel)
	if logical == "" {
		logical = c.logicalModel
	}
	if logical == "" {
		logical = DefaultLogicalModel
	}

	// Fail closed on static schema problems before any upstream call so the
	// bounded retry budget is reserved for malformed model output only.
	if err := validateOutputSchema(req.OutputSchema); err != nil {
		return JudgmentResponse{}, err
	}

	var lastValErr error
	for attempt := 1; attempt <= c.schemaValidationAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return JudgmentResponse{}, NewUnavailableError("context done", err)
		}

		servedModel, content, err := c.callChatCompletions(ctx, logical, req.Messages)
		if err != nil {
			return JudgmentResponse{}, err
		}

		judgment, valErr := extractAndValidateJudgment(content, req.OutputSchema)
		if valErr != nil {
			lastValErr = valErr
			continue
		}

		return JudgmentResponse{
			JudgmentJSON:   judgment,
			LogicalModelID: logical,
			ServedModelID:  servedModel,
		}, nil
	}

	if lastValErr == nil {
		lastValErr = NewValidationError("schema validation failed after retries")
	}
	return JudgmentResponse{}, lastValErr
}

func extractAndValidateJudgment(content string, schema json.RawMessage) (json.RawMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, NewValidationError("assistant content is empty")
	}
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, NewValidationError("assistant content is not valid JSON")
	}
	// Models sometimes return a JSON string whose contents are the object; unwrap once.
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return nil, NewValidationError("assistant content is empty")
		}
		if err := json.Unmarshal([]byte(asString), &raw); err != nil {
			return nil, NewValidationError("assistant content is not valid JSON")
		}
	}
	if err := validateJudgmentJSON(raw, schema); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *OpenAICompatClient) callChatCompletions(ctx context.Context, logicalModel string, messages []Message) (servedModel, content string, err error) {
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
		return "", "", NewUnavailableError("encode request", err)
	}

	endpoint := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", NewUnavailableError("build request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.authHeader != "" {
		httpReq.Header.Set("Authorization", c.authHeader)
	} else if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.extraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", "", NewUnavailableError("upstream request failed", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", "", NewUnavailableError("read upstream response", err)
	}

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests {
		return "", "", NewUnavailableError(fmt.Sprintf("upstream HTTP %d", resp.StatusCode), nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Non-retryable client/auth errors are still unavailability of inference for callers.
		return "", "", NewUnavailableError(fmt.Sprintf("upstream HTTP %d", resp.StatusCode), nil)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", "", NewUnavailableError("decode upstream response", err)
	}
	if len(parsed.Choices) == 0 {
		return "", "", NewUnavailableError("upstream response missing choices", nil)
	}
	return parsed.Model, parsed.Choices[0].Message.Content, nil
}
