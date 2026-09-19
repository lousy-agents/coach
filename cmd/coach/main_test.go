package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

func TestValidateSuggestProjectConfigFlags(t *testing.T) {
	t.Run("rejects an unenumerated flag by name", func(t *testing.T) {
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
	})

	t.Run("allows project-language when its value is typescript", func(t *testing.T) {
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
	})

	t.Run("rejects project-language when its value is go", func(t *testing.T) {
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
	})
}

// TestShouldContinueAfterSetup pins AC-SET-6/AC-7's continuation gate: the
// same scan may resume after a consented compiler-setup action only when
// the action itself succeeded AND its own mandatory readiness rerun reports
// no gap. Every ReadinessStatus value is exercised, so a status this table
// omits cannot silently regress to the wrong side of the gate.
func TestShouldContinueAfterSetup(t *testing.T) {
	allStatuses := []codesignalcli.ReadinessStatus{
		codesignalcli.StatusOutsideSupport,
		codesignalcli.StatusNeedsPrerequisite,
		codesignalcli.StatusNeedsPolicy,
		codesignalcli.StatusReadyWithLimits,
		codesignalcli.StatusReady,
	}

	t.Run("Succeeded false never continues, regardless of PostInstallReadiness", func(t *testing.T) {
		for _, status := range allStatuses {
			status := status
			readiness := codesignalcli.ReadinessResult{Status: status}
			result := codesignalcli.CompilerSetupOfferResult{Succeeded: false, PostInstallReadiness: &readiness}
			if shouldContinueAfterSetup(result) {
				t.Errorf("shouldContinueAfterSetup(Succeeded=false, PostInstallReadiness.Status=%s) = true, want false: an install that did not succeed must never let the scan continue", status)
			}
		}
	})

	t.Run("Succeeded true with a nil PostInstallReadiness never continues", func(t *testing.T) {
		result := codesignalcli.CompilerSetupOfferResult{Succeeded: true, PostInstallReadiness: nil}
		if shouldContinueAfterSetup(result) {
			t.Errorf("shouldContinueAfterSetup(Succeeded=true, PostInstallReadiness=nil) = true, want false: a rerun that never produced a readiness result must never let the scan continue")
		}
	})

	for _, status := range allStatuses {
		status := status
		want := status == codesignalcli.StatusReady || status == codesignalcli.StatusReadyWithLimits
		t.Run("Succeeded true with PostInstallReadiness.Status="+string(status), func(t *testing.T) {
			readiness := codesignalcli.ReadinessResult{Status: status}
			result := codesignalcli.CompilerSetupOfferResult{Succeeded: true, PostInstallReadiness: &readiness}
			if got := shouldContinueAfterSetup(result); got != want {
				t.Errorf("shouldContinueAfterSetup(Succeeded=true, PostInstallReadiness.Status=%s) = %v, want %v", status, got, want)
			}
		})
	}
}

// TestSetupResidueDisclosure pins AC-SET-7's two distinct answers apart.
// ResidueUnknown is not a quieter empty ChangedPaths: SetupOutcome carries
// the working directory itself as the fallback path in that case, which
// renders repository-relative as a bare "." -- a disclosure the customer
// cannot tell from a precise finding, for the one state where Coach in fact
// knows nothing about what the failed command left behind.
func TestSetupResidueDisclosure(t *testing.T) {
	t.Run("residue unknown names the unreadable directory without calling it a precise finding", func(t *testing.T) {
		unknown := setupResidueDisclosure(codesignalcli.CompilerSetupOfferResult{ResidueUnknown: true, ChangedPaths: []string{"packages/app"}})
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
		atRoot := setupResidueDisclosure(codesignalcli.CompilerSetupOfferResult{ResidueUnknown: true, ChangedPaths: []string{"."}})
		if !strings.Contains(atRoot, "could not determine") {
			t.Fatalf("setupResidueDisclosure(residue unknown at the repository root) = %q, want a line saying Coach could not determine what changed", atRoot)
		}
	})

	t.Run("known paths list them as may have changed", func(t *testing.T) {
		known := setupResidueDisclosure(codesignalcli.CompilerSetupOfferResult{ChangedPaths: []string{"mise.toml", "package-lock.json"}})
		if known != "coach codesignal: the setup command may have changed: mise.toml, package-lock.json" {
			t.Fatalf("setupResidueDisclosure(known paths) = %q", known)
		}
	})

	t.Run("nothing to disclose is empty", func(t *testing.T) {
		if got := setupResidueDisclosure(codesignalcli.CompilerSetupOfferResult{}); got != "" {
			t.Fatalf("setupResidueDisclosure(nothing to disclose) = %q, want empty", got)
		}
	})
}

// TestWithheldSetupChoicesLine pins the disclosure that breaks the
// --check-project loop when nothing at all could be offered, and its silence
// on the runtime-boundary path where no menu was ever built.
func TestWithheldSetupChoicesLine(t *testing.T) {
	t.Run("lists every withheld kind and reason when nothing is executable", func(t *testing.T) {
		line := withheldSetupChoicesLine([]codesignalcli.WithheldSetupChoice{
			{Kind: codesignalcli.SetupChoiceProjectPackage, Reason: "manifest_declaration"},
			{Kind: codesignalcli.SetupChoiceProjectMise, Reason: "mise_unconfigured"},
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

func TestShouldRenderAfterOptionalPreparation(t *testing.T) {
	cases := []struct {
		name   string
		result optionalPreparationResult
		want   bool
	}{
		{name: "zero value (nothing offered) still renders", result: optionalPreparationResult{}, want: true},
		{name: "declining an optional action still renders", result: optionalPreparationResult{Declined: true}, want: true},
		{name: "a succeeded optional action still renders", result: optionalPreparationResult{Succeeded: true}, want: true},
		{name: "cancelling an optional action does not render", result: optionalPreparationResult{Cancelled: true}, want: false},
		{name: "a failed optional action does not render", result: optionalPreparationResult{Failed: true}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRenderAfterOptionalPreparation(tc.result); got != tc.want {
				t.Fatalf("shouldRenderAfterOptionalPreparation(%+v) = %v, want %v", tc.result, got, tc.want)
			}
		})
	}
}

func TestScanShouldOfferCompilerSetupIgnoresRuntimeUnresolvedError(t *testing.T) {
	err := &codesignalcli.RuntimeUnresolvedError{Code: codesignalcli.GapNodeMissing, ConfigPath: "project.json"}
	if wrapped, ok := scanShouldOfferCompilerSetup(err, false); ok {
		t.Fatalf("scanShouldOfferCompilerSetup(RuntimeUnresolvedError) = (%v, true), want false: a runtime gap must not enter the compiler-setup offer", wrapped)
	}
}

func TestAnalysisErrorReportForRuntimeUnresolvedError(t *testing.T) {
	err := &codesignalcli.RuntimeUnresolvedError{Code: codesignalcli.GapNodeMissing, ConfigPath: "project.json"}
	got := analysisErrorReportFor(err, "typescript", false)
	if got.exitCode != 2 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).exitCode = %d, want 2", got.exitCode)
	}
	if len(got.lines) != 1 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines = %q, want exactly the gap line", got.lines)
	}
	if got.lines[0] != err.RemediationLine() {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines[0] = %q, want %q", got.lines[0], err.RemediationLine())
	}
}
