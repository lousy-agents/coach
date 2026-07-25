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
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// stubJobHandler is a no-op analysis path for worker lifecycle tests.
// Production main wires buildJobHandler instead.
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

func buildJobHandler(cfg Config) (worker.JobHandler, error) {
	baselineCfg := coachapi.RepoBaselineScanConfig{
		SmokeFixturePath:           cfg.SmokeFixturePath,
		SmokeRepoOwner:             cfg.SmokeRepoOwner,
		SmokeRepoName:              cfg.SmokeRepoName,
		MaxFiles:                   cfg.BaselineMaxFiles,
		MaxTotalBytes:              cfg.BaselineMaxTotalBytes,
		Gateway:                    buildModelGateway(),
		JudgmentMaxWallTime:        cfg.JudgmentMaxWallTime,
		MaxHiddenMutationJudgments: cfg.MaxHiddenMutationJudgments,
		PackConfig: rubrics.PackConfig{
			MaxFindingsPerJudgmentPack:      cfg.MaxFindingsPerJudgmentPack,
			MaxJudgmentPromptTokens:         cfg.MaxJudgmentPromptTokens,
			JudgmentFileAffinityMinFindings: cfg.JudgmentFileAffinityMinFindings,
			EvidenceWindowLines:             cfg.JudgmentEvidenceWindowLines,
		},
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
		// InstallationID is optional thinproof override; zero resolves per repo.
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

// classifyBaselineHandlerError marks transient fetch failures Retryable and
// leaves auth/not-found/too-large/params permanent (FailJob).
func classifyBaselineHandlerError(err error) error {
	if err == nil {
		return nil
	}
	if isPermanentBaselineError(err) {
		return err
	}
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
	// Message match stands in for net.Error without importing net; only after
	// permanent sentinels are ruled out.
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
// otherwise the success stub (core/smoke). If a base URL is set but the client
// cannot be built, return ErrUnavailable so rubrics degrade to diagnostics
// instead of canned source=agent judgments.
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
