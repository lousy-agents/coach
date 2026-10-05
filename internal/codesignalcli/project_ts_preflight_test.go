package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps pins
// AC-SET-10's runtime-boundary installer suppression against every gap code
// projectreadiness.KnownGapCodes() currently defines, via a literal per-code expectation rather
// than an expression derived from production code: deriving "should offer a
// command" from projectreadiness.NextActionExecutable/projectreadiness.KnownGapCodes() itself (as
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
		got := PrepareCompilerRemediation(code, "project.json")
		if wantExecutable {
			(&sigTestPrepareCompilerRemediationOffersOnlyExecutablePrepare{code: code, got: got, t: t}).call()

			continue
		}
		if got != "" {
			t.Errorf("PrepareCompilerRemediation(%q, ...) = %q, want empty: AC-SET-10 forbids offering a command for a non-prepare-compiler gap", code, got)
		}
	}
}

func TestAlsoFailingGapLinesFollowsReadinessGaps(t *testing.T) {
	t.Run("a shaped failing compiler is reported from gaps", func(t *testing.T) {
		body_projectTsPreflightTest_aShapedFailingCompilerIsReportedFromGaps_56(t)
	})

	t.Run("a passing compiler with no gaps reports nothing", func(t *testing.T) {
		body_projectTsPreflightTest_aPassingCompilerWithNoGapsReportsNothing_70(t)
	})

	t.Run("an unsupported repository shape still reports every gap readiness names", func(t *testing.T) {
		body_projectTsPreflightTest_anUnsupportedRepositoryShapeStillReportsEveryGap_82(t)
	})

	t.Run("a failing check absent from gaps is not reported", func(t *testing.T) {
		body_projectTsPreflightTest_aFailingCheckAbsentFromGapsIsNotReported_96(t)
	})
}
