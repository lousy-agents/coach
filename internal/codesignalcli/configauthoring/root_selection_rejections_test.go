package configauthoring

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("an empty selection is rejected and cancel stops the session before any later stage runs", func(t *testing.T) {
		anEmptySelectionRejectedCancelStopsSessionBefore(t, discovered)
	})

	t.Run("an empty selection is rejected and retry accepts a corrected answer", func(t *testing.T) {
		anEmptySelectionRejectedRetryAcceptsCorrectedAnswer(t, discovered)
	})

	t.Run("an absolute path is rejected and cancel stops the session", func(t *testing.T) {
		anAbsolutePathRejectedCancelStopsSession(t, discovered)
	})

	t.Run("a path escaping the repository is rejected and retry accepts a corrected answer", func(t *testing.T) {
		aPathEscapingRepositoryRejectedRetryAcceptsCorrected(t, discovered)
	})

	t.Run("a non-normalized path is rejected and cancel stops the session", func(t *testing.T) {
		aNonNormalizedPathRejectedCancelStopsSession(t, discovered)
	})

	t.Run("an out-of-range root number mixed with a valid one is rejected, not silently dropped, and cancel stops the session", func(t *testing.T) {
		anOutRangeRootNumberMixedValidOne(t, discovered)
	})

	t.Run("a root selection over the budget is rejected and cancel stops the session before any later stage runs", func(t *testing.T) {
		aRootSelectionOverBudgetRejectedCancelStops(t, discovered)
	})
}

func aNonNormalizedPathRejectedCancelStopsSession(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"src/",
		"cancel",
	)
	if !result.Cancelled {
		t.Fatalf("expected a non-normalized root path to be rejected and cancelled, got Cancelled = false")
	}
	if !strings.Contains(out, "src/") {
		t.Fatalf("expected the error explanation to reference the offending path, got:\n%s", out)
	}
}

func anOutRangeRootNumberMixedValidOne(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"1,3",
		"cancel",
	)
	if !result.Cancelled {
		t.Fatalf("expected a selection containing an out-of-range root number to be rejected and cancelled, got Cancelled = false, result = %+v", result)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("expected no roots to be recorded for a cancelled selection, got %v -- an out-of-range index must never be silently dropped while keeping the rest of the answer", result.Roots)
	}
	if !strings.Contains(out, `"3"`) {
		t.Fatalf("expected the rejection explanation to reference the offending token %q, got:\n%s", "3", out)
	}
}

func aRootSelectionOverBudgetRejectedCancelStops(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	roots := make([]string, projectconfig.MaxRoots+1)
	for i := range roots {
		roots[i] = fmt.Sprintf("dir%d", i)
	}

	result, out := runAuthoringWithTimeout(t, 3*time.Second, discovered,
		strings.Join(roots, ","),
		"cancel",
	)
	if !result.Cancelled {
		t.Fatalf("expected an over-budget root selection to be rejected and cancelled, got Cancelled = false, result = %+v", result)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("expected no roots to be recorded for a cancelled over-budget selection, got %v", result.Roots)
	}
	if !strings.Contains(out, fmt.Sprintf("%d", projectconfig.MaxRoots)) {
		t.Fatalf("expected the rejection explanation to reference the %d-entry budget, got:\n%s", projectconfig.MaxRoots, out)
	}
	if strings.Contains(strings.ToLower(out), "layer") {
		t.Fatalf("expected authoring to stop at cancellation, not continue to the layer stage, got:\n%s", out)
	}
}
