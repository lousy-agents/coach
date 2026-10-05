package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

func body_mainTest_residueUnknownNamesTheUnreadableDirectoryWithout_17(t *testing.T) {
	unknown := setupResidueDisclosure(tssetup.CompilerSetupOfferResult{ResidueUnknown: true, ChangedPaths: []string{"packages/app"}})
	if !strings.Contains(unknown, "packages/app") {
		t.Fatalf("setupResidueDisclosure(residue unknown) = %q, want it to name the directory Coach could not read -- the only location it has", unknown)
	}
	if !strings.Contains(unknown, "could not determine") {
		t.Fatalf("setupResidueDisclosure(residue unknown) = %q, want a line saying Coach could not determine what changed", unknown)
	}
	if strings.Contains(unknown, "may have changed: packages/app") {
		t.Fatalf("setupResidueDisclosure(residue unknown) = %q, must not render the working-directory fallback as a precise finding", unknown)
	}
}

func body_mainTest_workingDirectoryFallbackAtTheRepositoryRootStill_30(t *testing.T) {
	atRoot := setupResidueDisclosure(tssetup.CompilerSetupOfferResult{ResidueUnknown: true, ChangedPaths: []string{"."}})
	if !strings.Contains(atRoot, "could not determine") {
		t.Fatalf("setupResidueDisclosure(residue unknown at the repository root) = %q, want a line saying Coach could not determine what changed", atRoot)
	}
}

func body_mainTest_knownPathsListThemAsMayHaveChanged_37(t *testing.T) {
	known := setupResidueDisclosure(tssetup.CompilerSetupOfferResult{ChangedPaths: []string{"mise.toml", "package-lock.json"}})
	if known != "coach codesignal: the setup command may have changed: mise.toml, package-lock.json" {
		t.Fatalf("setupResidueDisclosure(known paths) = %q", known)
	}
}

func body_mainTest_nothingToDiscloseIsEmpty_44(t *testing.T) {
	if got := setupResidueDisclosure(tssetup.CompilerSetupOfferResult{}); got != "" {
		t.Fatalf("setupResidueDisclosure(nothing to disclose) = %q, want empty", got)
	}
}

func body_mainTest_rejectsAnUnenumeratedFlagByName_52(t *testing.T) {
	f := codesignalFlags{baseline: true, suggestProjectConfig: true}
	setFlags := map[string]bool{
		"suggest-project-config": true,
		"baseline":               true,
		"totally-new-flag":       true,
	}

	got := validateSuggestProjectConfigFlags(f, setFlags, nil, 1, 0)

	if got == "" {
		t.Fatalf("validateSuggestProjectConfigFlags returned no error for an unenumerated flag; expected rejection")
	}
	if !strings.Contains(got, "totally-new-flag") {
		t.Errorf("validateSuggestProjectConfigFlags = %q, want a message naming the unenumerated flag", got)
	}
}

func body_mainTest_allowsProjectLanguageWhenItsValueIsTypescript_70(t *testing.T) {
	f := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
	setFlags := map[string]bool{
		"suggest-project-config": true,
		"baseline":               true,
		"project-language":       true,
	}

	got := validateSuggestProjectConfigFlags(f, setFlags, nil, 1, 0)

	if got != "" {
		t.Fatalf("validateSuggestProjectConfigFlags = %q, want no error for --project-language typescript", got)
	}
}

func body_mainTest_rejectsProjectLanguageWhenItsValueIsGo_85(t *testing.T) {
	f := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "go"}
	setFlags := map[string]bool{
		"suggest-project-config": true,
		"baseline":               true,
		"project-language":       true,
	}

	got := validateSuggestProjectConfigFlags(f, setFlags, nil, 1, 0)

	if got == "" {
		t.Fatalf("validateSuggestProjectConfigFlags returned no error for --project-language go; expected rejection")
	}
	if !strings.Contains(got, "project-language") {
		t.Errorf("validateSuggestProjectConfigFlags = %q, want a message naming project-language", got)
	}
}
