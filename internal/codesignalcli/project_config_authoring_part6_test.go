package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_CollectsLayersForbiddenPairsAndRequiredLayerInOrder(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("layers only: no forbidden pairs, no required layer", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_layersOnlyNoForbiddenPairsNoRequiredLayer_12(t, discovered)
	})

	t.Run("layers and forbidden pairs, no required layer", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_layersAndForbiddenPairsNoRequiredLayer_42(t, discovered)
	})

	t.Run("layers, forbidden pairs, and a required layer", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_layersForbiddenPairsAndARequiredLayer_72(t, discovered)
	})

	t.Run("keeps the accepted root and layer after later stages are left blank", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_keepsTheAcceptedRootAndLayerAfterLaterStagesAreL_102(t, discovered)
	})
}
