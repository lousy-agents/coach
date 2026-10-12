package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

// TestWithheldSetupChoicesLine pins the disclosure that breaks the
// --check-project loop when nothing at all could be offered, and its silence
// on the runtime-boundary path where no menu was ever built.
func TestWithheldSetupChoicesLine(t *testing.T) {
	t.Run("lists every withheld kind and reason when nothing is executable", func(t *testing.T) {
		line := withheldSetupChoicesLine([]tssetup.WithheldChoice{
			{Kind: tssetup.ChoiceProjectPackage, Reason: "manifest_declaration"},
			{Kind: tssetup.ChoiceProjectMise, Reason: "mise_unconfigured"},
		})
		want := "coach codesignal: no compiler-setup choice is executable here: project_package (manifest_declaration), project_mise (mise_unconfigured)."
		if line != want {
			t.Fatalf("withheldSetupChoicesLine() = %q, want %q", line, want)
		}
	})
	t.Run("is silent when no menu was built", func(t *testing.T) {
		if got := withheldSetupChoicesLine(nil); got != "" {
			t.Fatalf("withheldSetupChoicesLine(nil) = %q, want empty: no menu was built, so there is nothing to explain", got)
		}
	})
}

// TestSetupResidueDisclosure pins AC-SET-7's two distinct answers apart.
// ResidueUnknown is not a quieter empty ChangedPaths: SetupOutcome carries
// the working directory itself as the fallback path in that case, which
// renders repository-relative as a bare "." -- a disclosure the customer
// cannot tell from a precise finding, for the one state where Coach in fact
// knows nothing about what the failed command left behind.
func TestSetupResidueDisclosure(t *testing.T) {
	t.Run("residue unknown names the unreadable directory without calling it a precise finding", func(t *testing.T) {
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
	})

	t.Run("working-directory fallback at the repository root still says Coach could not determine", func(t *testing.T) {
		atRoot := setupResidueDisclosure(tssetup.CompilerSetupOfferResult{ResidueUnknown: true, ChangedPaths: []string{"."}})
		if !strings.Contains(atRoot, "could not determine") {
			t.Fatalf("setupResidueDisclosure(residue unknown at the repository root) = %q, want a line saying Coach could not determine what changed", atRoot)
		}
	})

	t.Run("known paths list them as may have changed", func(t *testing.T) {
		known := setupResidueDisclosure(tssetup.CompilerSetupOfferResult{ChangedPaths: []string{"mise.toml", "package-lock.json"}})
		if known != "coach codesignal: the setup command may have changed: mise.toml, package-lock.json" {
			t.Fatalf("setupResidueDisclosure(known paths) = %q", known)
		}
	})

	t.Run("nothing to disclose is empty", func(t *testing.T) {
		if got := setupResidueDisclosure(tssetup.CompilerSetupOfferResult{}); got != "" {
			t.Fatalf("setupResidueDisclosure(nothing to disclose) = %q, want empty", got)
		}
	})
}
