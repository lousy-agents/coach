package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("an empty selection is rejected and cancel stops the session before any later stage runs", func(t *testing.T) {
		body_projectConfigAuthoringPart4Test_anEmptySelectionIsRejectedAndCancelStopsTheSessi_16(t, discovered)
	})

	t.Run("an empty selection is rejected and retry accepts a corrected answer", func(t *testing.T) {
		body_projectConfigAuthoringPart4Test_anEmptySelectionIsRejectedAndRetryAcceptsACorrec_35(t, discovered)
	})

	t.Run("an absolute path is rejected and cancel stops the session", func(t *testing.T) {
		body_projectConfigAuthoringPart4Test_anAbsolutePathIsRejectedAndCancelStopsTheSession_52(t, discovered)
	})

	t.Run("a path escaping the repository is rejected and retry accepts a corrected answer", func(t *testing.T) {
		body_projectConfigAuthoringPart4Test_aPathEscapingTheRepositoryIsRejectedAndRetryAcce_65(t, discovered)
	})

	t.Run("a non-normalized path is rejected and cancel stops the session", func(t *testing.T) {
		body_projectConfigAuthoringPart4Test_aNonNormalizedPathIsRejectedAndCancelStopsTheSes_82(t, discovered)
	})

	t.Run("an out-of-range root number mixed with a valid one is rejected, not silently dropped, and cancel stops the session", func(t *testing.T) {
		body_projectConfigAuthoringPart4Test_anOutOfRangeRootNumberMixedWithAValidOneIsReject_95(t, discovered)
	})

	t.Run("a root selection over the budget is rejected and cancel stops the session before any later stage runs", func(t *testing.T) {
		body_projectConfigAuthoringPart4Test_aRootSelectionOverTheBudgetIsRejectedAndCancelSt_111(t, discovered)
	})
}
