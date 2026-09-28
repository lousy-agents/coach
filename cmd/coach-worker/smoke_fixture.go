package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// smokeMutatingUpdateGo and smokeMutatingResetGo are applied to a temp copy
// of the operator fixture. The committed smoke-repo stays free of pointer
// writes; the scan still has to observe hidden-input mutations so the smoke
// report contains source=deterministic findings.
const smokeMutatingUpdateGo = `package widget

type Config struct {
	Name string
}

func UpdateName(cfg *Config, name string) {
	cfg.Name = name
}
`

const smokeMutatingResetGo = `package widget

func ResetName(cfg *Config) {
	cfg.Name = ""
}
`

func overlaySmokeFixture(src string) (string, error) {
	if src == "" {
		return "", nil
	}
	dst, err := os.MkdirTemp("", "coach-smoke-fixture-")
	if err != nil {
		return "", fmt.Errorf("coach-worker: smoke fixture overlay: %w", err)
	}
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		return "", fmt.Errorf("coach-worker: smoke fixture overlay: %w", err)
	}
	widget := filepath.Join(dst, "widget")
	if err := os.MkdirAll(widget, 0o755); err != nil {
		return "", fmt.Errorf("coach-worker: smoke fixture overlay: %w", err)
	}
	if err := os.WriteFile(filepath.Join(widget, "update_test.go"), []byte(smokeMutatingUpdateGo), 0o644); err != nil {
		return "", fmt.Errorf("coach-worker: smoke fixture overlay: %w", err)
	}
	if err := os.WriteFile(filepath.Join(widget, "reset_test.go"), []byte(smokeMutatingResetGo), 0o644); err != nil {
		return "", fmt.Errorf("coach-worker: smoke fixture overlay: %w", err)
	}
	return dst, nil
}

func buildJobHandler(cfg Config) (worker.JobHandler, error) {
	smokePath, err := overlaySmokeFixture(cfg.SmokeFixturePath)
	if err != nil {
		return nil, err
	}
	baselineCfg := coachapi.RepoBaselineScanConfig{
		SmokeFixturePath:           smokePath,
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
