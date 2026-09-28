package coachapi

import "context"

func judgePreparedBaseline(ctx context.Context, cfg RepoBaselineScanConfig, w BaselineJobWriter, prepared preparedRepoBaseline) (*Completion, error) {
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
