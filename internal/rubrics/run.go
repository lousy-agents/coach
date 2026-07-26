package rubrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

// Run executes one rubric judgment via the gateway.
//
// On schema validation failure or gateway unavailability/timeout it returns a
// Diagnostic and no Judgment (and a nil error) so the job can complete with
// deterministic-only findings (Story 5).
//
// context.Canceled — including Unavailable errors caused by cancel — is a
// lifecycle abort, not Story 5 degrade: Run returns that error with an empty
// Result so callers do not CompleteJob as deterministic-only success.
func Run(ctx context.Context, gw modelgateway.Gateway, def Definition, messages []modelgateway.Message) (Result, error) {
	if err := lifecycleAbortErr(ctx.Err()); err != nil {
		return Result{}, err
	}
	// Validate definition before gw so a missing id never yields scope "rubric:".
	if def.ID == "" {
		return degrade("unknown", "rubric definition id is required"), nil
	}
	if gw == nil {
		return degrade(def.ID, "model gateway is nil"), nil
	}

	resp, err := gw.Judge(ctx, modelgateway.JudgmentRequest{
		RubricID:      def.ID,
		RubricVersion: def.Version,
		Messages:      messages,
		OutputSchema:  def.OutputSchema,
	})
	if abort := firstLifecycleAbort(err, ctx.Err()); abort != nil {
		return Result{}, abort
	}
	// Wall-budget deadline on the tool op context: propagate so agentloop.mapWallErr
	// rewrites to ErrBudgetExceeded (Story 2). Gateway timeouts while ctx is still
	// live remain Story 5 soft-degrade via degradeFromErr below.
	if err != nil && isOpDeadlineExceeded(ctx) {
		return Result{}, err
	}
	if err != nil {
		return degradeFromErr(def.ID, err), nil
	}

	identity := FormatModelIdentity(resp.LogicalModelID, resp.ServedModelID)
	return Result{
		Judgment: &Judgment{
			RubricID:       def.ID,
			RubricVersion:  def.Version,
			ModelIdentity:  identity,
			LogicalModelID: resp.LogicalModelID,
			ServedModelID:  resp.ServedModelID,
			JudgmentJSON:   append(json.RawMessage(nil), resp.JudgmentJSON...),
		},
	}, nil
}

// lifecycleAbortErr returns a non-nil error when err represents owning-context
// cancellation. DeadlineExceeded is not a cancel abort: when the op context is
// still live it soft-degrades (Story 5); when the op context itself timed out
// (judgment wall), callers propagate via isOpDeadlineExceeded for mapWallErr.
func lifecycleAbortErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return nil
}

func firstLifecycleAbort(errs ...error) error {
	for _, err := range errs {
		if abort := lifecycleAbortErr(err); abort != nil {
			return abort
		}
	}
	return nil
}

// isOpDeadlineExceeded reports whether ctx was canceled by deadline (wall budget
// child). Distinct from a gateway-injected DeadlineExceeded cause while ctx lives.
func isOpDeadlineExceeded(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func degradeFromErr(rubricID string, err error) Result {
	switch {
	case errors.Is(err, modelgateway.ErrSchemaValidation):
		return degrade(rubricID, fmt.Sprintf("schema validation failed: %v", err))
	case errors.Is(err, modelgateway.ErrUnavailable):
		return degrade(rubricID, fmt.Sprintf("model gateway unavailable: %v", err))
	default:
		return degrade(rubricID, fmt.Sprintf("judgment failed: %v", err))
	}
}

func degrade(rubricID, message string) Result {
	return Result{
		Diagnostic: &Diagnostic{
			Scope:   diagnosticScope(rubricID),
			Message: message,
		},
	}
}
