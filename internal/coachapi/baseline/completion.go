package baseline

import (
	"context"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func baselineCompletion(cfg ScanConfig, w JobWriter, commitSHA string) *coachapi.Completion {
	now := cfg.Now().UTC()
	lease := w.Lease()
	return &coachapi.Completion{
		Attempt:   lease.Attempt,
		CommitSHA: commitSHA,
		Versions: coachapi.ReportVersions{
			Analyzer: baselineAnalyzerVersion,
			Rubrics:  seedRubricVersions(),
		},
		FinishedAt:  now,
		GeneratedAt: now,
	}
}

func seedRubricVersions() map[string]string {
	seed := rubrics.Seed()
	out := make(map[string]string, len(seed))
	for _, def := range seed {
		out[def.ID] = def.Version
	}
	return out
}

func insertBaselineFindings(ctx context.Context, w JobWriter, findings []coachapi.JobFinding) error {
	if len(findings) == 0 {
		return nil
	}
	return w.InsertFindings(ctx, findings)
}

func insertBaselineDiagnostics(ctx context.Context, w JobWriter, diagnostics []coachapi.JobDiagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	return w.InsertDiagnostics(ctx, diagnostics)
}
