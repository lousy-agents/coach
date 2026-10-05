package coachapi

import (
	"context"

	"errors"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// completeAfterJudgmentError keeps deterministic findings and any agent findings
// already InsertFindings'd during judgment, then completes the job unless the
// parent context was canceled.
func completeAfterJudgmentError(ctx context.Context, cfg RepoBaselineScanConfig, w BaselineJobWriter, commitSHA string, report *codesignal.Report, err error, priorDiags []JobDiagnostic) (*Completion, error) {
	if errors.Is(err, context.Canceled) {
		return nil, err
	}
	var diagnostics []JobDiagnostic
	diagnostics = append(diagnostics, priorDiags...)

	var budgetErr *judgmentBudgetExceededError
	if errors.As(err, &budgetErr) {
		diagnostics = append(diagnostics, judgmentBudgetDiagnostic(budgetErr.Judged, budgetErr.Remaining, budgetErr.Err))
	} else {
		diagnostics = append(diagnostics, JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   "judgment",
			Message: fmt.Sprintf("judgment phase failed: %v", err),
		})
	}
	diagnostics = append(diagnostics, diagnosticsFromCodeSignal(report)...)
	if err := insertBaselineDiagnostics(ctx, w, diagnostics); err != nil {
		return nil, err
	}
	return baselineCompletion(cfg, w, commitSHA), nil
}
