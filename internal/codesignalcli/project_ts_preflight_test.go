package codesignalcli

import (
	"testing"
)

// TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps pins
// AC-SET-10's runtime-boundary installer suppression against every gap code
// gapCodeTable currently defines, via a literal per-code expectation rather
// than an expression derived from production code: deriving "should offer a
// command" from nextActionExecutable/gapCodeTable itself (as
// PrepareCompilerRemediation does) would make the assertion tautological and
// unable to catch a mutation to either.
func TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps(t *testing.T) {
	wantCommand := map[string]bool{
		GapUnsupportedRepositoryShape:        false,
		GapNodeMissing:                       false,
		GapNodeUnsupported:                   false,
		GapNodeUnverifiable:                  false,
		GapTypescriptCompilerMissing:         true,
		GapTypescriptVersionMismatch:         true,
		GapTypescriptVersionConflict:         true,
		GapPackageManagerAmbiguous:           false,
		GapPackageManagerConfigUnverifiable:  false,
		GapPackageManagerVersionUnverifiable: false,
		GapPackageManagerVersionUnsupported:  false,
		GapPolicyMissing:                     false,
		GapPolicyInvalid:                     false,
	}

	if len(wantCommand) != len(gapCodeTable) {
		t.Fatalf("wantCommand has %d entries, gapCodeTable has %d: add the missing gap code to wantCommand with an explicit executable/non-executable decision", len(wantCommand), len(gapCodeTable))
	}
	for code := range gapCodeTable {
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
