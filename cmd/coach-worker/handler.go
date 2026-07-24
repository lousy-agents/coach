package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// stubJobHandler is retained for worker lifecycle tests that need a no-op
// analysis path. Production main wires buildJobHandler instead.
func stubJobHandler(_ context.Context, _ coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
	lease := w.Lease()
	now := time.Now().UTC()
	return &coachapi.Completion{
		Attempt:     lease.Attempt,
		CommitSHA:   "",
		Versions:    coachapi.ReportVersions{Analyzer: "stub@0"},
		FinishedAt:  now,
		GeneratedAt: now,
	}, nil
}

// buildJobHandler constructs the production worker.JobHandler for
// repo_baseline_scan (Task 8), including optional smoke fixture and GitHub tree
// via CredentialResolver (ADR-002).
func buildJobHandler(cfg Config) (worker.JobHandler, error) {
	baselineCfg := coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: cfg.SmokeFixturePath,
		SmokeRepoOwner:   cfg.SmokeRepoOwner,
		SmokeRepoName:    cfg.SmokeRepoName,
		MaxFiles:         cfg.BaselineMaxFiles,
		MaxTotalBytes:    cfg.BaselineMaxTotalBytes,
		Gateway:          buildModelGateway(),
	}

	if cfg.GitHubAppID > 0 && len(cfg.GitHubPrivateKey) > 0 {
		resolver, err := githubingest.NewCredentialResolver(githubingest.CredentialResolverConfig{
			AppID:      cfg.GitHubAppID,
			PrivateKey: cfg.GitHubPrivateKey,
			BaseURL:    cfg.GitHubBaseURL,
		})
		if err != nil {
			return nil, fmt.Errorf("coach-worker: constructing GitHub credential resolver: %w", err)
		}
		// Production path resolves installation per repo. Optional
		// InstallationID is a thinproof/backward-compat override only.
		baselineCfg.TreeSource = &coachapi.ResolvingGitHubBaselineTreeSource{
			Credentials:    resolver,
			BaseURL:        cfg.GitHubBaseURL,
			InstallationID: cfg.GitHubInstallationID,
		}
	} else if cfg.SmokeFixturePath == "" {
		log.Printf("coach-worker: warning: no COACH_SMOKE_FIXTURE_PATH and no GitHub App credentials; non-smoke baseline jobs will fail")
	}

	h := coachapi.NewRepoBaselineScanHandler(baselineCfg)
	return func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
		completion, err := h(ctx, job, w)
		return completion, classifyBaselineHandlerError(err)
	}, nil
}

// classifyBaselineHandlerError marks transient GitHub/fetch failures as
// worker.Retryable while keeping sentinel auth/not-found/too-large and bad
// params permanent (FailJob).
func classifyBaselineHandlerError(err error) error {
	if err == nil {
		return nil
	}
	if isPermanentBaselineError(err) {
		return err
	}
	// Transient fetch / dependency failures (timeouts, 5xx, network) retry.
	if strings.Contains(err.Error(), "baseline fetch failed") ||
		isTransientFetchCause(err) {
		return worker.Retryable(err)
	}
	return err
}

func isPermanentBaselineError(err error) bool {
	if errors.Is(err, githubingest.ErrNotFound) ||
		errors.Is(err, githubingest.ErrAuth) ||
		errors.Is(err, githubingest.ErrTooLarge) ||
		errors.Is(err, githubingest.ErrUnsupportedContent) ||
		errors.Is(err, githubingest.ErrEmptyContent) {
		return true
	}
	msg := err.Error()
	// Validation / params / config miswiring are permanent.
	if strings.Contains(msg, "baseline params") ||
		strings.Contains(msg, "not allowed") ||
		strings.Contains(msg, "are required") ||
		strings.Contains(msg, "unsupported job kind") ||
		strings.Contains(msg, "no tree source configured") ||
		strings.Contains(msg, "client-supplied") {
		return true
	}
	return false
}

func isTransientFetchCause(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// net.Error timeout without importing net in every path: message match is
	// intentionally narrow and only used after permanent sentinels are ruled out.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "504") ||
		strings.Contains(msg, "http 5")
}

// buildModelGateway prefers OpenAI-compat when MODEL_GATEWAY_BASE_URL is set;
// otherwise the deterministic stub (core/smoke profile).
//
// When the operator set a base URL but the client cannot be constructed,
// return a gateway that always yields ErrUnavailable so rubrics degrade to
// diagnostics instead of the success stub's canned source=agent judgments.
func buildModelGateway() modelgateway.Gateway {
	ocfg, err := modelgateway.ConfigFromEnv()
	if err != nil {
		return modelgateway.NewStubGateway()
	}
	client, err := modelgateway.NewOpenAICompatClient(ocfg)
	if err != nil {
		log.Printf("coach-worker: OpenAI-compat gateway unavailable (%v); degrading judgments", err)
		return modelgateway.NewStubGateway(modelgateway.StubOptions{
			JudgeErr: modelgateway.NewUnavailableError("openai-compat client construction failed", err),
		})
	}
	return client
}
