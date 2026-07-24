package modelgateway

import (
	"context"
	"fmt"
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
	httpClient, err := resolveHTTPClient(cfg.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &OpenAICompatClient{
		baseURL:                  base,
		logicalModel:             resolveLogicalModel(cfg.LogicalModel, ""),
		apiKey:                   cfg.APIKey,
		authHeader:               cfg.AuthHeader,
		extraHeaders:             cloneStringMap(cfg.ExtraHeaders),
		httpClient:               httpClient,
		schemaValidationAttempts: resolveSchemaAttempts(cfg.SchemaValidationAttempts),
	}, nil
}

func (c *OpenAICompatClient) HTTPClient() *http.Client {
	if c == nil {
		return nil
	}
	return c.httpClient
}

func (c *OpenAICompatClient) Judge(ctx context.Context, req JudgmentRequest) (JudgmentResponse, error) {
	if c == nil {
		return JudgmentResponse{}, NewUnavailableError("nil client", nil)
	}
	if err := ctx.Err(); err != nil {
		return JudgmentResponse{}, NewUnavailableError("context done", err)
	}

	logical := resolveLogicalModel(req.LogicalModel, c.logicalModel)

	// Fail closed on static schema problems before any upstream call so the
	// bounded retry budget is reserved for malformed model output only.
	if err := validateOutputSchema(req.OutputSchema); err != nil {
		return JudgmentResponse{}, err
	}

	return c.judgeWithRetries(ctx, logical, req)
}

func (c *OpenAICompatClient) judgeWithRetries(ctx context.Context, logical string, req JudgmentRequest) (JudgmentResponse, error) {
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

func resolveSchemaAttempts(n int) int {
	if n <= 0 {
		return DefaultSchemaValidationAttempts
	}
	return n
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
