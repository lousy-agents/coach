package tssetup

import (
	"fmt"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps pins
// AC-SET-10's runtime-boundary installer suppression against every gap code
// projectreadiness.KnownGapCodes reports, via a literal per-code expectation
// rather than an expression derived from production code: deriving "should
// offer a command" from projectreadiness.NextActionExecutable or the gap-code
// table itself (as
// PrepareCompilerRemediation does) would make the assertion tautological and
// unable to catch a mutation to either.
func TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps(t *testing.T) {
	wantCommand := map[string]bool{
		projectreadiness.GapUnsupportedRepositoryShape:        false,
		projectreadiness.GapNodeMissing:                       false,
		projectreadiness.GapNodeUnsupported:                   false,
		projectreadiness.GapNodeUnverifiable:                  false,
		projectreadiness.GapTypescriptCompilerMissing:         true,
		projectreadiness.GapTypescriptVersionMismatch:         true,
		projectreadiness.GapTypescriptVersionConflict:         true,
		projectreadiness.GapPackageManagerAmbiguous:           false,
		projectreadiness.GapPackageManagerConfigUnverifiable:  false,
		projectreadiness.GapPackageManagerVersionUnverifiable: false,
		projectreadiness.GapPackageManagerVersionUnsupported:  false,
		projectreadiness.GapPolicyMissing:                     false,
		projectreadiness.GapPolicyInvalid:                     false,
	}

	if len(wantCommand) != len(projectreadiness.KnownGapCodes()) {
		t.Fatalf("wantCommand has %d entries, gapCodeTable has %d: add the missing gap code to wantCommand with an explicit executable/non-executable decision", len(wantCommand), len(projectreadiness.KnownGapCodes()))
	}
	for _, code := range projectreadiness.KnownGapCodes() {
		if _, ok := wantCommand[code]; !ok {
			t.Fatalf("gapCodeTable defines %q but wantCommand does not: add an explicit executable/non-executable decision for it", code)
		}
	}

	for code, wantExecutable := range wantCommand {
		if problem := remediationOfferProblem(code, wantExecutable); problem != "" {
			t.Error(problem)
		}
	}
}

// remediationOfferProblem describes how PrepareCompilerRemediation's answer
// for code disagrees with wantExecutable, or returns "" when it agrees.
func remediationOfferProblem(code string, wantExecutable bool) string {
	got := PrepareCompilerRemediation(code, "project.json")
	switch {
	case wantExecutable && got == "":
		return fmt.Sprintf("PrepareCompilerRemediation(%q, ...) = \"\", want a non-empty --prepare-compiler command", code)
	case !wantExecutable && got != "":
		return fmt.Sprintf("PrepareCompilerRemediation(%q, ...) = %q, want empty: AC-SET-10 forbids offering a command for a non-prepare-compiler gap", code, got)
	}
	return ""
}
