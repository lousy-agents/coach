package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_InvalidAnswersExplainAndAllowRetryOrCancel(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}

	t.Run("duplicate layer name explains the error and cancel stops the session there", func(t *testing.T) {
		body_projectConfigAuthoringPart8Test_duplicateLayerNameExplainsTheErrorAndCancelStops_13(t, discovered)
	})

	t.Run("overlapping prefix explains the error and retry accepts a corrected answer", func(t *testing.T) {
		body_projectConfigAuthoringPart8Test_overlappingPrefixExplainsTheErrorAndRetryAccepts_36(t, discovered)
	})

	t.Run("forbidden pair referencing an undeclared layer explains the error and allows cancel", func(t *testing.T) {
		body_projectConfigAuthoringPart8Test_forbiddenPairReferencingAnUndeclaredLayerExplain_63(t, discovered)
	})

	t.Run("required layer naming an undeclared layer explains the error and retry accepts a declared layer", func(t *testing.T) {
		body_projectConfigAuthoringPart8Test_requiredLayerNamingAnUndeclaredLayerExplainsTheE_83(t, discovered)
	})
}
