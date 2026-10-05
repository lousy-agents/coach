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

func TestSeeAllCommandDropsNarrowingFlagsInEverySpelling(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"separate values", []string{"--baseline", "--min-severity", "high", "--top", "2"}, "coach codesignal --baseline"},
		{"inline values", []string{"--baseline", "--min-severity=high", "--top=2"}, "coach codesignal --baseline"},
		{"single dash", []string{"-baseline", "-min-severity", "high", "-top=2"}, "coach codesignal -baseline"},
		{"other flags keep their order", []string{"--top", "1", "--base", "origin/main", "--scope", "all"}, "coach codesignal --base origin/main --scope all"},
		{"a value that looks like a narrowing flag stays with its flag", []string{"--build-target", "--top", "--top", "3"}, "coach codesignal --build-target --top"},
		{"quotes a space", []string{"--project-config", "my config.json", "--top", "1"}, "coach codesignal --project-config 'my config.json'"},
		{"quotes an apostrophe", []string{"--build-target", "it's", "--top", "1"}, `coach codesignal --build-target 'it'\''s'`},
		{"quotes an empty value", []string{"--build-target", "", "--top", "1"}, "coach codesignal --build-target ''"},
		{"quotes a leading equals that zsh would expand to a path", []string{"--base", "=ls", "--top", "1"}, "coach codesignal --base '=ls'"},
		{"quotes a leading tilde", []string{"--base", "~/x", "--top", "1"}, "coach codesignal --base '~/x'"},
		{"leaves an inner equals unquoted", []string{"--base", "a=b", "--top", "1"}, "coach codesignal --base a=b"},
		{"no arguments", nil, "coach codesignal"},
	}
	for _, tc := range cases {
		if got := seeAllCommand(tc.args); got != tc.want {
			t.Errorf("%s: seeAllCommand(%q) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}
