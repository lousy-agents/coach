package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_SuggestsDiscoveredRootsWithoutPreselecting(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{
		Roots:      []string{"apps/api", "apps/web"},
		Candidates: []string{"libs/shared"},
		Complete:   true,
	}

	t.Run("prints discovered roots as a suggestion before reading any answer", func(t *testing.T) {
		body_projectConfigAuthoringPart7Test_printsDiscoveredRootsAsASuggestionBeforeReadingA_19(t, discovered)
	})

	t.Run("selects the roots the user names by number", func(t *testing.T) {
		body_projectConfigAuthoringPart7Test_selectsTheRootsTheUserNamesByNumber_52(t, discovered)
	})

	t.Run("selects only the single root the user names by number, not every discovered root", func(t *testing.T) {
		body_projectConfigAuthoringPart7Test_selectsOnlyTheSingleRootTheUserNamesByNumberNotE_64(t, discovered)
	})

	t.Run("selects a discovered root by number together with a literal path, in the order named", func(t *testing.T) {
		body_projectConfigAuthoringPart7Test_selectsADiscoveredRootByNumberTogetherWithALiter_76(t, discovered)
	})

	t.Run("cancels when the user never answers the prompt", func(t *testing.T) {
		body_projectConfigAuthoringPart7Test_cancelsWhenTheUserNeverAnswersThePrompt_88(t, discovered)
	})

	t.Run("selects a literal path the user types instead of a discovered root", func(t *testing.T) {
		body_projectConfigAuthoringPart7Test_selectsALiteralPathTheUserTypesInsteadOfADiscove_102(t, discovered)
	})
}
