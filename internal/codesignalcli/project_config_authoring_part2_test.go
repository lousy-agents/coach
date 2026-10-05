package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_CoveragePreviewShowsCandidateAndDirectoryCoverage(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{
		Roots:      []string{"apps/api", "apps/web"},
		Candidates: []string{"libs/shared", "libsx/legacy"},
		Complete:   true,
	}

	t.Run("mixed layer matches", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_mixedLayerMatches_17(t, discovered)
	})

	t.Run("no layers declared at all: every discovered directory is shown as uncovered", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_noLayersDeclaredAtAllEveryDiscoveredDirectoryIsS_89(t, discovered)
	})

	t.Run("complete candidate and gate order", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_completeCandidateAndGateOrder_57(t, discovered)
	})

	t.Run("a layer whose prefix is the universal root \".\" matches every discovered directory and leaves nothing uncovered", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_aLayerWhosePrefixIsTheUniversalRootMatchesEveryD_180(t, discovered)
	})
}
