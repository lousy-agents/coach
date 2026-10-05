package coachapi

import (
	"context"

	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"

	"time"
)

// Analyze wall time never consumes the judgment budget: judgment uses a
// fresh loop of its own.
func newAnalyzeLoop(cfg RepoBaselineScanConfig, fileCount int) (*agentloop.Loop, error) {
	analyzeMaxTools := fileCount + 1 + 10
	if analyzeMaxTools < agentloop.DefaultMaxToolCalls {
		analyzeMaxTools = agentloop.DefaultMaxToolCalls
	}
	analyzeLoop, err := agentloop.New(agentloop.Options{
		Budget: agentloop.Budget{MaxToolCalls: analyzeMaxTools},
	})
	if err != nil {
		return nil, fmt.Errorf("coachapi: constructing analyze agent loop: %w", err)
	}
	if cfg.ConfigureLoop != nil {
		cfg.ConfigureLoop(analyzeLoop)
	}
	return analyzeLoop, nil
}
func resolveBaselineTreeSource(cfg RepoBaselineScanConfig, params RepoBaselineScanParams) (BaselineTreeSource, error) {
	if cfg.SmokeFixturePath != "" &&
		cfg.SmokeRepoOwner != "" &&
		cfg.SmokeRepoName != "" &&
		params.RepoOwner == cfg.SmokeRepoOwner &&
		params.RepoName == cfg.SmokeRepoName {
		return &LocalFixtureTreeSource{Root: cfg.SmokeFixturePath}, nil
	}
	if cfg.TreeSource == nil {
		return nil, fmt.Errorf("coachapi: no tree source configured for %s/%s (not the smoke fixture pair)", params.RepoOwner, params.RepoName)
	}
	return cfg.TreeSource, nil
}
func NewRepoBaselineScanHandler(cfg RepoBaselineScanConfig) BaselineJobHandler {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return func(ctx context.Context, job Job, w BaselineJobWriter) (*Completion, error) {
		return runRepoBaselineScan(ctx, cfg, job, w)
	}
}
func insertBaselineFindings(ctx context.Context, w BaselineJobWriter, findings []JobFinding) error {
	if len(findings) == 0 {
		return nil
	}
	return w.InsertFindings(ctx, findings)
}
