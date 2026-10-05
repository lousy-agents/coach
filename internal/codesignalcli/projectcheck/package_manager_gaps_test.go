package projectcheck

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestAggregateReadinessKeepsPrepareCompilerWithVerifiedMiseChoice proves
// the seam mise version/config-hazard verification plugs into: a rejected
// project adapter never withholds prepare_compiler while a
// projectreadiness.MiseChoice reports a distinct, verified mise origin.
// Run now feeds aggregateReadiness a real
// tstoolchain.EvaluateMiseSetupChoices result (project_readiness.go); this test
// constructs that seam's input directly so aggregateReadiness's own
// contract is proven independently of mise's actual availability in the
// test environment.
func TestAggregateReadinessKeepsPrepareCompilerWithVerifiedMiseChoice(t *testing.T) {
	checks := projectreadiness.Checks{
		Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		PackageManager: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnsupported, Kind: "yarn"},
	}
	miseChoices := []projectreadiness.MiseChoice{{Kind: "mise_project", Verified: true}}

	status, gaps, nextActions, _ := aggregateReadiness(checks, false, miseChoices)

	if status != projectreadiness.StatusNeedsPrerequisite {
		t.Fatalf("status = %q, want %q", status, projectreadiness.StatusNeedsPrerequisite)
	}
	wantGaps := []projectreadiness.Gap{
		{Code: projectreadiness.GapTypescriptCompilerMissing},
		{Code: projectreadiness.GapPackageManagerVersionUnsupported, PackageManagerKind: "yarn"},
	}
	if !reflect.DeepEqual(gaps, wantGaps) {
		t.Fatalf("gaps = %#v, want %#v", gaps, wantGaps)
	}

	prepare, ok := findNextAction(nextActions, projectreadiness.NextActionPrepareCompiler)
	if !ok {
		t.Fatalf("prepare_compiler missing from %#v, want it present with the verified mise choice", nextActions)
	}
	if !reflect.DeepEqual(prepare.Choices, []string{"mise_project"}) {
		t.Fatalf("prepare_compiler.Choices = %#v, want [mise_project] (the rejected yarn adapter must not appear)", prepare.Choices)
	}

	resolve, ok := findNextAction(nextActions, projectreadiness.NextActionResolvePackageManager)
	if !ok {
		t.Fatalf("resolve_package_manager missing from %#v", nextActions)
	}
	if resolve.PackageManagerKind != "yarn" {
		t.Fatalf("resolve_package_manager.PackageManagerKind = %q, want %q", resolve.PackageManagerKind, "yarn")
	}
}

func findNextAction(actions []projectreadiness.NextAction, kind string) (projectreadiness.NextAction, bool) {
	for _, action := range actions {
		if action.Kind == kind {
			return action, true
		}
	}
	return projectreadiness.NextAction{}, false
}

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
