package main

import (
	"testing"
)

// TestSetupResidueDisclosure pins AC-SET-7's two distinct answers apart.
// ResidueUnknown is not a quieter empty ChangedPaths: SetupOutcome carries
// the working directory itself as the fallback path in that case, which
// renders repository-relative as a bare "." -- a disclosure the customer
// cannot tell from a precise finding, for the one state where Coach in fact
// knows nothing about what the failed command left behind.
func TestSetupResidueDisclosure(t *testing.T) {
	t.Run("residue unknown names the unreadable directory without calling it a precise finding", func(t *testing.T) {
		body_mainTest_residueUnknownNamesTheUnreadableDirectoryWithout_17(t)
	})

	t.Run("working-directory fallback at the repository root still says Coach could not determine", func(t *testing.T) {
		body_mainTest_workingDirectoryFallbackAtTheRepositoryRootStill_30(t)
	})

	t.Run("known paths list them as may have changed", func(t *testing.T) {
		body_mainTest_knownPathsListThemAsMayHaveChanged_37(t)
	})

	t.Run("nothing to disclose is empty", func(t *testing.T) {
		body_mainTest_nothingToDiscloseIsEmpty_44(t)
	})
}

func TestValidateSuggestProjectConfigFlags(t *testing.T) {
	t.Run("rejects an unenumerated flag by name", func(t *testing.T) {
		body_mainTest_rejectsAnUnenumeratedFlagByName_52(t)
	})

	t.Run("allows project-language when its value is typescript", func(t *testing.T) {
		body_mainTest_allowsProjectLanguageWhenItsValueIsTypescript_70(t)
	})

	t.Run("rejects project-language when its value is go", func(t *testing.T) {
		body_mainTest_rejectsProjectLanguageWhenItsValueIsGo_85(t)
	})
}
