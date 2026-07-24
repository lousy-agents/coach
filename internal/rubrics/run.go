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
// cancellation. DeadlineExceeded is not an abort — it is judgment-phase timeout
// and remains eligible for Story 5 soft degrade.
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
