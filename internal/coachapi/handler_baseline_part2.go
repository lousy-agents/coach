package coachapi

import (
	"context"

	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"

	"path/filepath"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// newJudgmentLoop builds the rubric-tool loop for hiddenCount
// hidden-mutation findings, with its own wall budget so analyze wall time
// never consumes it. MaxToolCalls scales for packs + cohesion + slack rather
// than 1:1 with findings: the ceiling is one Call per finding (worst pack
// size 1) plus cohesion plus slack.
func newJudgmentLoop(cfg RepoBaselineScanConfig, hiddenCount int) (*agentloop.Loop, error) {
	gw := cfg.Gateway
	if gw == nil {
		gw = modelgateway.NewStubGateway()
	}

	judgmentWall := cfg.JudgmentMaxWallTime
	if judgmentWall <= 0 {
		judgmentWall = DefaultJudgmentMaxWallTime
	}
	judgmentMaxTools := hiddenCount + 1 + 10
	if judgmentMaxTools < agentloop.DefaultMaxToolCalls {
		judgmentMaxTools = agentloop.DefaultMaxToolCalls
	}

	judgmentLoop, err := agentloop.New(agentloop.Options{
		Budget: agentloop.Budget{
			MaxToolCalls: judgmentMaxTools,
			MaxWallTime:  judgmentWall,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("coachapi: constructing judgment agent loop: %w", err)
	}
	if err := rubrics.RegisterTools(judgmentLoop, gw); err != nil {
		return nil, fmt.Errorf("coachapi: registering rubric tools: %w", err)
	}
	if cfg.ConfigureLoop != nil {
		cfg.ConfigureLoop(judgmentLoop)
	}
	return judgmentLoop, nil
}
func loadBaselineFiles(ctx context.Context, source BaselineTreeSource, params RepoBaselineScanParams, ref string, entries []BaselineFileEntry) ([]loadedBaselineFile, error) {
	out := make([]loadedBaselineFile, 0, len(entries))
	for _, e := range entries {
		lang, ok := semantics.LanguageForExtension(filepath.Ext(e.Path))
		if !ok {
			continue
		}
		content, _, err := source.ReadFile(ctx, params.RepoOwner, params.RepoName, ref, e.Path)
		if err != nil {
			return nil, mapBaselineFetchError(err)
		}
		out = append(out, loadedBaselineFile{
			Path:     e.Path,
			Language: lang,
			Content:  string(content),
		})
	}
	return out, nil
}
func insertBaselineDiagnostics(ctx context.Context, w BaselineJobWriter, diagnostics []JobDiagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	return w.InsertDiagnostics(ctx, diagnostics)
}
