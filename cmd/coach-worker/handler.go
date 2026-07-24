package main

import (
	"context"
	"fmt"
	"log"
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
// repo_baseline_scan (Task 8), including optional smoke fixture and GitHub tree.
func buildJobHandler(cfg Config) (worker.JobHandler, error) {
	baselineCfg := coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: cfg.SmokeFixturePath,
		SmokeRepoOwner:   cfg.SmokeRepoOwner,
		SmokeRepoName:    cfg.SmokeRepoName,
		MaxFiles:         cfg.BaselineMaxFiles,
		MaxTotalBytes:    cfg.BaselineMaxTotalBytes,
		Gateway:          buildModelGateway(),
	}

	if cfg.GitHubAppID > 0 && cfg.GitHubInstallationID > 0 && len(cfg.GitHubPrivateKey) > 0 {
		reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
			AppID:          cfg.GitHubAppID,
			InstallationID: cfg.GitHubInstallationID,
			PrivateKey:     cfg.GitHubPrivateKey,
			BaseURL:        cfg.GitHubBaseURL,
		})
		if err != nil {
			return nil, fmt.Errorf("coach-worker: constructing GitHub file reader: %w", err)
		}
		baselineCfg.TreeSource = &coachapi.GitHubBaselineTreeSource{Reader: reader}
	} else if cfg.SmokeFixturePath == "" {
		log.Printf("coach-worker: warning: no COACH_SMOKE_FIXTURE_PATH and no GitHub App credentials; non-smoke baseline jobs will fail")
	}

	h := coachapi.NewRepoBaselineScanHandler(baselineCfg)
	return func(ctx context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
		return h(ctx, job, w)
	}, nil
}

// buildModelGateway prefers OpenAI-compat when MODEL_GATEWAY_BASE_URL is set;
// otherwise the deterministic stub (core/smoke profile).
func buildModelGateway() modelgateway.Gateway {
	ocfg, err := modelgateway.ConfigFromEnv()
	if err != nil {
		return modelgateway.NewStubGateway()
	}
	client, err := modelgateway.NewOpenAICompatClient(ocfg)
	if err != nil {
		log.Printf("coach-worker: OpenAI-compat gateway unavailable (%v); using stub", err)
		return modelgateway.NewStubGateway()
	}
	return client
}
