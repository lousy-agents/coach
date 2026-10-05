package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestAggregateReadinessWithholdsPrepareCompilerWhenNoChoiceVerified(t *testing.T) {
	checks := projectreadiness.Checks{
		Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		PackageManager: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnsupported, Kind: "yarn"},
	}

	_, _, nextActions, _ := aggregateReadiness(checks, false, nil)

	if _, ok := findNextAction(nextActions, projectreadiness.NextActionPrepareCompiler); ok {
		t.Fatalf("prepare_compiler present in %#v, want withheld: no installation choice is verified", nextActions)
	}
}
