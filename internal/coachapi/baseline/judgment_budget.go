package baseline

import (
	"context"
	"errors"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// judgmentBudgetExceededError is returned when the judgment-phase wall/tool
// budget stops the pack loop mid-phase. Agent findings from completed packs
// are already persisted; completeAfterJudgmentError records the diagnostic.
type judgmentBudgetExceededError struct {
	Judged    int
	Remaining int
	Err       error
}

func (e *judgmentBudgetExceededError) Error() string {
	return fmt.Sprintf("judgment_budget_exceeded judged=%d remaining=%d: %v", e.Judged, e.Remaining, e.Err)
}

func (e *judgmentBudgetExceededError) Unwrap() error { return e.Err }

// judgmentBudgetDiagnostic builds the stable diagnostic for mid-phase wall/tool budget stop.
func judgmentBudgetDiagnostic(judged, remaining int, err error) coachapi.JobDiagnostic {
	msg := fmt.Sprintf("judgment_budget_exceeded judged=%d remaining=%d", judged, remaining)
	if err != nil {
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	return coachapi.JobDiagnostic{
		ID:      watermill.NewUUID(),
		Scope:   "judgment_budget",
		Message: msg,
	}
}

// completeAfterJudgmentError keeps deterministic findings and any agent findings
// already InsertFindings'd during judgment, then completes the job unless the
// parent context was canceled.
func completeAfterJudgmentError(ctx context.Context, cfg ScanConfig, w JobWriter, commitSHA string, report *codesignal.Report, err error, priorDiags []coachapi.JobDiagnostic) (*coachapi.Completion, error) {
	if errors.Is(err, context.Canceled) {
		return nil, err
	}
	var diagnostics []coachapi.JobDiagnostic
	diagnostics = append(diagnostics, priorDiags...)

	var budgetErr *judgmentBudgetExceededError
	if errors.As(err, &budgetErr) {
		diagnostics = append(diagnostics, judgmentBudgetDiagnostic(budgetErr.Judged, budgetErr.Remaining, budgetErr.Err))
	} else {
		diagnostics = append(diagnostics, coachapi.JobDiagnostic{
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
