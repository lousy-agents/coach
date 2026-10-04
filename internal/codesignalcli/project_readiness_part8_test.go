package codesignalcli

import (
	"testing"
)

func TestAggregateReadinessWithholdsPrepareCompilerWhenNoChoiceVerified(t *testing.T) {
	checks := ReadinessChecks{
		Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
		PackageManager: ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerVersionUnsupported, Kind: "yarn"},
	}

	_, _, nextActions, _ := aggregateReadiness(checks, false, nil)

	if _, ok := findNextAction(nextActions, nextActionKindPrepareCompiler); ok {
		t.Fatalf("prepare_compiler present in %#v, want withheld: no installation choice is verified", nextActions)
	}
}
