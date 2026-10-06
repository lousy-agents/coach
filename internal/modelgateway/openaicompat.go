package modelgateway

import (
	"context"
	"net/http"
)

// OpenAICompatClient implements Gateway via POST {baseURL}/v1/chat/completions.
type OpenAICompatClient struct {
	baseURL                  string
	logicalModel             string
	apiKey                   string
	authHeader               string
	extraHeaders             map[string]string
	httpClient               *http.Client
	schemaValidationAttempts int
	disableThinking          bool
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
		disableThinking:          cfg.DisableThinking,
	}, nil
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
