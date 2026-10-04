package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_LayerStageNeverInfersPreselectsOrRecommends(t *testing.T) {
	cases := []struct {
		name       string
		discovered projectmodel.TSRootDiscoveryResult
	}{
		{
			name:       "blank answers preselect nothing when discovered roots are present",
			discovered: projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true},
		},
		{
			name:       "blank answers preselect nothing when roots and candidates are present",
			discovered: projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Candidates: []string{"libs/shared"}, Complete: true},
		},
		{
			name:       "blank answers preselect nothing when nothing is discovered",
			discovered: projectmodel.TSRootDiscoveryResult{Complete: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectConfigAuthoringPart5Test_30(t, tc)
		})
	}

	t.Run("a defined layer's prefixes are exactly what the user typed, never augmented with discovered roots", func(t *testing.T) {
		body_projectConfigAuthoringPart5Test_aDefinedLayerSPrefixesAreExactlyWhatTheUserTyped_69(t)
	})

	t.Run("a blank prefix answer is never silently filled in with a discovered root", func(t *testing.T) {
		body_projectConfigAuthoringPart5Test_aBlankPrefixAnswerIsNeverSilentlyFilledInWithADi_86(t)
	})
}
