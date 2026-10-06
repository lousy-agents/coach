package baseline

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func judgePreparedBaseline(ctx context.Context, cfg ScanConfig, w JobWriter, prepared preparedRepoBaseline) (*coachapi.Completion, error) {
	judgmentLoop, err := newJudgmentLoop(cfg, countHiddenMutationFindings(prepared.findings))
	if err != nil {
		return nil, err
	}

	// Agent findings from hidden-mutation packs are inserted incrementally
	// inside judgeBaselineViaLoop; cohesion findings are inserted there too.
	// Do not re-insert agent findings here.
	_, agentDiags, err := judgeBaselineViaLoop(ctx, judgmentLoop, prepared.loaded, prepared.findings, w, cfg.PackConfig, cfg.MaxHiddenMutationJudgments)
	if cfg.ObserveLoop != nil {
		cfg.ObserveLoop(judgmentLoop)
	}
	if err != nil {
		return completeAfterJudgmentError(ctx, cfg, w, prepared.commitSHA, prepared.report, err, agentDiags)
	}

	diagnostics := append(agentDiags, diagnosticsFromCodeSignal(prepared.report)...)
	if err := insertBaselineDiagnostics(ctx, w, diagnostics); err != nil {
		return nil, err
	}
	return baselineCompletion(cfg, w, prepared.commitSHA), nil
}

func judgeBaselineViaLoop(
	ctx context.Context,
	loop *agentloop.Loop,
	files []loadedBaselineFile,
	detFindings []coachapi.JobFinding,
	w JobWriter,
	packCfg rubrics.PackConfig,
	maxHiddenMutationJudgments int,
) ([]coachapi.JobFinding, []coachapi.JobDiagnostic, error) {
	byPath := make(map[string]loadedBaselineFile, len(files))
	fileMetas := make([]rubrics.FileMeta, 0, len(files))
	for _, f := range files {
		byPath[f.Path] = f
		fileMetas = append(fileMetas, rubrics.FileMeta{
			Path:     f.Path,
			Language: string(f.Language),
		})
	}

	packCfg = rubrics.ApplyPackConfigDefaults(packCfg)
	maxJudgments := resolveMaxHiddenMutationJudgments(maxHiddenMutationJudgments)

	agentFindings, diagnostics, err := judgeHiddenMutationFindings(ctx, loop, byPath, detFindings, w, packCfg, maxJudgments)
	if err != nil {
		return agentFindings, diagnostics, err
	}

	cohesionFindings, cohesionDiags, err := judgeChangeCohesion(ctx, loop, fileMetas, detFindings)
	if err != nil {
		if errors.Is(err, agentloop.ErrBudgetExceeded) {
			// HM packs finished; wall died on cohesion. Report HM agent rows as judged.
			return agentFindings, diagnostics, &judgmentBudgetExceededError{
				Judged:    len(agentFindings),
				Remaining: 0,
				Err:       err,
			}
		}
		return agentFindings, diagnostics, err
	}
	if err := insertBaselineFindings(ctx, w, cohesionFindings); err != nil {
		return agentFindings, diagnostics, err
	}
	agentFindings = append(agentFindings, cohesionFindings...)
	diagnostics = append(diagnostics, cohesionDiags...)
	return agentFindings, diagnostics, nil
}
