package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_DeclinedApprovalNeverReachesTheWritePath(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("with --output set, no file is created", func(t *testing.T) {
		body_projectConfigAuthoringPart9Test_withOutputSetNoFileIsCreated_18(t, discovered)
	})

	t.Run("with --output unset, nothing beyond the interactive text is written to out", func(t *testing.T) {
		body_projectConfigAuthoringPart9Test_withOutputUnsetNothingBeyondTheInteractiveTextIs_50(t, discovered)
	})
}

func TestAuthorProjectConfig_RootSelectionNeverProposesLayerBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		discovered projectmodel.TSRootDiscoveryResult
		answer     string
	}{
		{
			name: "with only roots listed, prompt text never contains layer or boundary",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      []string{"services/checkout", "services/billing", "libs/shared"},
				Candidates: nil,
				Complete:   true,
			},
			answer: "1,2,3\n",
		},
		{
			name: "with roots and candidates listed, prompt text never contains layer or boundary",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      []string{"services/checkout", "services/billing"},
				Candidates: []string{"libs/shared"},
				Complete:   true,
			},
			answer: "1,2\n",
		},
		{
			name: "with nothing discovered, prompt text never contains layer or boundary",
			discovered: projectmodel.TSRootDiscoveryResult{
				Roots:      nil,
				Candidates: nil,
				Complete:   true,
			},
			answer: "services/checkout\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectConfigAuthoringPart9Test_108(t, tc)
		})
	}
}
